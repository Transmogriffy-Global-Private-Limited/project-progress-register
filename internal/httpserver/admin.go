package httpserver

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func adminListUsersAPIHandler(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, session, ok := authenticate(r, options)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		users, err := options.Identity.ListUsers(r.Context(), session.User, auditContext(r))
		if err != nil {
			writeIdentityError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users})
	}
}

func adminIdentityAuditAPIHandler(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, session, ok := authenticate(r, options)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		records, err := options.Identity.ListIdentityAudit(r.Context(), session.User, auditContext(r))
		if err != nil {
			writeIdentityError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"audit_events": records})
	}
}

func adminCreateUserAPIHandler(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, session, ok := authenticate(r, options)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		if options.Identity.ValidateCSRF(token, r.Header.Get("X-CSRF-Token")) != nil {
			writeError(w, http.StatusForbidden, "csrf_invalid", "The CSRF token is invalid.")
			return
		}
		var input identity.CreateUserInput
		if !decodeJSON(w, r, &input) {
			return
		}
		result, err := options.Identity.CreateUser(r.Context(), session.User, input, auditContext(r))
		if err != nil {
			writeIdentityError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusCreated, result)
	}
}

func adminUserAPIRouter(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetID, reset, ok := parseUserActionPath(strings.TrimPrefix(r.URL.Path, AdminUsersAPIPath+"/"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		expectedMethod := http.MethodPatch
		if reset {
			expectedMethod = http.MethodPost
		}
		if r.Method != expectedMethod {
			w.Header().Set("Allow", expectedMethod)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		token, session, authenticated := authenticate(r, options)
		if !authenticated {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		if options.Identity.ValidateCSRF(token, r.Header.Get("X-CSRF-Token")) != nil {
			writeError(w, http.StatusForbidden, "csrf_invalid", "The CSRF token is invalid.")
			return
		}
		if reset {
			result, err := options.Identity.ResetPassword(r.Context(), session.User, targetID, auditContext(r))
			if err != nil {
				writeIdentityError(w, err)
				return
			}
			if targetID == session.User.ID {
				clearSessionCookie(w, options.Production, options.BasePath)
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, result)
			return
		}
		var input identity.UpdateUserInput
		if !decodeJSON(w, r, &input) {
			return
		}
		user, err := options.Identity.UpdateUser(r.Context(), session.User, targetID, input, auditContext(r))
		if err != nil {
			writeIdentityError(w, err)
			return
		}
		if targetID == session.User.ID {
			clearSessionCookie(w, options.Production, options.BasePath)
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": user})
	}
}

func passwordAPIHandler(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, session, ok := authenticate(r, options)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		if options.Identity.ValidateCSRF(token, r.Header.Get("X-CSRF-Token")) != nil {
			writeError(w, http.StatusForbidden, "csrf_invalid", "The CSRF token is invalid.")
			return
		}
		var input identity.ChangePasswordInput
		if !decodeJSON(w, r, &input) {
			return
		}
		if err := options.Identity.ChangePassword(r.Context(), session.User, input, auditContext(r)); err != nil {
			writeIdentityError(w, err)
			return
		}
		clearSessionCookie(w, options.Production, options.BasePath)
		writeJSON(w, http.StatusOK, map[string]bool{"password_changed": true, "logged_out": true})
	}
}

func parseUserActionPath(value string) (string, bool, bool) {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) == 1 && uuidPattern.MatchString(parts[0]) {
		return parts[0], false, true
	}
	if len(parts) == 2 && uuidPattern.MatchString(parts[0]) && parts[1] == "password-reset" {
		return parts[0], true, true
	}
	return "", false, false
}
