package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFoundationRoutes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		readiness error
		path      string
		status    int
		contains  string
	}{
		{name: "home", path: "/", status: http.StatusOK, contains: "Project Progress Register"},
		{name: "liveness", path: LivenessPath, status: http.StatusOK, contains: `"status":"ok"`},
		{name: "ready", path: ReadinessPath, status: http.StatusOK, contains: `"status":"ready"`},
		{name: "not ready", readiness: errors.New("database down"), path: ReadinessPath, status: http.StatusServiceUnavailable, contains: `"status":"not_ready"`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler := testHandler(t, false, test.readiness)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.contains) {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
			if response.Header().Get("Content-Security-Policy") == "" {
				t.Fatal("security headers were not applied")
			}
		})
	}
}

func TestAPIDocsToggle(t *testing.T) {
	t.Parallel()

	for _, enabled := range []bool{false, true} {
		enabled := enabled
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			t.Parallel()
			handler := testHandler(t, enabled, nil)
			for _, path := range []string{OpenAPIPath, APIDocsPath} {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
				if enabled && response.Code != http.StatusOK {
					t.Fatalf("%s enabled status = %d", path, response.Code)
				}
				if !enabled && response.Code != http.StatusNotFound {
					t.Fatalf("%s disabled status = %d", path, response.Code)
				}
				if enabled && path == APIDocsPath && !strings.Contains(strings.ToLower(response.Body.String()), "swagger") {
					t.Fatal("enabled API documentation did not render Swagger UI HTML")
				}
				if enabled && path == OpenAPIPath && !strings.Contains(response.Body.String(), "openapi: 3.1.0") {
					t.Fatal("raw OpenAPI route did not serve the authoritative document")
				}
			}
		})
	}
}

func TestAPIRoutesRejectOtherMethods(t *testing.T) {
	t.Parallel()

	handler := testHandler(t, false, nil)
	for _, route := range ContractRoutes() {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, route.Path, nil))
		if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodGet {
			t.Fatalf("POST %s response = %d Allow=%q", route.Path, response.Code, response.Header().Get("Allow"))
		}
	}
}

func testHandler(t *testing.T, docsEnabled bool, readinessError error) http.Handler {
	t.Helper()
	handler, err := New(Options{
		AppName:        "Project Progress Register",
		APIDocsEnabled: docsEnabled,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Readiness:      staticReadiness{err: readinessError},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return handler
}

type staticReadiness struct {
	err error
}

func (s staticReadiness) Check(context.Context) error { return s.err }
