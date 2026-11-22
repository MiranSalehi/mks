package cms

import "testing"

func TestASCIISlug(t *testing.T) {
	for in, want := range map[string]string{
		"Go Lang!":     "go-lang",
		"  React 19  ": "react-19",
		"برنامه‌نویسی": "",
		"Go و React":   "go-react",
		"--a--b--":     "a-b",
	} {
		if got := asciiSlug(in); got != want {
			t.Errorf("asciiSlug(%q) = %q, want %q", in, got, want)
		}
	}
}
