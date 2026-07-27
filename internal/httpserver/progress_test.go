package httpserver

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProgressMultipartContract(t *testing.T) {
	handler := testHandler(t, false, nil)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("metadata", `{"content_markdown":"Work done","location":{"latitude":22.5726,"longitude":88.3639,"accuracy_metres":5,"browser_observed_at":null},"location_unavailable_reason":null,"attachments":[{"source":"camera","browser_last_modified_at":null}]}`); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("files", "photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("jpeg bytes"))
	_ = writer.Close()
	path := ProjectsAPIPath + "/" + testProjectID + "/tasks/" + testTaskID + "/progress-updates"
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Idempotency-Key", "idempotency-key-1234")
	request.Header.Set("X-CSRF-Token", "csrf-token")
	request.AddCookie(&http.Cookie{Name: SessionCookie, Value: "session-token"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("response=%d %s", response.Code, response.Body.String())
	}
}

func TestProgressFilesWithoutMetadataAreRejected(t *testing.T) {
	handler := testHandler(t, false, nil)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("files", "photo.jpg")
	_, _ = part.Write([]byte("jpeg"))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, ProjectsAPIPath+"/"+testProjectID+"/tasks/"+testTaskID+"/progress-updates", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("X-CSRF-Token", "csrf-token")
	request.AddCookie(&http.Cookie{Name: SessionCookie, Value: "session-token"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("response=%d %s", response.Code, response.Body.String())
	}
}
