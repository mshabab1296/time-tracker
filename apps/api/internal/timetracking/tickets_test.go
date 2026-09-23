package timetracking

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidTicketIDs(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	for _, test := range []struct {
		name  string
		ids   []uuid.UUID
		valid bool
	}{
		{"none", nil, true},
		{"multiple", []uuid.UUID{first, second}, true},
		{"duplicate", []uuid.UUID{first, first}, false},
		{"nil UUID", []uuid.UUID{uuid.Nil}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := validTicketIDs(test.ids); got != test.valid {
				t.Fatalf("validTicketIDs() = %t, want %t", got, test.valid)
			}
		})
	}
}
