package tickets_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func contractCompiler(t *testing.T) (*jsonschema.Compiler, map[string]any) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(content, &doc); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err = compiler.AddResource("https://timetracker.example.test/openapi.json", doc); err != nil {
		t.Fatal(err)
	}
	return compiler, doc
}

func escapePointer(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func responseContract(t *testing.T, compiler *jsonschema.Compiler, path, method, status string) *jsonschema.Schema {
	t.Helper()
	url := "https://timetracker.example.test/openapi.json#/paths/" + escapePointer(path) + "/" + strings.ToLower(method) + "/responses/" + status + "/content/application~1json/schema"
	schema, err := compiler.Compile(url)
	if err != nil {
		t.Fatalf("compile response contract %s %s %s: %v", method, path, status, err)
	}
	return schema
}

func TestAllJSONResponseContractsCompile(t *testing.T) {
	compiler, doc := contractCompiler(t)
	count := 0
	for path, methods := range doc["paths"].(map[string]any) {
		for method, operation := range methods.(map[string]any) {
			for status, response := range operation.(map[string]any)["responses"].(map[string]any) {
				content, _ := response.(map[string]any)["content"].(map[string]any)
				if content["application/json"] == nil {
					continue
				}
				responseContract(t, compiler, path, method, status)
				count++
			}
		}
	}
	if count < 100 {
		t.Fatalf("only %d JSON response contracts compiled", count)
	}
	invalid := map[string]any{"data": map[string]any{"id": "not-a-uuid"}, "error": nil, "meta": map[string]any{}}
	if err := responseContract(t, compiler, "/api/v1/organizations/{organizationID}/tickets", "POST", "201").Validate(invalid); err == nil {
		t.Fatal("incomplete malformed ticket response accepted")
	}
}

// Validate actual database-backed critical-flow responses, not just fixtures.
func assertIntegrationContract(t *testing.T, action http.HandlerFunc, method string, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
		return
	}
	name := runtime.FuncForPC(reflect.ValueOf(action).Pointer()).Name()
	name = strings.TrimSuffix(name, "-fm")
	name = name[strings.LastIndex(name, ".")+1:]
	var path string
	switch name {
	case "Create", "List":
		path = "/api/v1/organizations/{organizationID}/tickets"
	case "Get", "Update", "Delete":
		path = "/api/v1/organizations/{organizationID}/tickets/{ticketID}"
	case "CreateManual":
		path = "/api/v1/time-entries/manual"
	case "CompletedDetail", "UpdateCompleted", "DeleteCompleted":
		path = "/api/v1/time-entries/{entryID}"
	case "Entries":
		path = "/api/v1/reports/entries"
	case "Summary":
		path = "/api/v1/reports/summary"
	case "Start", "Stop", "Pause", "Resume":
		path = "/api/v1/timer/" + strings.ToLower(name)
	case "Active", "UpdateActive":
		path = "/api/v1/timer/active"
	default:
		t.Fatalf("unmapped integration handler %s", name)
	}
	compiler, _ := contractCompiler(t)
	status := fmt.Sprint(recorder.Code)
	if recorder.Code >= 400 {
		status = "default"
	}
	var payload any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if err := responseContract(t, compiler, path, method, status).Validate(payload); err != nil {
		t.Fatalf("%s %s response does not match OpenAPI: %v\n%s", method, path, err, recorder.Body.String())
	}
}
