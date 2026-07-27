package httpserver

import (
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

type authPageData struct {
	AppName, Error     string
	BootstrapAvailable bool
}

func loginPageHandler(templates *template.Template, options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := authenticate(r, options); ok {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		available, _ := options.Identity.BootstrapAvailable(r.Context())
		render(w, templates, "login", authPageData{AppName: options.AppName, BootstrapAvailable: available}, options)
	}
}

func setupPageHandler(templates *template.Template, options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		available, err := options.Identity.BootstrapAvailable(r.Context())
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if !available {
			http.NotFound(w, r)
			return
		}
		render(w, templates, "setup", authPageData{AppName: options.AppName}, options)
	}
}

func loginFormHandler(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) || !parseForm(w, r) {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		result, err := options.Identity.Login(r.Context(), identity.LoginInput{Identifier: r.FormValue("identifier"), Password: r.FormValue("password")}, auditContext(r))
		if err != nil {
			http.Redirect(w, r, "/login?error=credentials", http.StatusSeeOther)
			return
		}
		setSessionCookie(w, result.SessionToken, result.ExpiresAt, options.Production)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func setupFormHandler(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) || !parseForm(w, r) {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		_, err := options.Identity.BootstrapAdmin(r.Context(), identity.BootstrapInput{BootstrapToken: r.FormValue("bootstrap_token"), Username: r.FormValue("username"), Email: r.FormValue("email"), Password: r.FormValue("password")}, auditContext(r))
		if err != nil {
			http.Redirect(w, r, "/setup?error=setup", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

func logoutFormHandler(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, session, ok := authenticate(r, options)
		if !ok {
			clearSessionCookie(w, options.Production)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !sameOrigin(r) || !parseForm(w, r) || options.Identity.ValidateCSRF(token, r.FormValue("_csrf")) != nil {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		if err := options.Identity.Logout(r.Context(), token, session.User, auditContext(r)); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		clearSessionCookie(w, options.Production)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

func bootstrapAPIHandler(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input identity.BootstrapInput
		if !decodeJSON(w, r, &input) {
			return
		}
		user, err := options.Identity.BootstrapAdmin(r.Context(), input, auditContext(r))
		if err != nil {
			writeIdentityError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"user": user})
	}
}

func loginAPIHandler(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input identity.LoginInput
		if !decodeJSON(w, r, &input) {
			return
		}
		result, err := options.Identity.Login(r.Context(), input, auditContext(r))
		if err != nil {
			writeIdentityError(w, err)
			return
		}
		setSessionCookie(w, result.SessionToken, result.ExpiresAt, options.Production)
		writeJSON(w, http.StatusOK, map[string]any{"user": result.User, "csrf_token": result.CSRFToken, "expires_at": result.ExpiresAt})
	}
}

func sessionAPIHandler(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, session, ok := authenticate(r, options)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": session.User, "csrf_token": options.Identity.CSRFToken(token), "expires_at": session.ExpiresAt})
	}
}

func logoutAPIHandler(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, session, ok := authenticate(r, options)
		if !ok {
			clearSessionCookie(w, options.Production)
			writeJSON(w, http.StatusOK, map[string]bool{"logged_out": true})
			return
		}
		if options.Identity.ValidateCSRF(token, r.Header.Get("X-CSRF-Token")) != nil {
			writeError(w, http.StatusForbidden, "csrf_invalid", "The CSRF token is invalid.")
			return
		}
		if err := options.Identity.Logout(r.Context(), token, session.User, auditContext(r)); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
			return
		}
		clearSessionCookie(w, options.Production)
		writeJSON(w, http.StatusOK, map[string]bool{"logged_out": true})
	}
}

func authenticate(r *http.Request, options Options) (string, identity.Session, bool) {
	cookie, err := r.Cookie(SessionCookie)
	if err != nil {
		return "", identity.Session{}, false
	}
	session, err := options.Identity.CurrentSession(r.Context(), cookie.Value)
	return cookie.Value, session, err == nil
}

func setSessionCookie(w http.ResponseWriter, token string, expires time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(time.Until(expires).Seconds())})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, Expires: time.Unix(1, 0), MaxAge: -1})
}

func sameOrigin(r *http.Request) bool {
	if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
		return false
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	return origin == "" || origin == "http://"+r.Host || origin == "https://"+r.Host
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if media := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])); media != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json.")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "The JSON request is invalid.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "The JSON request must contain one value.")
		return false
	}
	return true
}

func parseForm(w http.ResponseWriter, r *http.Request) bool {
	media := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	if media != "application/x-www-form-urlencoded" {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	return r.ParseForm() == nil
}

func writeIdentityError(w http.ResponseWriter, err error) {
	var validation *identity.ValidationError
	switch {
	case errors.As(err, &validation):
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", validation.Error())
	case errors.Is(err, identity.ErrInvalidCredentials), errors.Is(err, identity.ErrLoginThrottled):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "The credentials are invalid.")
	case errors.Is(err, identity.ErrBootstrapUnavailable):
		writeError(w, http.StatusNotFound, "bootstrap_unavailable", "Initial setup is unavailable.")
	case errors.Is(err, identity.ErrBootstrapDenied):
		writeError(w, http.StatusForbidden, "bootstrap_denied", "Initial setup was denied.")
	case errors.Is(err, identity.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "This operation is not permitted.")
	case errors.Is(err, identity.ErrPasswordChangeNeeded):
		writeError(w, http.StatusForbidden, "password_change_required", "Replace the temporary password before continuing.")
	case errors.Is(err, identity.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "The account already exists or changed since it was loaded.")
	case errors.Is(err, identity.ErrLastAdmin):
		writeError(w, http.StatusConflict, "last_admin", "The final enabled Admin cannot be disabled or demoted.")
	case errors.Is(err, identity.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "The requested account was not found.")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func render(w http.ResponseWriter, templates *template.Template, name string, data any, options Options) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, name, data); err != nil {
		options.Logger.Error("render page", "template", name, "error", err)
	}
}
