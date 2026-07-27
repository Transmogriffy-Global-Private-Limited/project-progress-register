package httpserver

import (
	"net/http"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/projects"
)

func projectTasksAPIHandler(options Options, projectID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, session, ok := authenticate(r, options)
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
				return
			}
			tasks, err := options.Projects.ListTasks(r.Context(), session.User, projectID, auditContext(r))
			if err != nil {
				writeProjectError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
			return
		}
		session, ok := authenticatedWrite(w, r, options)
		if !ok {
			return
		}
		var input projects.CreateTaskInput
		if !decodeJSON(w, r, &input) {
			return
		}
		task, err := options.Projects.CreateTask(r.Context(), session.User, projectID, input, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"task": task})
	}
}

func projectTaskAPIHandler(options Options, projectID, taskID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, session, ok := authenticate(r, options)
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
				return
			}
			task, err := options.Projects.GetTask(r.Context(), session.User, projectID, taskID, auditContext(r))
			if err != nil {
				writeProjectError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"task": task})
			return
		}
		session, ok := authenticatedWrite(w, r, options)
		if !ok {
			return
		}
		var input projects.UpdateTaskInput
		if !decodeJSON(w, r, &input) {
			return
		}
		task, err := options.Projects.UpdateTask(r.Context(), session.User, projectID, taskID, input, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"task": task})
	}
}
