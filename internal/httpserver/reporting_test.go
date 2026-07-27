package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReportingRoutes(t *testing.T) {
	handler := testHandler(t, false, nil)
	for _, item := range []struct {
		path string
		want string
	}{
		{path: DashboardAPIPath, want: `"projects":[]`},
		{path: AdminAuditAPIPath + "?limit=25&outcome=succeeded", want: `"audit_events":[]`},
	} {
		request := httptest.NewRequest(http.MethodGet, item.path, nil)
		request.AddCookie(&http.Cookie{Name: SessionCookie, Value: "session-token"})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), item.want) {
			t.Fatalf("GET %s = %d %s", item.path, response.Code, response.Body.String())
		}
	}
}

func TestTaskTimelineRoute(t *testing.T) {
	handler := testHandler(t, false, nil)
	path := ProjectsAPIPath + "/" + testProjectID + "/tasks/" + testTaskID + "/timeline"
	request := httptest.NewRequest(http.MethodGet, path+"?limit=25", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookie, Value: "session-token"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"timeline":[]`) {
		t.Fatalf("GET %s = %d %s", path, response.Code, response.Body.String())
	}
}

func TestTaskTimelineRouteRejectsInvalidLimit(t *testing.T) {
	handler := testHandler(t, false, nil)
	path := ProjectsAPIPath + "/" + testProjectID + "/tasks/" + testTaskID + "/timeline?limit=unbounded"
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.AddCookie(&http.Cookie{Name: SessionCookie, Value: "session-token"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), `"code":"validation_failed"`) {
		t.Fatalf("GET %s = %d %s", path, response.Code, response.Body.String())
	}
}
