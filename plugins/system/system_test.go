package system

import (
	"strings"
	"testing"
)

func TestParseAllowsOnlyListedCommandsAndPlainArguments(t *testing.T) {
	pl := &Plugin{}
	ok := map[string]string{
		"migrate:status":                             "migrate:status",
		"artisan migrate:status":                     "migrate:status",
		"./artisan mks:cms-media-copies --site=tech": "mks:cms-media-copies --site=tech",
		"mks:types -o admin/x.ts":                    "mks:types -o admin/x.ts",
		"  list  ":                                   "list",
	}
	for in, want := range ok {
		args, err := pl.Parse(in)
		if err != nil || strings.Join(args, " ") != want {
			t.Errorf("Parse(%q) = %v, %v", in, args, err)
		}
	}
	bad := []string{
		"", "rm -rf /", "migrate:status; rm -rf /", "migrate:status && whoami", "migrate:status | sh",
		"migrate:status `id`", "migrate:status $(id)", "migrate:status > /tmp/x", "db:wipe", "migrate --env=production",
		"--help", "migrate:status\nwhoami", "key:generate", "mks:types \"x\"", strings.Repeat("a", 2001),
		"migrate:status ~/x", "migrate:status {a,b}",
	}
	for _, in := range bad {
		if _, err := pl.Parse(in); err == nil {
			t.Errorf("Parse(%q) accepted", in)
		}
	}
	custom := &Plugin{Commands: []string{"about"}}
	if _, err := custom.Parse("migrate"); err == nil {
		t.Error("Commands must replace the defaults")
	}
}

func TestCappedOutput(t *testing.T) {
	c := &capped{limit: 5}
	_, _ = c.Write([]byte("abc"))
	_, _ = c.Write([]byte("defgh"))
	if got := c.String(); !strings.HasPrefix(got, "abcde") || !strings.Contains(got, "کوتاه") {
		t.Errorf("capped = %q", got)
	}
	if stripANSI("\x1b[32mok\x1b[0m") != "ok" {
		t.Error("ANSI codes stay")
	}
}
