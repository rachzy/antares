package httpapi_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	api "antares/api"
	"antares/api/internal/httpapi"
)

type specDoc struct {
	OpenAPI string                               `yaml:"openapi"`
	Info    map[string]any                       `yaml:"info"`
	Paths   map[string]map[string]map[string]any `yaml:"paths"`
}

var httpMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

func loadSpec(t *testing.T) specDoc {
	t.Helper()
	var doc specDoc
	if err := yaml.Unmarshal(api.OpenAPI, &doc); err != nil {
		t.Fatalf("openapi.yaml does not parse: %v", err)
	}
	return doc
}

func TestTheSpecIsOpenAPI3(t *testing.T) {
	doc := loadSpec(t)
	if !strings.HasPrefix(doc.OpenAPI, "3.") {
		t.Fatalf("openapi = %q, want 3.x", doc.OpenAPI)
	}
	if doc.Info["title"] == nil || doc.Info["version"] == nil {
		t.Fatalf("info needs a title and a version: %v", doc.Info)
	}
	if len(doc.Paths) == 0 {
		t.Fatal("the spec has no paths")
	}

	var tree map[string]any
	if err := yaml.Unmarshal(api.OpenAPI, &tree); err != nil {
		t.Fatal(err)
	}
	for _, ref := range refs(tree) {
		var node any = tree
		for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			m, _ := node.(map[string]any)
			node = m[part]
		}
		if node == nil {
			t.Errorf("$ref %s does not resolve", ref)
		}
	}
}

func refs(node any) []string {
	var out []string
	switch v := node.(type) {
	case map[string]any:
		for key, child := range v {
			if ref, ok := child.(string); ok && key == "$ref" {
				out = append(out, ref)
			}
			out = append(out, refs(child)...)
		}
	case []any:
		for _, child := range v {
			out = append(out, refs(child)...)
		}
	}
	return out
}

func TestEveryRouteIsInTheSpecAndEveryOperationIsARoute(t *testing.T) {
	doc := loadSpec(t)
	var registered []string
	for _, route := range httpapi.Routes() {
		key := strings.ToLower(route.Method) + " " + route.Pattern
		registered = append(registered, key)
		if _, ok := doc.Paths[route.Pattern][strings.ToLower(route.Method)]; !ok {
			t.Errorf("route %s %s is missing from openapi.yaml", route.Method, route.Pattern)
		}
	}
	for path, item := range doc.Paths {
		for method := range item {
			if slices.Contains(httpMethods, method) && !slices.Contains(registered, method+" "+path) {
				t.Errorf("openapi.yaml documents %s %s, which is not registered", strings.ToUpper(method), path)
			}
		}
	}
}

func TestEveryRouteInTheTableIsServed(t *testing.T) {
	h := handler(t, &stub{}, nil)
	for _, route := range httpapi.Routes() {
		path := strings.ReplaceAll(route.Pattern, "{token}", "x")
		rec := do(h, route.Method, path, "")
		if rec.Code == http.StatusMethodNotAllowed || rec.Body.String() == "404 page not found\n" {
			t.Errorf("%s %s is not served by the mux: %d %q", route.Method, path, rec.Code, rec.Body.String())
		}
	}
}

func TestTheSpecIsServed(t *testing.T) {
	rec := do(handler(t, &stub{}, nil), "GET", "/openapi.yaml", "")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/yaml" {
		t.Fatalf("GET /openapi.yaml = %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != string(api.OpenAPI) {
		t.Fatal("GET /openapi.yaml does not return the embedded spec")
	}
}

func TestTheDocsPageLoadsSwaggerUIOnTheSpec(t *testing.T) {
	rec := do(handler(t, &stub{}, nil), "GET", "/docs", "")
	body := rec.Body.String()
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("GET /docs = %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, want := range []string{"/openapi.yaml", "swagger-ui-dist@5.", "swagger-ui-bundle.js"} {
		if !strings.Contains(body, want) {
			t.Errorf("the docs page does not reference %q", want)
		}
	}
}
