package timetracking

import (
	"strings"
	"testing"
)

func TestNormalizedDescription(t *testing.T) {
	for _, test := range []struct {
		name, input, want string
		valid             bool
	}{
		{name: "normal", input: " Investigating the bug ", want: "Investigating the bug", valid: true},
		{name: "empty", input: "", want: "", valid: false},
		{name: "whitespace", input: " \t\n ", want: "", valid: false},
		{name: "maximum", input: strings.Repeat("a", 200), want: strings.Repeat("a", 200), valid: true},
		{name: "too long", input: strings.Repeat("a", 201), want: strings.Repeat("a", 201), valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, valid := normalizedDescription(test.input)
			if got != test.want || valid != test.valid {
				t.Fatalf("normalizedDescription(%q) = (%q, %t), want (%q, %t)", test.input, got, valid, test.want, test.valid)
			}
		})
	}
}
