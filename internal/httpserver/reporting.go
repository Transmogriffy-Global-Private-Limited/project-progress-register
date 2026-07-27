package httpserver

import "net/http"

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
		events, err := options.Projects.GetTaskTimeline(r.Context(), session.User, projectID, taskID, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"timeline": events})
	}
}
