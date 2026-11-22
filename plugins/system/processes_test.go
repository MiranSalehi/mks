package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeArtisan makes a directory whose "artisan" script prints its arguments, then
// counts slowly (a long run).
func fakeArtisan(t *testing.T) *Plugin {
	dir := t.TempDir()
	script := "echo \"ran $*\"\nif [ \"$1\" = \"mks:slow\" ]; then i=0; while [ $i -lt 50 ]; do echo tick $i; i=$((i+1)); sleep 0.1; done; fi\n"
	if err := os.WriteFile(filepath.Join(dir, "artisan"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Plugin{Binary: "/bin/sh", Dir: dir}
}

func waitFor(t *testing.T, cond func() bool) {
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func status(p *process) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Status
}

func TestBackgroundRunCompletes(t *testing.T) {
	pl := fakeArtisan(t)
	p, err := pl.start([]string{"migrate:status"}, "1")
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return status(p) != "running" })
	snap := p.snapshot(0)
	if snap["status"] != "completed" || !strings.Contains(snap["output"].(string), "ran migrate:status") || *(snap["exit_code"].(*int)) != 0 {
		t.Fatalf("snapshot = %v", snap)
	}
	// Reading from an offset gives only what came after it.
	if more := p.snapshot(snap["offset"].(int)); more["output"] != "" {
		t.Fatalf("after the end: %q", more["output"])
	}
}

func TestBackgroundRunStreamsAndStops(t *testing.T) {
	pl := fakeArtisan(t)
	p, err := pl.start([]string{"mks:slow"}, "1")
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return strings.Contains(p.snapshot(0)["output"].(string), "tick 2") })
	if status(p) != "running" {
		t.Fatal("still running while it prints")
	}
	p.mu.Lock()
	p.stopped = true
	p.mu.Unlock()
	p.cancel()
	waitFor(t, func() bool { return status(p) != "running" })
	snap := p.snapshot(0)
	if snap["status"] != "stopped" || strings.Contains(snap["output"].(string), "tick 49") {
		t.Fatalf("stopped run = %v", snap["status"])
	}
}

func TestBackgroundRunsAreLimited(t *testing.T) {
	pl := fakeArtisan(t)
	var started []*process
	for range maxRunning {
		p, err := pl.start([]string{"mks:slow"}, "1")
		if err != nil {
			t.Fatal(err)
		}
		started = append(started, p)
	}
	if _, err := pl.start([]string{"mks:slow"}, "1"); err == nil {
		t.Fatal("more than the limit must be refused")
	}
	for _, p := range started {
		p.cancel()
	}
	for _, p := range started {
		waitFor(t, func() bool { return status(p) != "running" })
	}
	if pl.find(started[0].ID) == nil {
		t.Fatal("finished runs stay listed")
	}
}
