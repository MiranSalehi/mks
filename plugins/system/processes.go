package system

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"

	mks "github.com/MiranSalehi/mks"
)

// Background runs (MKSine's console processes): a long command (an import, a big
// migration) runs without the 5-minute limit, its output is read while it runs, and it
// can be stopped. Runs live in this server process: a restart ends them. When one ends,
// it is written to the console history like any other run.
//
//	POST {panel}/api/plugins/system/processes        {"command": "..."} → {"id": ...}
//	GET  {panel}/api/plugins/system/processes        the runs (running first)
//	GET  {panel}/api/plugins/system/processes/{id}?offset=N   status and output from byte N
//	POST {panel}/api/plugins/system/processes/{id}/stop

// The limits of background runs.
const (
	maxRunning        = 3
	keepFinished      = 20
	backgroundTimeout = 6 * time.Hour
)

type process struct {
	ID       string     `json:"id"`
	Command  string     `json:"command"`
	UserID   string     `json:"user_id"`
	Status   string     `json:"status"` // running, completed, failed, stopped
	ExitCode *int       `json:"exit_code"`
	Started  time.Time  `json:"started_at"`
	Finished *time.Time `json:"finished_at"`

	mu      sync.Mutex
	out     capped
	cancel  context.CancelFunc
	stopped bool
}

// processWriter adds output under the process's lock (it is read while it runs).
type processWriter struct{ p *process }

func (w processWriter) Write(b []byte) (int, error) {
	w.p.mu.Lock()
	defer w.p.mu.Unlock()
	return w.p.out.Write(b)
}

type processes struct {
	mu   sync.Mutex
	list []*process
}

// start runs args in the background for user.
func (pl *Plugin) start(args []string, user string) (*process, error) {
	pl.procs.mu.Lock()
	defer pl.procs.mu.Unlock()
	running := 0
	for _, p := range pl.procs.list {
		p.mu.Lock()
		if p.Status == "running" {
			running++
		}
		p.mu.Unlock()
	}
	if running >= maxRunning {
		return nil, fmt.Errorf("At most %d commands run in the background at once.", maxRunning)
	}
	bin := pl.Binary
	if bin == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		bin = exe
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	ctx, cancel := context.WithTimeout(context.Background(), backgroundTimeout)
	p := &process{ID: hex.EncodeToString(b), Command: strings.Join(args, " "), UserID: user, Status: "running",
		Started: time.Now(), cancel: cancel, out: capped{limit: maxOutput}}
	cmd := exec.CommandContext(ctx, bin, append([]string{"artisan"}, args...)...)
	cmd.Dir = pl.Dir
	cmd.Stdout, cmd.Stderr = processWriter{p}, processWriter{p}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	pl.procs.list = append(pl.procs.list, p)
	pl.trimFinished()
	go pl.wait(p, cmd, ctx)
	return p, nil
}

// wait records how a run ended and writes it to the history.
func (pl *Plugin) wait(p *process, cmd *exec.Cmd, ctx context.Context) {
	err := cmd.Wait()
	p.cancel()
	now := time.Now()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		} else {
			code = -1
		}
	}
	p.mu.Lock()
	p.Finished, p.ExitCode = &now, &code
	switch {
	case p.stopped:
		p.Status = "stopped"
		p.out.WriteString("\n[stopped]")
	case ctx.Err() == context.DeadlineExceeded:
		p.Status = "failed"
		p.out.WriteString("\n[command timed out]")
	case code == 0:
		p.Status = "completed"
	default:
		p.Status = "failed"
	}
	output := stripANSI(p.out.String())
	entry := Log{UserID: &p.UserID, Command: p.Command, Output: &output, ExitCode: code, DurationMs: int(now.Sub(p.Started).Milliseconds()), CreatedAt: &now}
	p.mu.Unlock()
	if pl.Orm == nil {
		return
	}
	if err := pl.Orm.Query().Create(&entry); err != nil {
		fmt.Fprintln(os.Stderr, "system: console log:", err)
	}
}

// trimFinished keeps the newest finished runs only (the lock is held).
func (pl *Plugin) trimFinished() {
	var finished []int
	for i, p := range pl.procs.list {
		p.mu.Lock()
		if p.Status != "running" {
			finished = append(finished, i)
		}
		p.mu.Unlock()
	}
	if extra := len(finished) - keepFinished; extra > 0 {
		drop := map[int]bool{}
		for _, i := range finished[:extra] {
			drop[i] = true
		}
		var keep []*process
		for i, p := range pl.procs.list {
			if !drop[i] {
				keep = append(keep, p)
			}
		}
		pl.procs.list = keep
	}
}

func (pl *Plugin) find(id string) *process {
	pl.procs.mu.Lock()
	defer pl.procs.mu.Unlock()
	for _, p := range pl.procs.list {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// snapshot is a run as JSON, with its output from offset on.
func (p *process) snapshot(offset int) http.Json {
	p.mu.Lock()
	defer p.mu.Unlock()
	all := p.out.Bytes()
	if offset < 0 || offset > len(all) {
		offset = 0
	}
	return http.Json{"id": p.ID, "command": p.Command, "status": p.Status, "exit_code": p.ExitCode,
		"started_at": p.Started, "finished_at": p.Finished, "output": stripANSI(string(all[offset:])), "offset": len(all), "cut": p.out.cut}
}

// Routes serves the background runs (the console permission).
func (pl *Plugin) Routes(r route.Router) {
	guard := func(ctx http.Context) (*mks.HookContext, http.Response) {
		hc := pl.panel.Context(ctx)
		if hc.User == nil || !pl.panel.Allowed(hc, Console) {
			return nil, pl.panel.Forbidden(ctx)
		}
		return hc, nil
	}
	r.Post("/processes", func(ctx http.Context) http.Response {
		hc, resp := guard(ctx)
		if resp != nil {
			return resp
		}
		args, err := pl.Parse(ctx.Request().Input("command"))
		if err != nil {
			return ctx.Response().Json(422, http.Json{"message": err.Error(), "errors": http.Json{"command": []string{err.Error()}}})
		}
		p, err := pl.start(args, fmt.Sprint(hc.UserID()))
		if err != nil {
			return ctx.Response().Json(422, http.Json{"message": err.Error(), "errors": http.Json{}})
		}
		return ctx.Response().Json(201, http.Json{"id": p.ID})
	})
	r.Get("/processes", func(ctx http.Context) http.Response {
		if _, resp := guard(ctx); resp != nil {
			return resp
		}
		pl.procs.mu.Lock()
		list := slices.Clone(pl.procs.list)
		pl.procs.mu.Unlock()
		out := make([]http.Json, 0, len(list))
		for i := len(list) - 1; i >= 0; i-- {
			s := list[i].snapshot(0)
			delete(s, "output")
			out = append(out, s)
		}
		slices.SortStableFunc(out, func(a, b http.Json) int {
			ar, br := a["status"] == "running", b["status"] == "running"
			switch {
			case ar && !br:
				return -1
			case br && !ar:
				return 1
			}
			return 0
		})
		return ctx.Response().Json(200, http.Json{"data": out})
	})
	r.Get("/processes/{id}", func(ctx http.Context) http.Response {
		if _, resp := guard(ctx); resp != nil {
			return resp
		}
		p := pl.find(ctx.Request().Route("id"))
		if p == nil {
			return ctx.Response().Json(404, http.Json{"message": "This run was not found."})
		}
		offset, _ := strconv.Atoi(ctx.Request().Query("offset", "0"))
		ctx.Response().Header("Cache-Control", "no-store")
		return ctx.Response().Json(200, p.snapshot(offset))
	})
	r.Post("/processes/{id}/stop", func(ctx http.Context) http.Response {
		if _, resp := guard(ctx); resp != nil {
			return resp
		}
		p := pl.find(ctx.Request().Route("id"))
		if p == nil {
			return ctx.Response().Json(404, http.Json{"message": "This run was not found."})
		}
		p.mu.Lock()
		running := p.Status == "running"
		if running {
			p.stopped = true
		}
		p.mu.Unlock()
		if running {
			p.cancel()
		}
		return ctx.Response().Json(200, http.Json{"stopped": running})
	})
}
