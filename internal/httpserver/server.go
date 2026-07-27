package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	openapiv1 "github.com/Transmogriffy-Global-Private-Limited/project-progress-register/api/openapi/v1"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/webui"
	"github.com/swaggest/swgui/v5emb"
)

const (
	LivenessPath  = "/api/v1/health/live"
	ReadinessPath = "/api/v1/health/ready"
	OpenAPIPath   = "/api/openapi/v1/openapi.yaml"
	APIDocsPath   = "/api/docs/"
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
	}
}

// Readiness is the stateful dependency boundary used by the readiness handler.
type Readiness interface {
	Check(context.Context) error
}

// Options contains the explicit dependencies needed by the HTTP transport.
type Options struct {
	AppName        string
	APIDocsEnabled bool
	Logger         *slog.Logger
	Readiness      Readiness
}

// New constructs the complete foundation HTTP handler.
func New(options Options) (http.Handler, error) {
	if strings.TrimSpace(options.AppName) == "" {
		return nil, fmt.Errorf("application name is required")
	}
	if options.Logger == nil || options.Readiness == nil {
		return nil, fmt.Errorf("logger and readiness checker are required")
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
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServerFS(static)))
	mux.HandleFunc(LivenessPath, method(http.MethodGet, livenessHandler))
	mux.HandleFunc(ReadinessPath, method(http.MethodGet, readinessHandler(options)))

	if options.APIDocsEnabled {
		mux.HandleFunc(OpenAPIPath, method(http.MethodGet, openAPIHandler))
		mux.Handle(APIDocsPath, v5emb.New(options.AppName+" API", OpenAPIPath, APIDocsPath))
		mux.HandleFunc(strings.TrimSuffix(APIDocsPath, "/"), method(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, APIDocsPath, http.StatusTemporaryRedirect)
		}))
	}

	return recoverPanics(options.Logger, requestLog(options.Logger, securityHeaders(mux))), nil
}

func homeHandler(templates *template.Template, options Options) http.HandlerFunc {
	data := struct {
		AppName        string
		APIDocsEnabled bool
	}{options.AppName, options.APIDocsEnabled}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
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

func openAPIHandler(w http.ResponseWriter, r *http.Request) {
	document := openapiv1.Document()
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "openapi.yaml", time.Time{}, strings.NewReader(string(document)))
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if strings.HasPrefix(r.URL.Path, APIDocsPath) {
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
		logger.Info("http request", "method", r.Method, "path", r.URL.Path, "status", recorder.status, "duration_ms", time.Since(started).Milliseconds())
	})
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
