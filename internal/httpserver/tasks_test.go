package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testTaskID = "44444444-4444-4444-8444-444444444444"

func TestTaskAPIContract(t *testing.T) {
	t.Parallel()
	handler := testHandler(t, false, nil)
	base := ProjectsAPIPath + "/" + testProjectID + "/tasks"

	for _, test := range []struct {
		method string
		path   string
		body   string
		status int
		find   string
	}{
		{http.MethodGet, base, "", http.StatusOK, `"tasks"`},
		{http.MethodPost, base, `{"name":"Foundation","goals_markdown":"**Goal**","description_markdown":"Description","responsible_user_id":null,"target_date":"2026-08-01"}`, http.StatusCreated, testTaskID},
		{http.MethodGet, base + "/" + testTaskID, "", http.StatusOK, `"task"`},
		{http.MethodPatch, base + "/" + testTaskID, `{"name":"Foundation updated","goals_markdown":"Goal","description_markdown":"Description","responsible_user_id":null,"target_date":null,"expected_version":1}`, http.StatusOK, `"version":2`},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, authenticatedProjectRequest(test.method, test.path, test.body))
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.find) {
			t.Fatalf("%s %s = %d %s", test.method, test.path, response.Code, response.Body.String())
		}
	}
}

func TestTaskWriteRequiresCSRF(t *testing.T) {
	t.Parallel()
	handler := testHandler(t, false, nil)
	request := httptest.NewRequest(http.MethodPost, ProjectsAPIPath+"/"+testProjectID+"/tasks", strings.NewReader(`{"name":"Task","goals_markdown":"","description_markdown":"","responsible_user_id":null,"target_date":null}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: SessionCookie, Value: "session-token"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "csrf_invalid") {
		t.Fatalf("task create without CSRF = %d %s", response.Code, response.Body.String())
	}
}
