package tickets

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadInputNormalizesAndValidates(t *testing.T) {
	tests := []struct {
		name, body string
		valid      bool
	}{
		{"trim", `{"reference":"  APP-17  ","title":"  Investigate bug  "}`, true},
		{"missing reference", `{"reference":"   ","title":"Investigate bug"}`, false},
		{"missing title", `{"reference":"APP-17","title":"  "}`, false},
		{"invalid JSON", `{`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writer := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/tickets", strings.NewReader(test.body))
			input, valid := readInput(writer, request)
			if valid != test.valid {
				t.Fatalf("valid = %v, want %v", valid, test.valid)
			}
			if valid && (input.Reference != "APP-17" || input.Title != "Investigate bug") {
				t.Fatalf("input not normalized: %+v", input)
			}
		})
	}
}
