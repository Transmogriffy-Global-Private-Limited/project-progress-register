package httpserver

import (
	"net/http"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/progress"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/projects"
)

func progressCommentsAPIHandler(options Options, projectID, taskID, updateID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, session, ok := authenticate(r, options)
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
				return
			}
			comments, err := options.Progress.ListComments(r.Context(), session.User, projectID, taskID, updateID, auditContext(r))
			if err != nil {
				writeProgressError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"comments": comments})
			return
		}
		session, ok := authenticatedWrite(w, r, options)
		if !ok {
			return
		}
		var input progress.CreateCommentInput
		if !decodeJSON(w, r, &input) {
			return
		}
		comment, err := options.Progress.CreateComment(r.Context(), session.User, projectID, taskID, updateID, input, auditContext(r))
		if err != nil {
			writeProgressError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"comment": comment})
	}
}

func acceptSuggestionAPIHandler(options Options, projectID, taskID, updateID, commentID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := authenticatedWrite(w, r, options)
		if !ok {
			return
		}
		suggestion, created, err := options.Progress.AcceptSuggestion(r.Context(), session.User, projectID, taskID, updateID, commentID, auditContext(r))
		if err != nil {
			writeProgressError(w, err)
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		writeJSON(w, status, map[string]any{"accepted_suggestion": suggestion, "created": created})
	}
}

func acceptedSuggestionsAPIHandler(options Options, projectID, taskID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, session, ok := authenticate(r, options)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		items, err := options.Progress.ListAcceptedSuggestions(r.Context(), session.User, projectID, taskID, auditContext(r))
		if err != nil {
			writeProgressError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"accepted_suggestions": items})
	}
}

func taskAssessmentAPIHandler(options Options, projectID, taskID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, session, ok := authenticate(r, options)
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
				return
			}
			assessment, err := options.Projects.GetCurrentAssessment(r.Context(), session.User, projectID, taskID, auditContext(r))
			if err != nil {
				writeProjectError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"assessment": assessment})
			return
		}
		session, ok := authenticatedWrite(w, r, options)
		if !ok {
			return
		}
		var input projects.SetAssessmentInput
		if !decodeJSON(w, r, &input) {
			return
		}
		assessment, err := options.Projects.SetAssessment(r.Context(), session.User, projectID, taskID, input, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"assessment": assessment})
	}
}

func taskAssessmentHistoryAPIHandler(options Options, projectID, taskID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, session, ok := authenticate(r, options)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		items, err := options.Projects.ListAssessments(r.Context(), session.User, projectID, taskID, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"assessments": items})
	}
}
