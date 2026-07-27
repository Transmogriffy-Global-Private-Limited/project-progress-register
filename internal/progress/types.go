// Package progress owns task diary entries, revisions, evidence, and attachment metadata.
package progress

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

var (
	ErrNotFound              = errors.New("progress resource not found")
	ErrConflict              = errors.New("progress resource conflict")
	ErrForbidden             = errors.New("progress operation forbidden")
	ErrInactiveProject       = errors.New("project is inactive")
	ErrPolicyChanged         = errors.New("geofence policy changed")
	ErrAttachmentPending     = errors.New("attachment finalization is pending")
	ErrAttachmentUnavailable = errors.New("attachment bytes are unavailable")
)

type ValidationError struct{ Field, Message string }

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

type Actor struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

type ReportedLocation struct {
	Latitude          float64    `json:"latitude"`
	Longitude         float64    `json:"longitude"`
	AccuracyMetres    float64    `json:"accuracy_metres"`
	BrowserObservedAt *time.Time `json:"browser_observed_at"`
}

type Evidence struct {
	LocationStatus            string            `json:"location_status"`
	LocationReason            string            `json:"location_reason"`
	ReportedLocation          *ReportedLocation `json:"reported_location"`
	LocationUnavailableReason *string           `json:"location_unavailable_reason"`
	Geofence                  *GeofenceSnapshot `json:"geofence"`
	ComputedDistanceMetres    *float64          `json:"computed_distance_metres"`
}

type GeofenceSnapshot struct {
	ID                string  `json:"id"`
	Version           int64   `json:"version"`
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
	RadiusMetres      float64 `json:"radius_metres"`
	MaxAccuracyMetres float64 `json:"max_accuracy_metres"`
}

type Attachment struct {
	ID                    string         `json:"id"`
	OriginalName          string         `json:"original_name"`
	ReportedMIME          string         `json:"reported_mime"`
	DetectedMIME          string         `json:"detected_mime"`
	MediaKind             string         `json:"media_kind"`
	Source                string         `json:"source"`
	SourceTrust           string         `json:"source_trust"`
	VerificationStatus    string         `json:"verification_status"`
	VerificationReason    string         `json:"verification_reason"`
	SizeBytes             int64          `json:"size_bytes"`
	SHA256                string         `json:"sha256"`
	BrowserLastModifiedAt *time.Time     `json:"browser_last_modified_at"`
	EmbeddedMetadata      map[string]any `json:"embedded_metadata"`
	EmbeddedMetadataTrust string         `json:"embedded_metadata_trust"`
	StorageState          string         `json:"storage_state"`
	FailureReason         string         `json:"failure_reason"`
	ContentPath           string         `json:"content_path"`
	CreatedAt             time.Time      `json:"created_at"`
	AvailableAt           *time.Time     `json:"available_at"`
	storageKey            string
}

type Revision struct {
	ID                      string    `json:"id"`
	FromVersion             int64     `json:"from_version"`
	ToVersion               int64     `json:"to_version"`
	PreviousContentMarkdown string    `json:"previous_content_markdown"`
	PreviousContentHTML     string    `json:"previous_content_html"`
	NewContentMarkdown      string    `json:"new_content_markdown"`
	NewContentHTML          string    `json:"new_content_html"`
	EditedBy                Actor     `json:"edited_by"`
	EditedAt                time.Time `json:"edited_at"`
}

type Update struct {
	ID              string       `json:"id"`
	ProjectID       string       `json:"project_id"`
	TaskID          string       `json:"task_id"`
	ContentMarkdown string       `json:"content_markdown"`
	ContentHTML     string       `json:"content_html"`
	CreatedBy       Actor        `json:"created_by"`
	Evidence        Evidence     `json:"evidence"`
	Attachments     []Attachment `json:"attachments"`
	Revisions       []Revision   `json:"revisions"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
	Version         int64        `json:"version"`
}

type AttachmentDescriptor struct {
	Source                string     `json:"source"`
	BrowserLastModifiedAt *time.Time `json:"browser_last_modified_at"`
}

type CreateMetadata struct {
	ContentMarkdown           string                 `json:"content_markdown"`
	Location                  *ReportedLocation      `json:"location"`
	LocationUnavailableReason *string                `json:"location_unavailable_reason"`
	Attachments               []AttachmentDescriptor `json:"attachments"`
}

type UploadFile struct {
	OriginalName string
	ReportedMIME string
	Open         func() (io.ReadCloser, error)
}

type UpdateInput struct {
	ContentMarkdown string `json:"content_markdown"`
	ExpectedVersion int64  `json:"expected_version"`
}

type AuditContext = identity.AuditContext

type TaskPolicy struct {
	Active   bool
	Geofence *GeofenceSnapshot
}

type attachmentPersistence struct {
	Attachment
	StorageKey string
}

type progressPersistence struct {
	ContentMarkdown string
	IdempotencyKey  string
	RequestSHA256   string
	Evidence        Evidence
	ExpectedVersion int64
}

type pendingAttachment struct{ ID, StorageKey, UploadedBy string }

type auditEvent struct {
	ActorUserID, Action, TargetType, TargetID, Outcome string
	Context                                            AuditContext
	Details                                            map[string]any
}

func validateContent(value string) (string, error) {
	if strings.TrimSpace(value) == "" || len(value) > 50000 {
		return "", &ValidationError{Field: "content_markdown", Message: "must contain text and not exceed 50000 UTF-8 bytes"}
	}
	return value, nil
}

func validateLocation(location *ReportedLocation, unavailable *string) error {
	if location != nil && unavailable != nil {
		return &ValidationError{Field: "location", Message: "cannot be combined with location_unavailable_reason"}
	}
	if location != nil {
		values := []struct {
			name            string
			value, min, max float64
		}{
			{"latitude", location.Latitude, -90, 90}, {"longitude", location.Longitude, -180, 180}, {"accuracy_metres", location.AccuracyMetres, 0.1, 100000},
		}
		for _, item := range values {
			if math.IsNaN(item.value) || math.IsInf(item.value, 0) || item.value < item.min || item.value > item.max {
				return &ValidationError{Field: "location." + item.name, Message: fmt.Sprintf("must be between %g and %g", item.min, item.max)}
			}
		}
		return nil
	}
	if unavailable != nil {
		value := strings.TrimSpace(*unavailable)
		allowed := map[string]bool{"permission_denied": true, "timeout": true, "unavailable": true, "not_supported": true}
		if !allowed[value] {
			return &ValidationError{Field: "location_unavailable_reason", Message: "must be permission_denied, timeout, unavailable, not_supported, or null"}
		}
		*unavailable = value
	}
	return nil
}
