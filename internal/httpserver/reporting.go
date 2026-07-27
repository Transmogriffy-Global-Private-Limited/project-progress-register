package httpserver

import (
	"net/http"
	"strconv"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/projects"
)

func dashboardAPIHandler(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, session, ok := authenticate(r, options)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		dashboard, err := options.Projects.GetDashboard(r.Context(), session.User, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, dashboard)
	}
}

func taskTimelineAPIHandler(options Options, projectID, taskID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, session, ok := authenticate(r, options)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		limit := 0
		if value := r.URL.Query().Get("limit"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				writeError(w, http.StatusUnprocessableEntity, "validation_failed", "limit: must be between 1 and 200")
				return
			}
			limit = parsed
		}
		page, err := options.Projects.GetTaskTimeline(r.Context(), session.User, projectID, taskID, projects.TimelineQuery{
			Limit:  limit,
			Cursor: r.URL.Query().Get("cursor"),
		}, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}
