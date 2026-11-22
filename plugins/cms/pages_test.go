package cms

import (
	"testing"
	"time"
)

func TestEmptyPageDateHandlesSupportedValues(t *testing.T) {
	date := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "nil", value: nil, want: true},
		{name: "empty string", value: "", want: true},
		{name: "whitespace string", value: " \t", want: true},
		{name: "pointer", value: &date},
		{name: "time", value: date},
		{name: "object is not blank", value: map[string]any{"bad": true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := emptyDate(tc.value); got != tc.want {
				t.Fatalf("emptyDate(%#v) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}
