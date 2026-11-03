package httpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readContract(t *testing.T) map[string]any {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "docs", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(content, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// Enforce typed payloads and resolvable references for every documented JSON success.
func TestOpenAPIResponseSchemasAreTyped(t *testing.T) {
	doc := readContract(t)
	var check func(any)
	check = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			if reference, ok := value["$ref"].(string); ok {
				var target any = doc
				for _, part := range strings.Split(strings.TrimPrefix(reference, "#/"), "/") {
					object, ok := target.(map[string]any)
					if !ok || object[part] == nil {
						t.Fatalf("unresolved reference %s", reference)
					}
					target = object[part]
				}
			}
			for _, child := range value {
				check(child)
			}
		case []any:
			for _, child := range value {
				check(child)
			}
		}
	}
	check(doc)
	for path, methods := range doc["paths"].(map[string]any) {
		for method, operation := range methods.(map[string]any) {
			for status, response := range operation.(map[string]any)["responses"].(map[string]any) {
				if !strings.HasPrefix(status, "2") {
					continue
				}
				content, _ := response.(map[string]any)["content"].(map[string]any)
				media, ok := content["application/json"].(map[string]any)
				if !ok {
					continue
				}
				schema := media["schema"].(map[string]any)
				properties, ok := schema["properties"].(map[string]any)
				if !ok {
					t.Fatalf("%s %s missing typed envelope", method, path)
				}
				data, ok := properties["data"].(map[string]any)
				if !ok || len(data) == 0 {
					t.Fatalf("%s %s has unrestricted data", method, path)
				}
				if properties["error"].(map[string]any)["type"] != "null" {
					t.Fatalf("%s %s invalid success error", method, path)
				}
			}
		}
	}
}

// All session-protected operations must reject anonymous requests before querying
// the database. A nil pool makes accidental pre-authentication DB access fail too.
func TestProtectedRoutesRejectAnonymousRequests(t *testing.T) {
	doc := readContract(t)
	handler := New(nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, "http://localhost", false)
	count := 0
	for path, methods := range doc["paths"].(map[string]any) {
		for method, value := range methods.(map[string]any) {
			operation := value.(map[string]any)
			security, _ := operation["security"].([]any)
			if len(security) == 0 {
				continue
			}
			count++
			t.Run(method+" "+path, func(t *testing.T) {
				actualPath := regexp.MustCompile(`\{[^}]+\}`).ReplaceAllString(path, "10000000-0000-4000-8000-000000000001")
				if strings.HasSuffix(path, "/{decision}") {
					actualPath = strings.TrimSuffix(actualPath, "10000000-0000-4000-8000-000000000001") + "accept"
				}
				if strings.HasSuffix(path, "/reports/export") {
					actualPath += "?format=detailed"
				}
				request := httptest.NewRequest(strings.ToUpper(method), actualPath, strings.NewReader(`{}`))
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				if recorder.Code != 401 {
					t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
				}
				var envelope struct {
					Data  any `json:"data"`
					Error struct {
						Code    string `json:"code"`
						Message string `json:"message"`
					} `json:"error"`
					Meta map[string]any `json:"meta"`
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				if envelope.Data != nil || envelope.Error.Code == "" || envelope.Error.Message == "" || envelope.Meta == nil {
					t.Fatal(fmt.Sprintf("invalid error envelope: %s", recorder.Body.String()))
				}
			})
		}
	}
	if count < 40 {
		t.Fatalf("only %d protected operations checked", count)
	}
}

func TestOpenAPICoversRegisteredRoutes(t *testing.T) {
	specification, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "docs", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(specification, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.OpenAPI != "3.1.0" {
		t.Fatalf("unexpected OpenAPI version: %s", doc.OpenAPI)
	}
	source, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	routes := regexp.MustCompile(`mux.Handle(?:Func)?\("([A-Z]+) ([^"]+)"`).FindAllStringSubmatch(string(source), -1)
	count := 0
	for _, route := range routes {
		method, path := strings.ToLower(route[1]), route[2]
		if len(doc.Paths[path][method]) == 0 {
			t.Errorf("OpenAPI missing %s %s", method, path)
		}
		count++
	}
	documented := 0
	for path, methods := range doc.Paths {
		for method := range methods {
			if method == "parameters" {
				continue
			}
			documented++
			found := false
			for _, route := range routes {
				if path == route[2] && method == strings.ToLower(route[1]) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("OpenAPI documents unregistered route: %s %s", method, path)
			}
		}
	}
	if count != documented {
		t.Errorf("route count=%d documented=%d", count, documented)
	}
}
