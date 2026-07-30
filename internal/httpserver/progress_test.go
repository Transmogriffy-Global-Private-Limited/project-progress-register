package httpserver

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/progress"
)

type attachmentContent struct{ *bytes.Reader }

func (*attachmentContent) Close() error { return nil }

func TestProgressAttachmentContentPathUsesDeploymentBasePath(t *testing.T) {
	handler := testHandlerAtBasePath(t, false, nil, "/backend")
	path := "/backend" + ProjectsAPIPath + "/" + testProjectID + "/tasks/" + testTaskID + "/progress-updates/55555555-5555-4555-8555-555555555555"
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.AddCookie(&http.Cookie{Name: SessionCookie, Value: "session-token"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("response=%d %s", response.Code, response.Body.String())
	}
	var payload struct {
		ProgressUpdate struct {
			Attachments []struct {
				ContentPath string `json:"content_path"`
			} `json:"attachments"`
		} `json:"progress_update"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	expected := "/backend/api/v1/projects/" + testProjectID + "/tasks/" + testTaskID + "/progress-updates/55555555-5555-4555-8555-555555555555/attachments/66666666-6666-4666-8666-666666666666/content"
	if len(payload.ProgressUpdate.Attachments) != 1 || payload.ProgressUpdate.Attachments[0].ContentPath != expected {
		t.Fatalf("content_path=%q, expected %q", payload.ProgressUpdate.Attachments[0].ContentPath, expected)
	}
}

func TestVideoAttachmentSupportsAuthorizedInlineRangeStreaming(t *testing.T) {
	payload := []byte("0123456789abcdef")
	download := progress.Download{
		Attachment: progress.Attachment{OriginalName: "camera.mp4", DetectedMIME: "video/mp4", MediaKind: "video"},
		Content:    &attachmentContent{Reader: bytes.NewReader(payload)},
	}
	options := Options{Identity: fakeIdentity{}, Progress: fakeProgress{download: &download}}
	handler := progressAttachmentDownloadHandler(options, testProjectID, testTaskID, "update", "attachment")
	request := httptest.NewRequest(http.MethodGet, "/content", nil)
	request.Header.Set("Range", "bytes=2-5")
	request.AddCookie(&http.Cookie{Name: SessionCookie, Value: "session-token"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusPartialContent || response.Body.String() != "2345" {
		t.Fatalf("response=%d body=%q", response.Code, response.Body.String())
	}
	if disposition := response.Header().Get("Content-Disposition"); !strings.HasPrefix(disposition, "inline;") {
		t.Fatalf("Content-Disposition=%q", disposition)
	}
	if response.Header().Get("Accept-Ranges") != "bytes" || response.Header().Get("Content-Range") != "bytes 2-5/16" {
		t.Fatalf("range headers=%v", response.Header())
	}
}

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
