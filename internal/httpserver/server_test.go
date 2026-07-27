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
	"time"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
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
		{name: "home", path: "/", status: http.StatusSeeOther, contains: "login"},
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
		wrongMethod := http.MethodPost
		if route.Method == http.MethodPost {
			wrongMethod = http.MethodGet
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(wrongMethod, route.Path, nil))
		if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != route.Method {
			t.Fatalf("%s %s response = %d Allow=%q", wrongMethod, route.Path, response.Code, response.Header().Get("Allow"))
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
		Identity:       fakeIdentity{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return handler
}

type fakeIdentity struct{}

func (fakeIdentity) BootstrapAvailable(context.Context) (bool, error) { return true, nil }
func (fakeIdentity) BootstrapAdmin(context.Context, identity.BootstrapInput, identity.AuditContext) (identity.User, error) {
	return identity.User{ID: "user-1", Username: "admin", Email: "admin@example.com", Role: identity.RoleAdmin, Enabled: true}, nil
}
func (fakeIdentity) Login(_ context.Context, input identity.LoginInput, _ identity.AuditContext) (identity.LoginResult, error) {
	if input.Password != "correct password" {
		return identity.LoginResult{}, identity.ErrInvalidCredentials
	}
	return identity.LoginResult{User: identity.User{ID: "user-1", Username: "admin", Role: identity.RoleAdmin, Enabled: true}, SessionToken: "session-token", CSRFToken: "csrf-token", ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (fakeIdentity) CurrentSession(_ context.Context, token string) (identity.Session, error) {
	if token != "session-token" {
		return identity.Session{}, identity.ErrUnauthenticated
	}
	return identity.Session{ID: "session-1", User: identity.User{ID: "user-1", Username: "admin", Role: identity.RoleAdmin, Enabled: true}, ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (fakeIdentity) CSRFToken(string) string { return "csrf-token" }
func (fakeIdentity) ValidateCSRF(_, supplied string) error {
	if supplied != "csrf-token" {
		return identity.ErrCSRFInvalid
	}
	return nil
}
func (fakeIdentity) Logout(context.Context, string, identity.User, identity.AuditContext) error {
	return nil
}

func TestAuthenticationHTTPContract(t *testing.T) {
	t.Parallel()
	handler := testHandler(t, false, nil)

	login := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, LoginAPIPath, strings.NewReader(`{"identifier":"admin","password":"correct password"}`))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(login, request)
	if login.Code != http.StatusOK || !strings.Contains(login.Body.String(), `"csrf_token":"csrf-token"`) {
		t.Fatalf("login = %d %s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie = %#v", cookies)
	}

	session := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, SessionPath, nil)
	request.AddCookie(cookies[0])
	handler.ServeHTTP(session, request)
	if session.Code != http.StatusOK {
		t.Fatalf("session = %d %s", session.Code, session.Body.String())
	}

	forbidden := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, LogoutAPIPath, strings.NewReader("{}"))
	request.AddCookie(cookies[0])
	handler.ServeHTTP(forbidden, request)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("logout without CSRF = %d", forbidden.Code)
	}

	logout := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, LogoutAPIPath, strings.NewReader("{}"))
	request.AddCookie(cookies[0])
	request.Header.Set("X-CSRF-Token", "csrf-token")
	handler.ServeHTTP(logout, request)
	if logout.Code != http.StatusOK || logout.Result().Cookies()[0].MaxAge != -1 {
		t.Fatalf("logout = %d %#v", logout.Code, logout.Result().Cookies())
	}
}

func TestBootstrapAcceptsDocumentedJSONFields(t *testing.T) {
	t.Parallel()
	handler := testHandler(t, false, nil)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, BootstrapPath, strings.NewReader(`{"bootstrap_token":"configured-bootstrap-token-value","username":"admin","email":"admin@example.com","password":"correct password value"}`))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"username":"admin"`) {
		t.Fatalf("bootstrap = %d %s", response.Code, response.Body.String())
	}
}

func TestLoginFailuresAreGeneric(t *testing.T) {
	t.Parallel()
	handler := testHandler(t, false, nil)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, LoginAPIPath, strings.NewReader(`{"identifier":"missing","password":"wrong"}`))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "invalid_credentials") {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}

func TestJSONContentTypeAndBrowserOriginChecks(t *testing.T) {
	t.Parallel()
	handler := testHandler(t, false, nil)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, LoginAPIPath, strings.NewReader(`{"identifier":"admin","password":"correct password"}`))
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("JSON without content type = %d", response.Code)
	}

	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("identifier=admin&password=correct+password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://attacker.example")
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("cross-origin HTML login = %d", response.Code)
	}
}

func TestProductionCookieAndTrustedClientIP(t *testing.T) {
	t.Parallel()
	handler, err := New(Options{AppName: "Project Progress Register", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Readiness: staticReadiness{}, Identity: fakeIdentity{}, Production: true})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, LoginAPIPath, strings.NewReader(`{"identifier":"admin","password":"correct password"}`))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, request)
	if cookies := response.Result().Cookies(); len(cookies) != 1 || !cookies[0].Secure {
		t.Fatalf("production cookie = %#v", cookies)
	}

	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.10, 127.0.0.1")
	if got := clientIP(request); got != "203.0.113.10" {
		t.Fatalf("trusted proxy client IP = %q", got)
	}
	request.RemoteAddr = "192.0.2.2:1234"
	if got := clientIP(request); got != "192.0.2.2" {
		t.Fatalf("untrusted proxy client IP = %q", got)
	}
}

type staticReadiness struct {
	err error
}

func (s staticReadiness) Check(context.Context) error { return s.err }
