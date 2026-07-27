package httpserver

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/projects"
)

func listProjectsAPIHandler(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, session, ok := authenticate(r, options)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		items, err := options.Projects.ListProjects(r.Context(), session.User, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"projects": items})
	}
}

func createProjectAPIHandler(options Options) http.HandlerFunc {
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
		var input projects.CreateProjectInput
		if !decodeJSON(w, r, &input) {
			return
		}
		project, err := options.Projects.CreateProject(r.Context(), session.User, input, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"project": project})
	}
}

func projectAPIRouter(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, ProjectsAPIPath+"/"), "/"), "/")
		if len(parts) == 0 || !uuidPattern.MatchString(parts[0]) {
			http.NotFound(w, r)
			return
		}
		projectID := parts[0]
		switch {
		case len(parts) == 1 && r.Method == http.MethodGet:
			getProjectAPIHandler(options, projectID)(w, r)
		case len(parts) == 1 && r.Method == http.MethodPatch:
			updateProjectAPIHandler(options, projectID)(w, r)
		case len(parts) == 2 && parts[1] == "members" && r.Method == http.MethodGet:
			listProjectMembersAPIHandler(options, projectID)(w, r)
		case len(parts) == 3 && parts[1] == "members" && uuidPattern.MatchString(parts[2]) && (r.Method == http.MethodPut || r.Method == http.MethodDelete):
			mutateProjectMemberAPIHandler(options, projectID, parts[2])(w, r)
		case len(parts) == 2 && parts[1] == "geofence" && r.Method == http.MethodPut:
			replaceProjectGeofenceAPIHandler(options, projectID)(w, r)
		case len(parts) == 2 && parts[1] == "tasks" && (r.Method == http.MethodGet || r.Method == http.MethodPost):
			projectTasksAPIHandler(options, projectID)(w, r)
		case len(parts) == 3 && parts[1] == "tasks" && uuidPattern.MatchString(parts[2]) && (r.Method == http.MethodGet || r.Method == http.MethodPatch):
			projectTaskAPIHandler(options, projectID, parts[2])(w, r)
		case len(parts) == 4 && parts[1] == "tasks" && uuidPattern.MatchString(parts[2]) && parts[3] == "progress-updates" && (r.Method == http.MethodGet || r.Method == http.MethodPost):
			progressUpdatesAPIHandler(options, projectID, parts[2])(w, r)
		case len(parts) == 5 && parts[1] == "tasks" && uuidPattern.MatchString(parts[2]) && parts[3] == "progress-updates" && uuidPattern.MatchString(parts[4]) && (r.Method == http.MethodGet || r.Method == http.MethodPatch):
			progressUpdateAPIHandler(options, projectID, parts[2], parts[4])(w, r)
		case len(parts) == 8 && parts[1] == "tasks" && uuidPattern.MatchString(parts[2]) && parts[3] == "progress-updates" && uuidPattern.MatchString(parts[4]) && parts[5] == "attachments" && uuidPattern.MatchString(parts[6]) && parts[7] == "content" && r.Method == http.MethodGet:
			progressAttachmentDownloadHandler(options, projectID, parts[2], parts[4], parts[6])(w, r)
		case validProjectResourcePath(parts):
			w.Header().Set("Allow", allowedProjectMethods(parts))
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		default:
			http.NotFound(w, r)
		}
	}
}

func getProjectAPIHandler(options Options, projectID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, session, ok := authenticate(r, options)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		project, err := options.Projects.GetProject(r.Context(), session.User, projectID, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"project": project})
	}
}

func updateProjectAPIHandler(options Options, projectID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := authenticatedWrite(w, r, options)
		if !ok {
			return
		}
		var input projects.UpdateProjectInput
		if !decodeJSON(w, r, &input) {
			return
		}
		project, err := options.Projects.UpdateProject(r.Context(), session.User, projectID, input, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"project": project})
	}
}

func listProjectMembersAPIHandler(options Options, projectID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, session, ok := authenticate(r, options)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		members, err := options.Projects.ListMembers(r.Context(), session.User, projectID, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"members": members})
	}
}

func mutateProjectMemberAPIHandler(options Options, projectID, userID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := authenticatedWrite(w, r, options)
		if !ok {
			return
		}
		if r.Method == http.MethodPut {
			member, err := options.Projects.AddMember(r.Context(), session.User, projectID, userID, auditContext(r))
			if err != nil {
				writeProjectError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"member": member})
			return
		}
		if err := options.Projects.RemoveMember(r.Context(), session.User, projectID, userID, auditContext(r)); err != nil {
			writeProjectError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func replaceProjectGeofenceAPIHandler(options Options, projectID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := authenticatedWrite(w, r, options)
		if !ok {
			return
		}
		var input projects.ReplaceGeofenceInput
		if !decodeJSON(w, r, &input) {
			return
		}
		geofence, err := options.Projects.ReplaceGeofence(r.Context(), session.User, projectID, input, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"geofence": geofence})
	}
}

func authenticatedWrite(w http.ResponseWriter, r *http.Request, options Options) (identity.Session, bool) {
	token, session, ok := authenticate(r, options)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
		return identity.Session{}, false
	}
	if options.Identity.ValidateCSRF(token, r.Header.Get("X-CSRF-Token")) != nil {
		writeError(w, http.StatusForbidden, "csrf_invalid", "The CSRF token is invalid.")
		return identity.Session{}, false
	}
	return session, true
}

func writeProjectError(w http.ResponseWriter, err error) {
	var validation *projects.ValidationError
	switch {
	case errors.As(err, &validation):
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", validation.Error())
	case errors.Is(err, identity.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
	case errors.Is(err, projects.ErrPasswordChangeRequired):
		writeError(w, http.StatusForbidden, "password_change_required", "Replace the temporary password before continuing.")
	case errors.Is(err, projects.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "This operation is not permitted.")
	case errors.Is(err, projects.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "The requested resource was not found in the authorized project scope.")
	case errors.Is(err, projects.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "The resource already exists or changed since it was loaded.")
	case errors.Is(err, projects.ErrInvalidMember):
		writeError(w, http.StatusUnprocessableEntity, "invalid_member", "The target must be an enabled Member account.")
	case errors.Is(err, projects.ErrInvalidResponsible):
		writeError(w, http.StatusUnprocessableEntity, "invalid_responsible_member", "The responsible user must be a current enabled project Member.")
	case errors.Is(err, projects.ErrInactiveProject):
		writeError(w, http.StatusConflict, "project_inactive", "Tasks cannot be changed in an inactive project.")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
	}
}

func validProjectResourcePath(parts []string) bool {
	return len(parts) == 1 ||
		(len(parts) == 2 && (parts[1] == "members" || parts[1] == "geofence" || parts[1] == "tasks")) ||
		(len(parts) == 3 && (parts[1] == "members" || parts[1] == "tasks") && uuidPattern.MatchString(parts[2])) ||
		(len(parts) == 4 && parts[1] == "tasks" && uuidPattern.MatchString(parts[2]) && parts[3] == "progress-updates") ||
		(len(parts) == 5 && parts[1] == "tasks" && uuidPattern.MatchString(parts[2]) && parts[3] == "progress-updates" && uuidPattern.MatchString(parts[4])) ||
		(len(parts) == 8 && parts[1] == "tasks" && uuidPattern.MatchString(parts[2]) && parts[3] == "progress-updates" && uuidPattern.MatchString(parts[4]) && parts[5] == "attachments" && uuidPattern.MatchString(parts[6]) && parts[7] == "content")
}

func allowedProjectMethods(parts []string) string {
	switch {
	case len(parts) == 1:
		return "GET, PATCH"
	case len(parts) == 2 && parts[1] == "members":
		return "GET"
	case len(parts) == 2 && parts[1] == "tasks":
		return "GET, POST"
	case len(parts) == 3 && parts[1] == "members":
		return "PUT, DELETE"
	case len(parts) == 3 && parts[1] == "tasks":
		return "GET, PATCH"
	case len(parts) == 4 && parts[3] == "progress-updates":
		return "GET, POST"
	case len(parts) == 5 && parts[3] == "progress-updates":
		return "GET, PATCH"
	case len(parts) == 8 && parts[5] == "attachments":
		return "GET"
	default:
		return "PUT"
	}
}
