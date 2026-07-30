package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/projects"
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

func TestTaskV2APIContract(t *testing.T) {
	t.Parallel()
	handler := testHandler(t, false, nil)
	base := ProjectsV2APIPath + "/" + testProjectID + "/tasks"
	first := "22222222-2222-4222-8222-222222222222"
	second := "33333333-3333-4333-8333-333333333333"

	create := httptest.NewRecorder()
	handler.ServeHTTP(create, authenticatedProjectRequest(http.MethodPost, base, `{"name":"Foundation","goals_markdown":"Goal","description_markdown":"Description","responsible_user_ids":["`+first+`","`+second+`"],"target_date":null}`))
	if create.Code != http.StatusCreated || !strings.Contains(create.Body.String(), `"responsible_members":[`) || strings.Contains(create.Body.String(), `"responsible_member":`) {
		t.Fatalf("V2 create = %d %s", create.Code, create.Body.String())
	}

	update := httptest.NewRecorder()
	handler.ServeHTTP(update, authenticatedProjectRequest(http.MethodPatch, base+"/"+testTaskID, `{"name":"Foundation","goals_markdown":"Goal","description_markdown":"Description","responsible_user_ids":["`+second+`"],"target_date":null,"expected_version":1}`))
	if update.Code != http.StatusOK || !strings.Contains(update.Body.String(), second) || strings.Contains(update.Body.String(), first) {
		t.Fatalf("V2 update = %d %s", update.Code, update.Body.String())
	}

	missingAssignments := httptest.NewRecorder()
	handler.ServeHTTP(missingAssignments, authenticatedProjectRequest(http.MethodPatch, base+"/"+testTaskID, `{"name":"Foundation","goals_markdown":"Goal","description_markdown":"Description","target_date":null,"expected_version":1}`))
	if missingAssignments.Code != http.StatusUnprocessableEntity || !strings.Contains(missingAssignments.Body.String(), "responsible_user_ids") {
		t.Fatalf("V2 missing assignments = %d %s", missingAssignments.Code, missingAssignments.Body.String())
	}
}

func TestLegacyTaskProjectionRemainsSingular(t *testing.T) {
	t.Parallel()
	task := projects.Task{ResponsibleMembers: []projects.ResponsibleMember{
		{UserID: "22222222-2222-4222-8222-222222222222", Username: "alpha", Enabled: true},
		{UserID: "33333333-3333-4333-8333-333333333333", Username: "beta", Enabled: true},
	}}
	legacy := legacyTask(task)
	if legacy.ResponsibleMember == nil || legacy.ResponsibleMember.UserID != task.ResponsibleMembers[0].UserID {
		t.Fatalf("legacy task = %#v", legacy)
	}
}
