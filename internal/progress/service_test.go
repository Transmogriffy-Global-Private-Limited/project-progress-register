package progress

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/filestore"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

func TestCreateGeotagsEveryFileAndVerifiesOnlyCameraPhoto(t *testing.T) {
	repository := &fakeRepository{policy: TaskPolicy{Active: true, Geofence: &GeofenceSnapshot{ID: "geofence", Version: 1, Latitude: 22.5726, Longitude: 88.3639, RadiusMetres: 100, MaxAccuracyMetres: 20}}}
	storage := &fakeStorage{}
	service, _ := NewService(repository, storage, fakeRenderer{}, 10)
	location := &ReportedLocation{Latitude: 22.5726, Longitude: 88.3639, AccuracyMetres: 5}
	metadata := CreateMetadata{ContentMarkdown: "Work done", Location: location, Attachments: []AttachmentDescriptor{{Source: "camera"}, {Source: "upload"}, {Source: "upload"}}}
	files := []UploadFile{fakeUpload("photo.jpg"), fakeUpload("report.pdf"), fakeUpload("clip.mp4")}
	update, err := service.Create(context.Background(), identity.User{ID: "actor", Username: "member", Role: identity.RoleMember, Enabled: true}, "project", "task", "idempotency-key-1234", metadata, files, testAudit())
	if err != nil {
		t.Fatal(err)
	}
	if update.Evidence.ReportedLocation == nil || len(update.Attachments) != 3 {
		t.Fatalf("update=%#v", update)
	}
	if update.Attachments[0].VerificationStatus != "verified" {
		t.Fatalf("camera=%#v", update.Attachments[0])
	}
	for _, attachment := range update.Attachments[1:] {
		if attachment.VerificationStatus != "non_verified" {
			t.Fatalf("attachment=%#v", attachment)
		}
	}
}

func TestCreateFilesRequireLocationButTextDoesNot(t *testing.T) {
	service, _ := NewService(&fakeRepository{policy: TaskPolicy{Active: true}}, &fakeStorage{}, fakeRenderer{}, 10)
	actor := identity.User{ID: "actor", Role: identity.RoleMember, Enabled: true}
	_, err := service.Create(context.Background(), actor, "project", "task", "idempotency-key-1234", CreateMetadata{ContentMarkdown: "Text", Attachments: []AttachmentDescriptor{{Source: "upload"}}}, []UploadFile{fakeUpload("photo.jpg")}, testAudit())
	if err == nil || !strings.Contains(err.Error(), "location") {
		t.Fatalf("file without location error=%v", err)
	}
	_, err = service.Create(context.Background(), actor, "project", "task", "idempotency-key-5678", CreateMetadata{ContentMarkdown: "Text", Attachments: []AttachmentDescriptor{}}, nil, testAudit())
	if err != nil {
		t.Fatalf("text-only create error=%v", err)
	}
}

func TestCreateAuthorizesBeforeStagingFiles(t *testing.T) {
	repository := &fakeRepository{policyErr: ErrNotFound}
	storage := &fakeStorage{}
	service, _ := NewService(repository, storage, fakeRenderer{}, 10)
	location := &ReportedLocation{Latitude: 22.5726, Longitude: 88.3639, AccuracyMetres: 5}
	metadata := CreateMetadata{ContentMarkdown: "Work done", Location: location, Attachments: []AttachmentDescriptor{{Source: "camera"}}}
	_, err := service.Create(context.Background(), identity.User{ID: "actor", Role: identity.RoleMember, Enabled: true}, "project", "task", "idempotency-key-1234", metadata, []UploadFile{fakeUpload("photo.jpg")}, testAudit())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error=%v", err)
	}
	if storage.counter != 0 {
		t.Fatalf("staged files before authorization=%d", storage.counter)
	}
}

type fakeRepository struct {
	policy    TaskPolicy
	policyErr error
	captured  []attachmentPersistence
}

func (f *fakeRepository) GetTaskPolicy(context.Context, string, bool, string, string, bool) (TaskPolicy, error) {
	return f.policy, f.policyErr
}
func (f *fakeRepository) ListUpdates(context.Context, string, bool, string, string) ([]Update, error) {
	return []Update{}, nil
}
func (f *fakeRepository) GetUpdate(context.Context, string, bool, string, string, string) (Update, error) {
	return Update{}, ErrNotFound
}
func (f *fakeRepository) CreateUpdate(_ context.Context, actorID string, _ bool, projectID, taskID string, input progressPersistence, attachments []attachmentPersistence, _ auditEvent) (Update, bool, error) {
	f.captured = attachments
	result := Update{ID: "update", ProjectID: projectID, TaskID: taskID, ContentMarkdown: input.ContentMarkdown, CreatedBy: Actor{UserID: actorID}, Evidence: input.Evidence, Version: 1, Attachments: make([]Attachment, len(attachments)), Revisions: []Revision{}}
	for i := range attachments {
		result.Attachments[i] = attachments[i].Attachment
		result.Attachments[i].ID = string(rune('a' + i))
		result.Attachments[i].storageKey = attachments[i].StorageKey
	}
	return result, true, nil
}
func (f *fakeRepository) UpdateProgress(context.Context, string, bool, string, string, string, progressPersistence, auditEvent) (Update, error) {
	return Update{}, nil
}
func (f *fakeRepository) GetAttachment(context.Context, string, bool, string, string, string, string) (Attachment, error) {
	return Attachment{}, ErrNotFound
}
func (f *fakeRepository) MarkAttachmentAvailable(context.Context, string, auditEvent) (time.Time, error) {
	return time.Now(), nil
}
func (f *fakeRepository) MarkAttachmentFailed(context.Context, string, string, auditEvent) error {
	return nil
}
func (f *fakeRepository) ListPendingAttachments(context.Context) ([]pendingAttachment, error) {
	return nil, nil
}
func (f *fakeRepository) AppendAudit(context.Context, auditEvent) error { return nil }

type fakeStorage struct{ counter int }

func (f *fakeStorage) Stage(_ context.Context, _ io.Reader, name, mime string) (filestore.StagedFile, error) {
	f.counter++
	kind := "image"
	detected := "image/jpeg"
	if strings.HasSuffix(name, ".pdf") {
		kind, detected = "document", "application/pdf"
	}
	if strings.HasSuffix(name, ".mp4") {
		kind, detected = "video", "video/mp4"
	}
	return filestore.StagedFile{StorageKey: strings.Repeat(string(rune('a'+f.counter)), 64), OriginalName: name, ReportedMIME: mime, DetectedMIME: detected, MediaKind: kind, SizeBytes: 10, SHA256: strings.Repeat("a", 64)}, nil
}
func (*fakeStorage) Finalize(string) error { return nil }
func (*fakeStorage) Open(string) (filestore.ReadSeekCloser, error) {
	return &memoryFile{Reader: *bytes.NewReader([]byte("file"))}, nil
}
func (*fakeStorage) DiscardStaged(string) error                             { return nil }
func (*fakeStorage) CleanupOrphans(map[string]bool, time.Time) (int, error) { return 0, nil }

type memoryFile struct{ bytes.Reader }

func (*memoryFile) Close() error { return nil }

type fakeRenderer struct{}

func (fakeRenderer) Render(value string) (string, error) { return "<p>" + value + "</p>", nil }
func fakeUpload(name string) UploadFile {
	return UploadFile{OriginalName: name, ReportedMIME: "application/octet-stream", Open: func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("content")), nil }}
}
func testAudit() AuditContext {
	return AuditContext{RequestID: "request-12345678", ClientIP: "127.0.0.1", UserAgent: "test"}
}

var _ = time.Time{}
