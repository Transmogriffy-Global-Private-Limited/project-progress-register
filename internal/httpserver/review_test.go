package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReviewRoutesAndContracts(t *testing.T) {
	handler := testHandler(t, false, nil)
	updateBase := ProjectsAPIPath + "/" + testProjectID + "/tasks/" + testTaskID + "/progress-updates/55555555-5555-4555-8555-555555555555"
	taskBase := ProjectsAPIPath + "/" + testProjectID + "/tasks/" + testTaskID

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{name: "list comments", method: http.MethodGet, path: updateBase + "/comments", status: http.StatusOK},
		{name: "create comment", method: http.MethodPost, path: updateBase + "/comments", body: `{"content_markdown":"Please confirm"}`, status: http.StatusCreated},
		{name: "accept suggestion", method: http.MethodPost, path: updateBase + "/comments/77777777-7777-4777-8777-777777777777/accept", status: http.StatusCreated},
		{name: "list suggestions", method: http.MethodGet, path: taskBase + "/accepted-suggestions", status: http.StatusOK},
		{name: "current assessment", method: http.MethodGet, path: taskBase + "/assessment", status: http.StatusOK},
		{name: "set assessment", method: http.MethodPut, path: taskBase + "/assessment", body: `{"verdict":"on_track","remark_markdown":"Proceed","expected_version":0}`, status: http.StatusCreated},
		{name: "assessment history", method: http.MethodGet, path: taskBase + "/assessments", status: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.AddCookie(&http.Cookie{Name: SessionCookie, Value: "session-token"})
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			if test.method == http.MethodPost || test.method == http.MethodPut {
				request.Header.Set("X-CSRF-Token", "csrf-token")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("response=%d %s", response.Code, response.Body.String())
			}
		})
	}
}
