// Package system adds MKSine's System health page and admin console to the panel:
//
//   - "system_health": database, cache, storage, disk space, memory, debug mode and
//     runtime, each ok / warning / failed.
//   - "console": runs allowlisted artisan commands of the app itself (never a shell:
//     the app's own binary is executed with plain arguments), with a timeout, an output
//     cap and a history.
//
// Both need their permission (system.health, system.console); give the console to full
// managers only. Its frontend half is @mksine/mks/system.
//
//	p.Use(&system.Plugin{Orm: facades.Orm(), Cache: facades.Cache(), Storage: facades.Storage(), Debug: debug})
//
// The history table comes from a migration: system.CreateTable(facades.Schema()).
package system

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/goravel/framework/contracts/cache"
	contractsconfig "github.com/goravel/framework/contracts/config"
	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/contracts/filesystem"

	mks "github.com/MiranSalehi/mks"
)

// Permissions.
const (
	Health  = "system.health"
	Console = "system.console"
)

// Table keeps the console history.
const Table = "panel_console_logs"

// DefaultCommands are the artisan commands the console may run. A trailing "*" allows a
// prefix ("mks:*").
var DefaultCommands = []string{"list", "about", "migrate:status", "migrate", "cache:clear", "route:list", "schedule:run", "schedule:list", "queue:failed", "queue:retry", "mks:*"}

const (
	maxCommand = 2000
	maxOutput  = 512 << 10
	timeout    = 300 * time.Second
)

// CreateTable creates the console history table; call it from a migration.
func CreateTable(s schema.Schema) error {
	return s.Create(Table, func(t schema.Blueprint) {
		t.ID()
		t.String("user_id", 64).Nullable()
		t.String("command", 2000)
		t.LongText("output").Nullable()
		t.Integer("exit_code").Default(0)
		t.Integer("duration_ms").Default(0)
		t.Timestamp("created_at").Nullable()
		t.Index("created_at")
	})
}

// Log is one console run.
type Log struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     *string    `json:"user_id"`
	Command    string     `json:"command"`
	Output     *string    `json:"output"`
	ExitCode   int        `json:"exit_code"`
	DurationMs int        `json:"duration_ms"`
	CreatedAt  *time.Time `json:"created_at"`
}

func (Log) TableName() string { return Table }

// Plugin is the health page and the console. Orm is required; Cache and Storage make
// their checks possible.
type Plugin struct {
	Orm     orm.Orm
	Cache   cache.Cache
	Storage filesystem.Storage
	// Config is optional: it fills the queue card of the console page.
	Config contractsconfig.Config
	// Disk is checked for writes. Default: "public".
	Disk string
	// Debug reports whether the app runs in debug mode (a warning in production).
	Debug bool
	// Commands replaces DefaultCommands.
	Commands []string
	// Binary and Dir run the commands: the app's executable (default os.Executable) with
	// "artisan" first, in Dir (default the working directory).
	Binary string
	Dir    string

	started time.Time
	badgeMu sync.Mutex
	badge   int64
	badgeAt time.Time
	procs   processes
	panel   *mks.Panel
}

func (*Plugin) Name() string { return "system" }

func (pl *Plugin) Register(p *mks.Panel) {
	pl.panel = p
	if pl.Orm == nil {
		panic("system: Orm is required")
	}
	pl.started = time.Now()
	p.Permission(Health, Console)
	p.Register(mks.Define[Log](mks.Config{
		Name: "console_logs", Searchable: []string{"command"}, Sortable: []string{"id", "created_at"},
		TitleColumn: "command", DefaultSort: "-id", Deletable: true,
	}).Policy(mks.Policy[Log]{
		ViewAny: func(ctx *mks.HookContext) bool { return p.Allowed(ctx, Console) },
		Create:  func(*mks.HookContext) bool { return false },
		Update:  func(*mks.HookContext, *Log) bool { return false },
		Delete:  func(ctx *mks.HookContext, _ *Log) bool { return p.Allowed(ctx, Console) },
	}))
	p.Pages(
		mks.Page{
			Name:      "system_health",
			Authorize: func(ctx *mks.HookContext) bool { return p.Allowed(ctx, Health) },
			Data:      func(*mks.WidgetContext) (any, error) { return map[string]any{"checks": pl.checks()}, nil },
			// MKSine's badge: the number of problems, cached for a minute.
			NavigationBadge:      func(*mks.HookContext) int64 { return pl.problems() },
			NavigationBadgeColor: "danger",
		},
		mks.Page{
			Name:      "console",
			Authorize: func(ctx *mks.HookContext) bool { return p.Allowed(ctx, Console) },
			Data: func(*mks.WidgetContext) (any, error) {
				return map[string]any{
					"commands":  pl.allowed(),
					"queue":     pl.queueCard(),
					"workers":   map[string]any{"items": pl.queueWorkers(), "can_start": pl.allowedCommand("queue:work")},
					"scheduler": pl.schedulerCard(),
				}, nil
			},
			Rules: map[string]string{"command": "required|max_len:2000"},
			Handle: func(ctx *mks.PageContext) (*mks.ActionResult, error) {
				line, _ := ctx.Data["command"].(string)
				args, err := pl.Parse(line)
				if err != nil {
					return nil, mks.ValidationErrors{"command": err.Error()}
				}
				out, code, dur := pl.run(args)
				user := fmt.Sprint(ctx.UserID())
				now := time.Now()
				entry := Log{UserID: &user, Command: strings.Join(args, " "), Output: &out, ExitCode: code, DurationMs: int(dur.Milliseconds()), CreatedAt: &now}
				if err := ctx.Query.Create(&entry); err != nil {
					return nil, err
				}
				r := mks.Success(fmt.Sprintf("Finished with code %d in %s.", code, dur.Round(time.Millisecond)))
				if code != 0 {
					r = mks.Warning(fmt.Sprintf("The command exited with code %d.", code))
				}
				r.Invalidate = []string{"console_logs"}
				r.Data = map[string]any{"run": entry}
				return r, nil
			},
		},
	)
}

func (pl *Plugin) allowed() []string {
	if len(pl.Commands) > 0 {
		return pl.Commands
	}
	return DefaultCommands
}

var (
	argRE     = regexp.MustCompile(`^[A-Za-z0-9_:.,=/@+-]+$`)
	forbidden = "; & | ` $ < > # \\ ' \" ( ) { } * ? ~ !"
)

// Parse checks a command line ("migrate:status", "artisan mks:cms-media-copies --site=tech")
// and returns its arguments: the allowed command and plain arguments, nothing a shell
// would interpret (no shell runs it anyway), no --env.
func (pl *Plugin) Parse(line string) ([]string, error) {
	line = strings.TrimSpace(line)
	if line == "" || len(line) > maxCommand {
		return nil, errors.New("The command is empty or too long.")
	}
	for _, r := range line {
		if r < ' ' || r == 0x7f {
			return nil, errors.New("The command contains a control character.")
		}
	}
	for _, bad := range strings.Fields(forbidden) {
		if strings.Contains(line, bad) {
			return nil, fmt.Errorf("The character %q is not allowed.", bad)
		}
	}
	fields := strings.Fields(line)
	if len(fields) > 0 && (fields[0] == "artisan" || fields[0] == "./artisan") {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return nil, errors.New("No command was entered.")
	}
	ok := false
	for _, allowed := range pl.allowed() {
		if allowed == fields[0] || (strings.HasSuffix(allowed, "*") && strings.HasPrefix(fields[0], strings.TrimSuffix(allowed, "*"))) {
			ok = true
		}
	}
	if !ok || strings.HasPrefix(fields[0], "-") {
		return nil, fmt.Errorf("The command \"%s\" is not allowed.", fields[0])
	}
	for _, a := range fields[1:] {
		if !argRE.MatchString(a) {
			return nil, fmt.Errorf("The argument \"%s\" is not allowed.", a)
		}
		if a == "--env" || strings.HasPrefix(a, "--env=") {
			return nil, errors.New("--env is not allowed.")
		}
	}
	return fields, nil
}

// run executes "<binary> artisan args..." with a timeout and an output cap.
func (pl *Plugin) run(args []string) (string, int, time.Duration) {
	bin := pl.Binary
	if bin == "" {
		exe, err := os.Executable()
		if err != nil {
			return "executable: " + err.Error(), -1, 0
		}
		bin = exe
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, append([]string{"artisan"}, args...)...)
	cmd.Dir = pl.Dir
	out := &capped{limit: maxOutput}
	cmd.Stdout, cmd.Stderr = out, out
	start := time.Now()
	err := cmd.Run()
	dur := time.Since(start)
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		} else {
			code = -1
			out.WriteString("\n" + err.Error())
		}
	}
	if ctx.Err() != nil {
		out.WriteString("\n[command timed out]")
	}
	return stripANSI(out.String()), code, dur
}

type capped struct {
	bytes.Buffer
	limit int
	cut   bool
}

func (c *capped) Write(p []byte) (int, error) {
	if room := c.limit - c.Len(); room > 0 {
		if len(p) > room {
			c.Buffer.Write(p[:room])
			c.cut = true
		} else {
			c.Buffer.Write(p)
		}
	} else {
		c.cut = true
	}
	return len(p), nil
}

func (c *capped) String() string {
	if c.cut {
		return c.Buffer.String() + "\n[output truncated]"
	}
	return c.Buffer.String()
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSI(s string) string { return ansi.ReplaceAllString(s, "") }

// --- queue, workers and scheduler (the console page's cards) ------------------------------

// queueCard is the queue connection in use and what waits in its table.
func (pl *Plugin) queueCard() map[string]any {
	connection, driver, queue := "sync", "sync", "default"
	if pl.Config != nil {
		if v := pl.Config.GetString("queue.default"); v != "" {
			connection = v
		}
		if v := pl.Config.GetString("queue.connections." + connection + ".driver"); v != "" {
			driver = v
		}
		if v := pl.Config.GetString("queue.connections." + connection + ".queue"); v != "" {
			queue = v
		}
	}
	out := map[string]any{"connection": connection, "driver": driver, "queue": queue, "pending": 0, "failed": 0, "oldest": nil}
	if pending, err := pl.Orm.Query().Table("jobs").Count(); err == nil {
		out["pending"] = pending
	}
	if failed, err := pl.Orm.Query().Table("failed_jobs").Count(); err == nil {
		out["failed"] = failed
	}
	var row struct{ Oldest *time.Time }
	if err := pl.Orm.Query().Table("jobs").Select("MIN(created_at) AS oldest").Scan(&row); err == nil && row.Oldest != nil {
		out["oldest"] = row.Oldest
	}
	return out
}

// queueWorkers are the running "queue:work" background processes.
func (pl *Plugin) queueWorkers() []map[string]any {
	pl.procs.mu.Lock()
	list := append([]*process(nil), pl.procs.list...)
	pl.procs.mu.Unlock()
	out := []map[string]any{}
	for _, p := range list {
		p.mu.Lock()
		running := p.Status == "running"
		info := map[string]any{"id": p.ID, "command": p.Command, "started_at": p.Started}
		p.mu.Unlock()
		if running && strings.HasPrefix(p.Command, "queue:work") {
			out = append(out, info)
		}
	}
	return out
}

// schedulerCard reports the scheduler: a running "schedule:work", the last
// "schedule:run" and the cron line that would run it every minute.
func (pl *Plugin) schedulerCard() map[string]any {
	running := false
	runID := ""
	pl.procs.mu.Lock()
	list := append([]*process(nil), pl.procs.list...)
	pl.procs.mu.Unlock()
	for _, p := range list {
		p.mu.Lock()
		if p.Status == "running" && strings.HasPrefix(p.Command, "schedule:work") {
			running = true
			runID = p.ID
		}
		p.mu.Unlock()
	}
	var last *time.Time
	var row Log
	if err := pl.Orm.Query().Model(&Log{}).Where("command LIKE ?", "schedule:run%").Order("id DESC").Limit(1).Find(&row); err == nil && row.ID > 0 {
		last = row.CreatedAt
	}
	return map[string]any{
		"running":   running,
		"id":        runID,
		"last_run":  last,
		"cron":      pl.cronLine(),
		"can_start": pl.allowedCommand("schedule:work"),
	}
}

// cronLine is the crontab line that runs the scheduler every minute.
func (pl *Plugin) cronLine() string {
	dir := pl.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}
	bin := pl.Binary
	if bin == "" {
		bin, _ = os.Executable()
	}
	return "cd " + shellQuote(dir) + " && " + shellQuote(bin) + " artisan schedule:run"
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

// allowedCommand reports whether the console may run this command (a trailing * allows a prefix).
func (pl *Plugin) allowedCommand(command string) bool {
	for _, allowed := range pl.allowed() {
		if allowed == command || (strings.HasSuffix(allowed, "*") && strings.HasPrefix(command, strings.TrimSuffix(allowed, "*"))) {
			return true
		}
	}
	return false
}

// --- health ------------------------------------------------------------------------------

// Check is one line of the health report.
type Check struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"` // ok, warning, failed
	Detail string `json:"detail"`
}

// problems counts the checks that are not ok, at most once a minute.
func (pl *Plugin) problems() int64 {
	pl.badgeMu.Lock()
	defer pl.badgeMu.Unlock()
	if time.Since(pl.badgeAt) < time.Minute {
		return pl.badge
	}
	var n int64
	for _, c := range pl.checks() {
		if c.Status != "ok" {
			n++
		}
	}
	pl.badge, pl.badgeAt = n, time.Now()
	return n
}

func (pl *Plugin) checks() []Check {
	var out []Check
	add := func(key, label, status, detail string) { out = append(out, Check{key, label, status, detail}) }

	start := time.Now()
	var one int
	if err := pl.Orm.Query().Raw("SELECT 1").Scan(&one); err != nil {
		add("database", "Database", "failed", err.Error())
	} else {
		ms := time.Since(start).Milliseconds()
		status := "ok"
		if ms > 200 {
			status = "warning"
		}
		add("database", "Database", status, fmt.Sprintf("Responded in %d ms", ms))
	}

	if pl.Cache == nil {
		add("cache", "cache", "warning", "Not set")
	} else {
		key := "system:health:" + fmt.Sprint(time.Now().UnixNano())
		if err := pl.Cache.Put(key, "ok", time.Minute); err != nil || pl.Cache.GetString(key) != "ok" {
			add("cache", "cache", "failed", "Could not read or write")
		} else {
			add("cache", "cache", "ok", "Read and write")
		}
		pl.Cache.Forget(key)
	}

	if pl.Storage == nil {
		add("storage", "File storage", "warning", "Not set")
	} else {
		disk := pl.Disk
		if disk == "" {
			disk = "public"
		}
		d := pl.Storage.Disk(disk)
		name := ".health-" + fmt.Sprint(time.Now().UnixNano())
		if err := d.Put(name, "ok"); err != nil {
			add("storage", "File storage", "failed", disk+": "+err.Error())
		} else {
			_ = d.Delete(name)
			add("storage", "File storage", "ok", disk+": writable")
		}
	}

	var fs syscall.Statfs_t
	if wd, err := os.Getwd(); err == nil && syscall.Statfs(wd, &fs) == nil {
		free := float64(fs.Bavail) * float64(fs.Bsize)
		total := float64(fs.Blocks) * float64(fs.Bsize)
		status := "ok"
		if total > 0 && free/total < 0.1 {
			status = "warning"
		}
		add("disk", "Disk space", status, fmt.Sprintf("%.1f GB free of %.1f", free/(1<<30), total/(1<<30)))
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	add("memory", "Memory", "ok", fmt.Sprintf("%.1f MB in use, %d goroutines", float64(mem.Alloc)/(1<<20), runtime.NumGoroutine()))

	if pl.Debug {
		add("debug", "Debug mode", "warning", "Debug is on; turn it off in production (error details are shown).")
	} else {
		add("debug", "Debug mode", "ok", "Off")
	}
	add("runtime", "Go", "ok", fmt.Sprintf("%s, %s/%s, up for %s", runtime.Version(), runtime.GOOS, runtime.GOARCH, time.Since(pl.started).Round(time.Second)))
	return out
}
