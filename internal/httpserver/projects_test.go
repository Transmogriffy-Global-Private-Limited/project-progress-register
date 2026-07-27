package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testProjectID = "11111111-1111-4111-8111-111111111111"

func TestProjectAPIContract(t *testing.T) {
	t.Parallel()
	handler := testHandler(t, false, nil)

	list := authenticatedProjectRequest(http.MethodGet, ProjectsAPIPath, "")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, list)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Site Alpha") {
		t.Fatalf("list projects = %d %s", response.Code, response.Body.String())
	}

	create := authenticatedProjectRequest(http.MethodPost, ProjectsAPIPath, `{"name":"Site Alpha","description_markdown":"Internal project"}`)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, create)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), testProjectID) {
		t.Fatalf("create project = %d %s", response.Code, response.Body.String())
	}

	update := authenticatedProjectRequest(http.MethodPatch, ProjectsAPIPath+"/"+testProjectID, `{"name":"Site Alpha","description_markdown":"Updated","active":true,"expected_version":1}`)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, update)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"version":2`) {
		t.Fatalf("update project = %d %s", response.Code, response.Body.String())
	}
}

func TestProjectMembershipAndGeofenceAPIContract(t *testing.T) {
	t.Parallel()
	handler := testHandler(t, false, nil)
	memberID := "22222222-2222-4222-8222-222222222222"

	for _, test := range []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodGet, ProjectsAPIPath + "/" + testProjectID + "/members", "", http.StatusOK},
		{http.MethodPut, ProjectsAPIPath + "/" + testProjectID + "/members/" + memberID, `{}`, http.StatusCreated},
		{http.MethodDelete, ProjectsAPIPath + "/" + testProjectID + "/members/" + memberID, `{}`, http.StatusNoContent},
		{http.MethodPut, ProjectsAPIPath + "/" + testProjectID + "/geofence", `{"latitude":22.5726,"longitude":88.3639,"radius_metres":250,"max_accuracy_metres":30,"expected_version":0}`, http.StatusOK},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, authenticatedProjectRequest(test.method, test.path, test.body))
		if response.Code != test.status {
			t.Fatalf("%s %s = %d %s", test.method, test.path, response.Code, response.Body.String())
		}
	}
}

func TestProjectWritesRequireCSRF(t *testing.T) {
	t.Parallel()
	handler := testHandler(t, false, nil)
	request := httptest.NewRequest(http.MethodPost, ProjectsAPIPath, strings.NewReader(`{"name":"Site Alpha","description_markdown":""}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: SessionCookie, Value: "session-token"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "csrf_invalid") {
		t.Fatalf("create without CSRF = %d %s", response.Code, response.Body.String())
	}
}

func authenticatedProjectRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.AddCookie(&http.Cookie{Name: SessionCookie, Value: "session-token"})
	if method != http.MethodGet {
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-CSRF-Token", "csrf-token")
	}
	return request
}
