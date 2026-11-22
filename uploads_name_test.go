package mks

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestUploadNameFitsAColumn(t *testing.T) {
	if got := uploadName("photo.jpg"); got != "photo.jpg" {
		t.Fatalf("short names stay: %q", got)
	}
	long := strings.Repeat("ع", 300) + ".jpeg"
	got := uploadName(long)
	if utf8.RuneCountInString(got) != 255 || !strings.HasSuffix(got, ".jpeg") {
		t.Fatalf("long name = %d runes, %q…", utf8.RuneCountInString(got), got[len(got)-10:])
	}
}
