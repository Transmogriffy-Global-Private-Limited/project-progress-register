// Package projects owns project access, membership, and site geofence policy.
package projects

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

var (
	ErrForbidden              = errors.New("project operation forbidden")
	ErrPasswordChangeRequired = errors.New("password change required")
	ErrNotFound               = errors.New("project resource not found")
	ErrConflict               = errors.New("project resource conflict")
	ErrInvalidMember          = errors.New("target is not an enabled Member")
	ErrInactiveProject        = errors.New("project is inactive")
	ErrInvalidResponsible     = errors.New("responsible user is not a current enabled project Member")
	ErrTaskV2Required         = errors.New("task has multiple responsibilities and requires the V2 task API")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

type Project struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	DescriptionMarkdown string    `json:"description_markdown"`
	DescriptionHTML     string    `json:"description_html"`
	Active              bool      `json:"active"`
	CreatedBy           string    `json:"created_by"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	Version             int64     `json:"version"`
	Geofence            *Geofence `json:"geofence"`
}

type Geofence struct {
	ID                string    `json:"id"`
	Version           int64     `json:"version"`
	Latitude          float64   `json:"latitude"`
	Longitude         float64   `json:"longitude"`
	RadiusMetres      float64   `json:"radius_metres"`
	MaxAccuracyMetres float64   `json:"max_accuracy_metres"`
	ValidFrom         time.Time `json:"valid_from"`
}

type Member struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Enabled  bool      `json:"enabled"`
	AddedAt  time.Time `json:"added_at"`
}

type CreateProjectInput struct {
	Name                string `json:"name"`
	DescriptionMarkdown string `json:"description_markdown"`
}

type UpdateProjectInput struct {
	Name                string `json:"name"`
	DescriptionMarkdown string `json:"description_markdown"`
	Active              *bool  `json:"active"`
	ExpectedVersion     int64  `json:"expected_version"`
}

type ReplaceGeofenceInput struct {
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
	RadiusMetres      float64 `json:"radius_metres"`
	MaxAccuracyMetres float64 `json:"max_accuracy_metres"`
	ExpectedVersion   int64   `json:"expected_version"`
}

type AuditContext = identity.AuditContext

type auditEvent struct {
	ActorUserID string
	Action      string
	TargetType  string
	TargetID    string
	Outcome     string
	Context     AuditContext
	Details     map[string]any
}

func validateProject(name, description string) (string, string, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 120 {
		return "", "", &ValidationError{Field: "name", Message: "must contain 1-120 characters"}
	}
	if len(description) > 20000 {
		return "", "", &ValidationError{Field: "description_markdown", Message: "must not exceed 20000 UTF-8 bytes"}
	}
	return name, description, nil
}

func validateGeofence(input ReplaceGeofenceInput) error {
	values := []struct {
		field string
		value float64
		min   float64
		max   float64
	}{
		{"latitude", input.Latitude, -90, 90},
		{"longitude", input.Longitude, -180, 180},
		{"radius_metres", input.RadiusMetres, 1, 100000},
		{"max_accuracy_metres", input.MaxAccuracyMetres, 0.1, 10000},
	}
	for _, item := range values {
		if math.IsNaN(item.value) || math.IsInf(item.value, 0) || item.value < item.min || item.value > item.max {
			return &ValidationError{Field: item.field, Message: fmt.Sprintf("must be between %g and %g", item.min, item.max)}
		}
	}
	if input.ExpectedVersion < 0 {
		return &ValidationError{Field: "expected_version", Message: "must be zero or a positive integer"}
	}
	return nil
}
