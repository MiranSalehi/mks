package translations

import (
	"fmt"
	"strings"
	"testing"

	mks "github.com/MiranSalehi/mks"
)

// Invalid input is refused before the database is touched (q is nil here).
func TestSaveValidates(t *testing.T) {
	pl := &Plugin{}
	cases := map[string]map[string]any{
		"bad key":    {"1bad": "x"},
		"script key": {"a<b": "x"},
		"not text":   {"nav.home": 3},
		"too long":   {"nav.home": strings.Repeat("b", 1001)},
	}
	for name, values := range cases {
		err := pl.save(nil, "fa", values)
		if _, ok := err.(mks.ValidationErrors); !ok {
			t.Errorf("%s: err = %v, want validation errors", name, err)
		}
	}
	many := map[string]any{}
	for i := range 2001 {
		many[fmt.Sprintf("k%d", i)] = "v"
	}
	if _, ok := pl.save(nil, "fa", many).(mks.ValidationErrors); !ok {
		t.Error("more than 2000 texts must be refused")
	}
}

func TestKeyPattern(t *testing.T) {
	for _, k := range []string{"nav.home", "table.empty_state", "a"} {
		if !keyRE.MatchString(k) {
			t.Errorf("%q should be a valid key", k)
		}
	}
	for _, k := range []string{"", "_a", "a-b", "a b", strings.Repeat("a", 121)} {
		if keyRE.MatchString(k) {
			t.Errorf("%q should be refused", k)
		}
	}
}

func TestLocaleRules(t *testing.T) {
	for _, ok := range []string{"ku", "ckb", "ar_IQ", "zh_Hant"} {
		if !codeRE.MatchString(ok) {
			t.Errorf("%q should be a valid code", ok)
		}
	}
	for _, bad := range []string{"", "k", "KU", "abcd", "ku-IQ", "ku_", "ar_IQ_x", "<x>"} {
		if codeRE.MatchString(bad) {
			t.Errorf("%q should be refused", bad)
		}
	}
	for _, ok := range []string{"ckb-IQ", "fa-IR", "zh-Hant-TW", "ku"} {
		if !tagRE.MatchString(ok) {
			t.Errorf("tag %q should be valid", ok)
		}
	}
	for _, bad := range []string{"ckb_IQ", "not a tag", "x"} {
		if tagRE.MatchString(bad) {
			t.Errorf("tag %q should be refused", bad)
		}
	}
}

func TestBuiltInLocalesCannotBeRemoved(t *testing.T) {
	pl := &Plugin{}
	for _, code := range Locales {
		if _, err := pl.removeLocale(nil, map[string]any{"code": code}); err == nil {
			t.Errorf("%s must not be removable", code)
		}
	}
}
