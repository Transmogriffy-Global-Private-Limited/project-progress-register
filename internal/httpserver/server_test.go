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
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/progress"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/projects"
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
	allowedByPath := map[string][]string{}
	for _, route := range ContractRoutes() {
		allowedByPath[route.Path] = append(allowedByPath[route.Path], route.Method)
	}
	for path, allowed := range allowedByPath {
		response := httptest.NewRecorder()
		requestPath := strings.ReplaceAll(path, "{user_id}", "22222222-2222-4222-8222-222222222222")
		requestPath = strings.ReplaceAll(requestPath, "{project_id}", "11111111-1111-4111-8111-111111111111")
		requestPath = strings.ReplaceAll(requestPath, "{task_id}", "44444444-4444-4444-8444-444444444444")
		requestPath = strings.ReplaceAll(requestPath, "{update_id}", "55555555-5555-4555-8555-555555555555")
		requestPath = strings.ReplaceAll(requestPath, "{attachment_id}", "66666666-6666-4666-8666-666666666666")
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodOptions, requestPath, nil))
		if response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("OPTIONS %s response = %d", path, response.Code)
		}
		for _, method := range allowed {
			if !strings.Contains(response.Header().Get("Allow"), method) {
				t.Fatalf("OPTIONS %s Allow=%q, missing %s", path, response.Header().Get("Allow"), method)
			}
		}
	}
}

func testHandler(t *testing.T, docsEnabled bool, readinessError error) http.Handler {
	return testHandlerAtBasePath(t, docsEnabled, readinessError, "")
}

func testHandlerAtBasePath(t *testing.T, docsEnabled bool, readinessError error, basePath string) http.Handler {
	t.Helper()
	handler, err := New(Options{
		AppName:            "Project Progress Register",
		BasePath:           basePath,
		APIDocsEnabled:     docsEnabled,
		Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		Readiness:          staticReadiness{err: readinessError},
		Identity:           fakeIdentity{},
		Projects:           fakeProjects{},
		Progress:           fakeProgress{},
		AttachmentMaxBytes: 100 << 20,
		AttachmentMaxCount: 10,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return handler
}

func TestBasePathScopesRoutesRedirectsAssetsDocsAndCookies(t *testing.T) {
	t.Parallel()
	handler := testHandlerAtBasePath(t, true, nil, "/backend")

	for _, path := range []string{LivenessPath, OpenAPIPath, "/login"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("unprefixed %s = %d", path, response.Code)
		}
	}

	redirect := httptest.NewRecorder()
	handler.ServeHTTP(redirect, httptest.NewRequest(http.MethodGet, "/backend", nil))
	if redirect.Code != http.StatusPermanentRedirect || redirect.Header().Get("Location") != "/backend/" {
		t.Fatalf("base redirect = %d %q", redirect.Code, redirect.Header().Get("Location"))
	}

	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequest(http.MethodGet, "/backend/", nil))
	if home.Code != http.StatusSeeOther || home.Header().Get("Location") != "/backend/login" {
		t.Fatalf("home redirect = %d %q", home.Code, home.Header().Get("Location"))
	}

	loginPage := httptest.NewRecorder()
	handler.ServeHTTP(loginPage, httptest.NewRequest(http.MethodGet, "/backend/login", nil))
	for _, expected := range []string{`href="/backend/assets/app.css"`, `action="/backend/login"`, `href="/backend/setup"`} {
		if loginPage.Code != http.StatusOK || !strings.Contains(loginPage.Body.String(), expected) {
			t.Fatalf("login page missing %q: %d %s", expected, loginPage.Code, loginPage.Body.String())
		}
	}

	login := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/backend"+LoginAPIPath, strings.NewReader(`{"identifier":"admin","password":"correct password"}`))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(login, request)
	if cookies := login.Result().Cookies(); login.Code != http.StatusOK || len(cookies) != 1 || cookies[0].Path != "/backend/" {
		t.Fatalf("prefixed login cookie = %d %#v", login.Code, cookies)
	}

	for _, path := range []string{"/backend" + LivenessPath, "/backend" + OpenAPIPath, "/backend" + APIDocsPath} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("prefixed %s = %d", path, response.Code)
		}
	}
}

type fakeIdentity struct{ currentUser *identity.User }

type fakeProjects struct{}
type fakeProgress struct{}

func (fakeProgress) List(context.Context, identity.User, string, string, progress.AuditContext) ([]progress.Update, error) {
	return []progress.Update{}, nil
}
func (fakeProgress) Get(_ context.Context, _ identity.User, projectID, taskID, updateID string, _ progress.AuditContext) (progress.Update, error) {
	return progress.Update{ID: updateID, ProjectID: projectID, TaskID: taskID, ContentMarkdown: "Progress", ContentHTML: "<p>Progress</p>", Version: 1, Attachments: []progress.Attachment{}, Revisions: []progress.Revision{}}, nil
}
func (fakeProgress) Create(_ context.Context, actor identity.User, projectID, taskID, _ string, metadata progress.CreateMetadata, _ []progress.UploadFile, _ progress.AuditContext) (progress.Update, error) {
	return progress.Update{ID: "55555555-5555-4555-8555-555555555555", ProjectID: projectID, TaskID: taskID, ContentMarkdown: metadata.ContentMarkdown, CreatedBy: progress.Actor{UserID: actor.ID, Username: actor.Username}, Version: 1, Attachments: []progress.Attachment{}, Revisions: []progress.Revision{}}, nil
}
func (fakeProgress) Update(_ context.Context, _ identity.User, projectID, taskID, updateID string, input progress.UpdateInput, _ progress.AuditContext) (progress.Update, error) {
	return progress.Update{ID: updateID, ProjectID: projectID, TaskID: taskID, ContentMarkdown: input.ContentMarkdown, Version: input.ExpectedVersion + 1, Attachments: []progress.Attachment{}, Revisions: []progress.Revision{}}, nil
}
func (fakeProgress) Download(context.Context, identity.User, string, string, string, string, progress.AuditContext) (progress.Download, error) {
	return progress.Download{}, progress.ErrNotFound
}

func (fakeProjects) ListProjects(context.Context, identity.User, projects.AuditContext) ([]projects.Project, error) {
	return []projects.Project{{ID: "11111111-1111-4111-8111-111111111111", Name: "Site Alpha", Active: true, Version: 1}}, nil
}
func (fakeProjects) GetProject(_ context.Context, _ identity.User, projectID string, _ projects.AuditContext) (projects.Project, error) {
	return projects.Project{ID: projectID, Name: "Site Alpha", Active: true, Version: 1}, nil
}
func (fakeProjects) CreateProject(_ context.Context, _ identity.User, input projects.CreateProjectInput, _ projects.AuditContext) (projects.Project, error) {
	return projects.Project{ID: "11111111-1111-4111-8111-111111111111", Name: input.Name, DescriptionMarkdown: input.DescriptionMarkdown, Active: true, Version: 1}, nil
}
func (fakeProjects) UpdateProject(_ context.Context, _ identity.User, projectID string, input projects.UpdateProjectInput, _ projects.AuditContext) (projects.Project, error) {
	return projects.Project{ID: projectID, Name: input.Name, DescriptionMarkdown: input.DescriptionMarkdown, Active: *input.Active, Version: input.ExpectedVersion + 1}, nil
}
func (fakeProjects) ListMembers(context.Context, identity.User, string, projects.AuditContext) ([]projects.Member, error) {
	return []projects.Member{{UserID: "22222222-2222-4222-8222-222222222222", Username: "member", Enabled: true}}, nil
}
func (fakeProjects) AddMember(_ context.Context, _ identity.User, _ string, userID string, _ projects.AuditContext) (projects.Member, error) {
	return projects.Member{UserID: userID, Username: "member", Enabled: true}, nil
}
func (fakeProjects) RemoveMember(context.Context, identity.User, string, string, projects.AuditContext) error {
	return nil
}
func (fakeProjects) ReplaceGeofence(_ context.Context, _ identity.User, _ string, input projects.ReplaceGeofenceInput, _ projects.AuditContext) (projects.Geofence, error) {
	return projects.Geofence{ID: "33333333-3333-4333-8333-333333333333", Version: input.ExpectedVersion + 1, Latitude: input.Latitude, Longitude: input.Longitude, RadiusMetres: input.RadiusMetres, MaxAccuracyMetres: input.MaxAccuracyMetres}, nil
}
func (fakeProjects) ListTasks(context.Context, identity.User, string, projects.AuditContext) ([]projects.Task, error) {
	return []projects.Task{{ID: "44444444-4444-4444-8444-444444444444", ProjectID: testProjectID, Name: "Foundation", GoalsHTML: "<p>Goal</p>", DescriptionHTML: "<p>Description</p>", Version: 1}}, nil
}
func (fakeProjects) GetTask(_ context.Context, _ identity.User, projectID, taskID string, _ projects.AuditContext) (projects.Task, error) {
	return projects.Task{ID: taskID, ProjectID: projectID, Name: "Foundation", Version: 1}, nil
}
func (fakeProjects) CreateTask(_ context.Context, actor identity.User, projectID string, input projects.CreateTaskInput, _ projects.AuditContext) (projects.Task, error) {
	return projects.Task{ID: "44444444-4444-4444-8444-444444444444", ProjectID: projectID, Name: input.Name, GoalsMarkdown: input.GoalsMarkdown, DescriptionMarkdown: input.DescriptionMarkdown, CreatedBy: projects.TaskActor{UserID: actor.ID, Username: actor.Username}, Version: 1}, nil
}
func (fakeProjects) UpdateTask(_ context.Context, actor identity.User, projectID, taskID string, input projects.UpdateTaskInput, _ projects.AuditContext) (projects.Task, error) {
	return projects.Task{ID: taskID, ProjectID: projectID, Name: input.Name, GoalsMarkdown: input.GoalsMarkdown, DescriptionMarkdown: input.DescriptionMarkdown, CreatedBy: projects.TaskActor{UserID: actor.ID, Username: actor.Username}, Version: input.ExpectedVersion + 1}, nil
}

func (fakeIdentity) BootstrapAvailable(context.Context) (bool, error) { return true, nil }
func (fakeIdentity) BootstrapAdmin(context.Context, identity.BootstrapInput, identity.AuditContext) (identity.User, error) {
	return identity.User{ID: "00000000-0000-0000-0000-000000000001", Username: "admin", Email: "admin@example.com", Role: identity.RoleAdmin, Enabled: true, Version: 1}, nil
}
func (fakeIdentity) Login(_ context.Context, input identity.LoginInput, _ identity.AuditContext) (identity.LoginResult, error) {
	if input.Password != "correct password" {
		return identity.LoginResult{}, identity.ErrInvalidCredentials
	}
	return identity.LoginResult{User: identity.User{ID: "00000000-0000-0000-0000-000000000001", Username: "admin", Role: identity.RoleAdmin, Enabled: true, Version: 1}, SessionToken: "session-token", CSRFToken: "csrf-token", ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (f fakeIdentity) CurrentSession(_ context.Context, token string) (identity.Session, error) {
	if token != "session-token" {
		return identity.Session{}, identity.ErrUnauthenticated
	}
	user := identity.User{ID: "00000000-0000-0000-0000-000000000001", Username: "admin", Role: identity.RoleAdmin, Enabled: true, Version: 1}
	if f.currentUser != nil {
		user = *f.currentUser
	}
	return identity.Session{ID: "00000000-0000-0000-0000-000000000010", User: user, ExpiresAt: time.Now().Add(time.Hour)}, nil
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
func (fakeIdentity) ListUsers(_ context.Context, actor identity.User, _ identity.AuditContext) ([]identity.User, error) {
	if actor.Role != identity.RoleAdmin || actor.MustChangePassword {
		return nil, identity.ErrForbidden
	}
	return []identity.User{actor}, nil
}
func (fakeIdentity) CreateUser(_ context.Context, actor identity.User, input identity.CreateUserInput, _ identity.AuditContext) (identity.CredentialResult, error) {
	if actor.Role != identity.RoleAdmin {
		return identity.CredentialResult{}, identity.ErrForbidden
	}
	return identity.CredentialResult{User: identity.User{ID: "00000000-0000-0000-0000-000000000002", Username: input.Username, Email: input.Email, Role: input.Role, Enabled: true, MustChangePassword: true, Version: 1}, TemporaryPassword: "temporary-password-value"}, nil
}
func (fakeIdentity) UpdateUser(_ context.Context, actor identity.User, _ string, input identity.UpdateUserInput, _ identity.AuditContext) (identity.User, error) {
	if actor.Role != identity.RoleAdmin {
		return identity.User{}, identity.ErrForbidden
	}
	return identity.User{ID: "00000000-0000-0000-0000-000000000002", Username: "member", Role: input.Role, Enabled: *input.Enabled, Version: input.ExpectedVersion + 1}, nil
}
func (fakeIdentity) ResetPassword(_ context.Context, actor identity.User, targetID string, _ identity.AuditContext) (identity.CredentialResult, error) {
	if actor.Role != identity.RoleAdmin {
		return identity.CredentialResult{}, identity.ErrForbidden
	}
	return identity.CredentialResult{User: identity.User{ID: targetID, Username: "member", MustChangePassword: true}, TemporaryPassword: "temporary-password-value"}, nil
}
func (fakeIdentity) ChangePassword(context.Context, identity.User, identity.ChangePasswordInput, identity.AuditContext) error {
	return nil
}
func (fakeIdentity) ListIdentityAudit(context.Context, identity.User, identity.AuditContext) ([]identity.AuditRecord, error) {
	return []identity.AuditRecord{{ID: "33333333-3333-4333-8333-333333333333", Action: "identity.user_created", TargetType: "user", Outcome: "succeeded", RequestID: "request-12345678", ClientIP: "127.0.0.1", Details: map[string]any{}}}, nil
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
	handler, err := New(Options{AppName: "Project Progress Register", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Readiness: staticReadiness{}, Identity: fakeIdentity{}, Projects: fakeProjects{}, Progress: fakeProgress{}, AttachmentMaxBytes: 100 << 20, AttachmentMaxCount: 10, Production: true})
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

func TestAccountAdministrationHTTPContract(t *testing.T) {
	t.Parallel()
	handler := testHandler(t, false, nil)
	cookie := &http.Cookie{Name: SessionCookie, Value: "session-token"}

	list := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, AdminUsersAPIPath, nil)
	request.AddCookie(cookie)
	handler.ServeHTTP(list, request)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"users"`) {
		t.Fatalf("list users = %d %s", list.Code, list.Body.String())
	}

	audit := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, AdminIdentityAuditAPIPath, nil)
	request.AddCookie(cookie)
	handler.ServeHTTP(audit, request)
	if audit.Code != http.StatusOK || !strings.Contains(audit.Body.String(), `"identity.user_created"`) {
		t.Fatalf("identity audit = %d %s", audit.Code, audit.Body.String())
	}

	create := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, AdminUsersAPIPath, strings.NewReader(`{"username":"member","email":"member@example.com","role":"member"}`))
	request.AddCookie(cookie)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "csrf-token")
	handler.ServeHTTP(create, request)
	if create.Code != http.StatusCreated || !strings.Contains(create.Body.String(), `"temporary_password"`) || create.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("create user = %d %s", create.Code, create.Body.String())
	}

	target := "22222222-2222-4222-8222-222222222222"
	update := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPatch, AdminUsersAPIPath+"/"+target, strings.NewReader(`{"role":"member","enabled":false,"expected_version":1}`))
	request.AddCookie(cookie)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "csrf-token")
	handler.ServeHTTP(update, request)
	if update.Code != http.StatusOK {
		t.Fatalf("update user = %d %s", update.Code, update.Body.String())
	}

	reset := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, AdminUsersAPIPath+"/"+target+"/password-reset", nil)
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", "csrf-token")
	handler.ServeHTTP(reset, request)
	if reset.Code != http.StatusOK || !strings.Contains(reset.Body.String(), `"temporary_password"`) {
		t.Fatalf("reset password = %d %s", reset.Code, reset.Body.String())
	}

	change := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, PasswordAPIPath, strings.NewReader(`{"password":"replacement-password-value"}`))
	request.AddCookie(cookie)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "csrf-token")
	handler.ServeHTTP(change, request)
	if change.Code != http.StatusOK || change.Result().Cookies()[0].MaxAge != -1 {
		t.Fatalf("change password = %d %#v", change.Code, change.Result().Cookies())
	}
}

func TestAccountAdministrationRequiresCSRF(t *testing.T) {
	t.Parallel()
	handler := testHandler(t, false, nil)
	request := httptest.NewRequest(http.MethodPost, AdminUsersAPIPath, strings.NewReader(`{"username":"member","email":"member@example.com","role":"member"}`))
	request.AddCookie(&http.Cookie{Name: SessionCookie, Value: "session-token"})
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("create without CSRF = %d", response.Code)
	}
}

func TestForcedPasswordChangeBlocksHome(t *testing.T) {
	t.Parallel()
	forced := identity.User{ID: "00000000-0000-0000-0000-000000000002", Username: "member", Role: identity.RoleMember, Enabled: true, MustChangePassword: true, Version: 1}
	handler, err := New(Options{AppName: "Project Progress Register", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Readiness: staticReadiness{}, Identity: fakeIdentity{currentUser: &forced}, Projects: fakeProjects{}, Progress: fakeProgress{}, AttachmentMaxBytes: 100 << 20, AttachmentMaxCount: 10})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookie, Value: "session-token"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("forced home = %d", response.Code)
	}
}

type staticReadiness struct {
	err error
}

func (s staticReadiness) Check(context.Context) error { return s.err }
