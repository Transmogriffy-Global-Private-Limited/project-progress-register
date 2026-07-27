package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	openapiv1 "github.com/Transmogriffy-Global-Private-Limited/project-progress-register/api/openapi/v1"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/progress"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/projects"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/webui"
	"github.com/swaggest/swgui/v5emb"
)

const (
	LivenessPath              = "/api/v1/health/live"
	ReadinessPath             = "/api/v1/health/ready"
	OpenAPIPath               = "/api/openapi/v1/openapi.yaml"
	APIDocsPath               = "/api/docs/"
	BootstrapPath             = "/api/v1/setup/bootstrap"
	LoginAPIPath              = "/api/v1/auth/login"
	SessionPath               = "/api/v1/auth/session"
	LogoutAPIPath             = "/api/v1/auth/logout"
	PasswordAPIPath           = "/api/v1/auth/password"
	AdminUsersAPIPath         = "/api/v1/admin/users"
	AdminIdentityAuditAPIPath = "/api/v1/admin/audit/identity"
	ProjectsAPIPath           = "/api/v1/projects"
	SessionCookie             = "ppr_session"
)

// Route identifies one externally callable API operation covered by OpenAPI.
type Route struct {
	Method string
	Path   string
}

// ContractRoutes returns the registered versioned API surface.
func ContractRoutes() []Route {
	return []Route{
		{Method: http.MethodGet, Path: LivenessPath},
		{Method: http.MethodGet, Path: ReadinessPath},
		{Method: http.MethodPost, Path: BootstrapPath},
		{Method: http.MethodPost, Path: LoginAPIPath},
		{Method: http.MethodGet, Path: SessionPath},
		{Method: http.MethodPost, Path: LogoutAPIPath},
		{Method: http.MethodPost, Path: PasswordAPIPath},
		{Method: http.MethodGet, Path: AdminUsersAPIPath},
		{Method: http.MethodPost, Path: AdminUsersAPIPath},
		{Method: http.MethodPatch, Path: AdminUsersAPIPath + "/{user_id}"},
		{Method: http.MethodPost, Path: AdminUsersAPIPath + "/{user_id}/password-reset"},
		{Method: http.MethodGet, Path: AdminIdentityAuditAPIPath},
		{Method: http.MethodGet, Path: ProjectsAPIPath},
		{Method: http.MethodPost, Path: ProjectsAPIPath},
		{Method: http.MethodGet, Path: ProjectsAPIPath + "/{project_id}"},
		{Method: http.MethodPatch, Path: ProjectsAPIPath + "/{project_id}"},
		{Method: http.MethodGet, Path: ProjectsAPIPath + "/{project_id}/members"},
		{Method: http.MethodPut, Path: ProjectsAPIPath + "/{project_id}/members/{user_id}"},
		{Method: http.MethodDelete, Path: ProjectsAPIPath + "/{project_id}/members/{user_id}"},
		{Method: http.MethodPut, Path: ProjectsAPIPath + "/{project_id}/geofence"},
		{Method: http.MethodGet, Path: ProjectsAPIPath + "/{project_id}/tasks"},
		{Method: http.MethodPost, Path: ProjectsAPIPath + "/{project_id}/tasks"},
		{Method: http.MethodGet, Path: ProjectsAPIPath + "/{project_id}/tasks/{task_id}"},
		{Method: http.MethodPatch, Path: ProjectsAPIPath + "/{project_id}/tasks/{task_id}"},
		{Method: http.MethodGet, Path: ProjectsAPIPath + "/{project_id}/tasks/{task_id}/progress-updates"},
		{Method: http.MethodPost, Path: ProjectsAPIPath + "/{project_id}/tasks/{task_id}/progress-updates"},
		{Method: http.MethodGet, Path: ProjectsAPIPath + "/{project_id}/tasks/{task_id}/progress-updates/{update_id}"},
		{Method: http.MethodPatch, Path: ProjectsAPIPath + "/{project_id}/tasks/{task_id}/progress-updates/{update_id}"},
		{Method: http.MethodGet, Path: ProjectsAPIPath + "/{project_id}/tasks/{task_id}/progress-updates/{update_id}/attachments/{attachment_id}/content"},
	}
}

// Readiness is the stateful dependency boundary used by the readiness handler.
type Readiness interface {
	Check(context.Context) error
}

// Identity is the authentication policy boundary used by the HTTP transport.
type Identity interface {
	BootstrapAvailable(context.Context) (bool, error)
	BootstrapAdmin(context.Context, identity.BootstrapInput, identity.AuditContext) (identity.User, error)
	Login(context.Context, identity.LoginInput, identity.AuditContext) (identity.LoginResult, error)
	CurrentSession(context.Context, string) (identity.Session, error)
	CSRFToken(string) string
	ValidateCSRF(string, string) error
	Logout(context.Context, string, identity.User, identity.AuditContext) error
	ListUsers(context.Context, identity.User, identity.AuditContext) ([]identity.User, error)
	CreateUser(context.Context, identity.User, identity.CreateUserInput, identity.AuditContext) (identity.CredentialResult, error)
	UpdateUser(context.Context, identity.User, string, identity.UpdateUserInput, identity.AuditContext) (identity.User, error)
	ResetPassword(context.Context, identity.User, string, identity.AuditContext) (identity.CredentialResult, error)
	ChangePassword(context.Context, identity.User, identity.ChangePasswordInput, identity.AuditContext) error
	ListIdentityAudit(context.Context, identity.User, identity.AuditContext) ([]identity.AuditRecord, error)
}

// ProjectAccess is the project authorization and policy boundary used by HTTP.
type ProjectAccess interface {
	ListProjects(context.Context, identity.User, projects.AuditContext) ([]projects.Project, error)
	GetProject(context.Context, identity.User, string, projects.AuditContext) (projects.Project, error)
	CreateProject(context.Context, identity.User, projects.CreateProjectInput, projects.AuditContext) (projects.Project, error)
	UpdateProject(context.Context, identity.User, string, projects.UpdateProjectInput, projects.AuditContext) (projects.Project, error)
	ListMembers(context.Context, identity.User, string, projects.AuditContext) ([]projects.Member, error)
	AddMember(context.Context, identity.User, string, string, projects.AuditContext) (projects.Member, error)
	RemoveMember(context.Context, identity.User, string, string, projects.AuditContext) error
	ReplaceGeofence(context.Context, identity.User, string, projects.ReplaceGeofenceInput, projects.AuditContext) (projects.Geofence, error)
	ListTasks(context.Context, identity.User, string, projects.AuditContext) ([]projects.Task, error)
	GetTask(context.Context, identity.User, string, string, projects.AuditContext) (projects.Task, error)
	CreateTask(context.Context, identity.User, string, projects.CreateTaskInput, projects.AuditContext) (projects.Task, error)
	UpdateTask(context.Context, identity.User, string, string, projects.UpdateTaskInput, projects.AuditContext) (projects.Task, error)
}

type ProgressAccess interface {
	List(context.Context, identity.User, string, string, progress.AuditContext) ([]progress.Update, error)
	Get(context.Context, identity.User, string, string, string, progress.AuditContext) (progress.Update, error)
	Create(context.Context, identity.User, string, string, string, progress.CreateMetadata, []progress.UploadFile, progress.AuditContext) (progress.Update, error)
	Update(context.Context, identity.User, string, string, string, progress.UpdateInput, progress.AuditContext) (progress.Update, error)
	Download(context.Context, identity.User, string, string, string, string, progress.AuditContext) (progress.Download, error)
}

// Options contains the explicit dependencies needed by the HTTP transport.
type Options struct {
	AppName            string
	BasePath           string
	APIDocsEnabled     bool
	Logger             *slog.Logger
	Readiness          Readiness
	Identity           Identity
	Projects           ProjectAccess
	Progress           ProgressAccess
	AttachmentMaxBytes int64
	AttachmentMaxCount int
	Production         bool
}

// New constructs the complete foundation HTTP handler.
func New(options Options) (http.Handler, error) {
	if strings.TrimSpace(options.AppName) == "" {
		return nil, fmt.Errorf("application name is required")
	}
	if options.Logger == nil || options.Readiness == nil || options.Identity == nil || options.Projects == nil || options.Progress == nil || options.AttachmentMaxBytes < 1 || options.AttachmentMaxCount < 1 {
		return nil, fmt.Errorf("logger, readiness, identity, project, progress, and positive attachment limits are required")
	}
	templates, err := webui.Templates()
	if err != nil {
		return nil, err
	}
	static, err := webui.Static()
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", method(http.MethodGet, homeHandler(templates, options)))
	mux.HandleFunc("/login", methodSwitch(map[string]http.HandlerFunc{
		http.MethodGet: loginPageHandler(templates, options), http.MethodPost: loginFormHandler(options),
	}))
	mux.HandleFunc("/setup", methodSwitch(map[string]http.HandlerFunc{
		http.MethodGet: setupPageHandler(templates, options), http.MethodPost: setupFormHandler(options),
	}))
	mux.HandleFunc("/logout", method(http.MethodPost, logoutFormHandler(options)))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServerFS(static)))
	mux.HandleFunc(LivenessPath, method(http.MethodGet, livenessHandler))
	mux.HandleFunc(ReadinessPath, method(http.MethodGet, readinessHandler(options)))
	mux.HandleFunc(BootstrapPath, method(http.MethodPost, bootstrapAPIHandler(options)))
	mux.HandleFunc(LoginAPIPath, method(http.MethodPost, loginAPIHandler(options)))
	mux.HandleFunc(SessionPath, method(http.MethodGet, sessionAPIHandler(options)))
	mux.HandleFunc(LogoutAPIPath, method(http.MethodPost, logoutAPIHandler(options)))
	mux.HandleFunc(PasswordAPIPath, method(http.MethodPost, passwordAPIHandler(options)))
	mux.HandleFunc(AdminUsersAPIPath, methodSwitch(map[string]http.HandlerFunc{
		http.MethodGet: adminListUsersAPIHandler(options), http.MethodPost: adminCreateUserAPIHandler(options),
	}))
	mux.HandleFunc(AdminUsersAPIPath+"/", adminUserAPIRouter(options))
	mux.HandleFunc(AdminIdentityAuditAPIPath, method(http.MethodGet, adminIdentityAuditAPIHandler(options)))
	mux.HandleFunc(ProjectsAPIPath, methodSwitch(map[string]http.HandlerFunc{
		http.MethodGet: listProjectsAPIHandler(options), http.MethodPost: createProjectAPIHandler(options),
	}))
	mux.HandleFunc(ProjectsAPIPath+"/", projectAPIRouter(options))

	if options.APIDocsEnabled {
		openAPIDocument, err := openapiv1.DocumentForBasePath(options.BasePath)
		if err != nil {
			return nil, fmt.Errorf("prepare OpenAPI document: %w", err)
		}
		mux.HandleFunc(OpenAPIPath, method(http.MethodGet, openAPIHandler(openAPIDocument)))
		docsHandler := http.Handler(v5emb.New(options.AppName+" API", externalPath(options, OpenAPIPath), externalPath(options, APIDocsPath)))
		if options.BasePath != "" {
			docsHandler = prependRequestPath(options.BasePath, docsHandler)
		}
		mux.Handle(APIDocsPath, docsHandler)
		mux.HandleFunc(strings.TrimSuffix(APIDocsPath, "/"), method(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, externalPath(options, APIDocsPath), http.StatusTemporaryRedirect)
		}))
	}

	var handler http.Handler = mux
	if options.BasePath != "" {
		outer := http.NewServeMux()
		outer.Handle(options.BasePath+"/", http.StripPrefix(options.BasePath, mux))
		outer.HandleFunc(options.BasePath, method(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, options.BasePath+"/", http.StatusPermanentRedirect)
		}))
		handler = outer
	}

	return recoverPanics(options.Logger, requestContext(requestLog(options.Logger, securityHeaders(handler, options.BasePath)))), nil
}

func externalPath(options Options, path string) string {
	return options.BasePath + path
}

func prependRequestPath(prefix string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cloned := r.Clone(r.Context())
		cloned.URL.Path = prefix + r.URL.Path
		next.ServeHTTP(w, cloned)
	})
}

func homeHandler(templates *template.Template, options Options) http.HandlerFunc {
	type homeData struct {
		AppName        string
		BasePath       string
		APIDocsEnabled bool
		User           identity.User
		CSRFToken      string
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		token, session, ok := authenticate(r, options)
		if !ok {
			http.Redirect(w, r, externalPath(options, "/login"), http.StatusSeeOther)
			return
		}
		if session.User.MustChangePassword {
			http.Error(w, "Password change required", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := homeData{options.AppName, options.BasePath, options.APIDocsEnabled, session.User, options.Identity.CSRFToken(token)}
		if err := templates.ExecuteTemplate(w, "home", data); err != nil {
			options.Logger.Error("render home page", "error", err)
		}
	}
}

func livenessHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readinessHandler(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := options.Readiness.Check(r.Context()); err != nil {
			options.Logger.Warn("readiness check failed", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func openAPIHandler(document []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		http.ServeContent(w, r, "openapi.yaml", time.Time{}, strings.NewReader(string(document)))
	}
}

func method(expected string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != expected {
			w.Header().Set("Allow", expected)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}

func methodSwitch(handlers map[string]http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler, ok := handlers[r.Method]
		if !ok {
			allowed := make([]string, 0, len(handlers))
			for name := range handlers {
				allowed = append(allowed, name)
			}
			w.Header().Set("Allow", strings.Join(allowed, ", "))
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		handler(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func securityHeaders(next http.Handler, basePath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if strings.HasPrefix(r.URL.Path, basePath+APIDocsPath) {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'")
		} else {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self'; style-src 'self'; script-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		}
		next.ServeHTTP(w, r)
	})
}

func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		logger.Info("http request", "request_id", requestID(r.Context()), "method", r.Method, "path", r.URL.Path, "status", recorder.status, "duration_ms", time.Since(started).Milliseconds())
	})
}

type requestKey int

const requestIDKey requestKey = 1

func requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw [16]byte
		if _, err := rand.Read(raw[:]); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		id := fmt.Sprintf("%x", raw[:])
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func requestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func auditContext(r *http.Request) identity.AuditContext {
	return identity.AuditContext{RequestID: requestID(r.Context()), ClientIP: clientIP(r), UserAgent: r.UserAgent()}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(strings.TrimSpace(host))
	if peer != nil && peer.IsLoopback() {
		if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); net.ParseIP(forwarded) != nil {
			return forwarded
		}
	}
	if peer == nil {
		return "0.0.0.0"
	}
	return peer.String()
}

func recoverPanics(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("http panic recovered", "panic", recovered)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
