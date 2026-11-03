package timetracking

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestTimerResponseTicketReferences(t *testing.T) {
	for _, references := range [][]string{nil, {"FLD-78"}, {"FLD-78", "FLD-79"}} {
		writer := httptest.NewRecorder()
		respondEntry(writer, http.StatusOK, entry{Status: "STOPPED", Description: "Investigation", TicketReferences: references}, time.Now())
		var response struct {
			Data struct {
				Description      string   `json:"description"`
				TicketReferences []string `json:"ticketReferences"`
			} `json:"data"`
		}
		if err := json.Unmarshal(writer.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		want := append([]string{}, references...)
		if !reflect.DeepEqual(response.Data.TicketReferences, want) || response.Data.Description != "Investigation" {
			t.Fatalf("unexpected timer response: %+v", response.Data)
		}
	}
}
