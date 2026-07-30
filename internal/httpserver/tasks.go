package httpserver

import (
	"net/http"
	"strings"
	"time"

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
			writeJSON(w, http.StatusOK, map[string]any{"tasks": legacyTasks(tasks)})
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
		writeJSON(w, http.StatusCreated, map[string]any{"task": legacyTask(task)})
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
			writeJSON(w, http.StatusOK, map[string]any{"task": legacyTask(task)})
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
		writeJSON(w, http.StatusOK, map[string]any{"task": legacyTask(task)})
	}
}

type legacyTaskResponse struct {
	ID                  string                      `json:"id"`
	ProjectID           string                      `json:"project_id"`
	Name                string                      `json:"name"`
	GoalsMarkdown       string                      `json:"goals_markdown"`
	GoalsHTML           string                      `json:"goals_html"`
	DescriptionMarkdown string                      `json:"description_markdown"`
	DescriptionHTML     string                      `json:"description_html"`
	CreatedBy           projects.TaskActor          `json:"created_by"`
	ResponsibleMember   *projects.ResponsibleMember `json:"responsible_member"`
	TargetDate          *string                     `json:"target_date"`
	CreatedAt           time.Time                   `json:"created_at"`
	UpdatedAt           time.Time                   `json:"updated_at"`
	Version             int64                       `json:"version"`
}

func legacyTask(task projects.Task) legacyTaskResponse {
	var responsible *projects.ResponsibleMember
	if len(task.ResponsibleMembers) > 0 {
		value := task.ResponsibleMembers[0]
		responsible = &value
	}
	return legacyTaskResponse{
		ID: task.ID, ProjectID: task.ProjectID, Name: task.Name, GoalsMarkdown: task.GoalsMarkdown, GoalsHTML: task.GoalsHTML,
		DescriptionMarkdown: task.DescriptionMarkdown, DescriptionHTML: task.DescriptionHTML, CreatedBy: task.CreatedBy,
		ResponsibleMember: responsible, TargetDate: task.TargetDate, CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt, Version: task.Version,
	}
}

func legacyTasks(tasks []projects.Task) []legacyTaskResponse {
	result := make([]legacyTaskResponse, len(tasks))
	for index := range tasks {
		result[index] = legacyTask(tasks[index])
	}
	return result
}

func taskV2APIRouter(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, ProjectsV2APIPath+"/"), "/"), "/")
		if len(parts) < 2 || !uuidPattern.MatchString(parts[0]) || parts[1] != "tasks" {
			http.NotFound(w, r)
			return
		}
		projectID := parts[0]
		switch {
		case len(parts) == 2 && (r.Method == http.MethodGet || r.Method == http.MethodPost):
			projectTasksV2APIHandler(options, projectID)(w, r)
		case len(parts) == 3 && uuidPattern.MatchString(parts[2]) && (r.Method == http.MethodGet || r.Method == http.MethodPatch):
			projectTaskV2APIHandler(options, projectID, parts[2])(w, r)
		case len(parts) == 2:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		case len(parts) == 3 && uuidPattern.MatchString(parts[2]):
			w.Header().Set("Allow", "GET, PATCH")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		default:
			http.NotFound(w, r)
		}
	}
}

func projectTasksV2APIHandler(options Options, projectID string) http.HandlerFunc {
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
		var input projects.CreateTaskV2Input
		if !decodeJSON(w, r, &input) {
			return
		}
		task, err := options.Projects.CreateTaskV2(r.Context(), session.User, projectID, input, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"task": task})
	}
}

func projectTaskV2APIHandler(options Options, projectID, taskID string) http.HandlerFunc {
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
		var input projects.UpdateTaskV2Input
		if !decodeJSON(w, r, &input) {
			return
		}
		task, err := options.Projects.UpdateTaskV2(r.Context(), session.User, projectID, taskID, input, auditContext(r))
		if err != nil {
			writeProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"task": task})
	}
}
