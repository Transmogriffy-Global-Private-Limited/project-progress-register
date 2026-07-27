// Package identity owns local accounts, authentication, sessions, CSRF, login
// throttling, and identity audit orchestration.
package identity

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

var (
	ErrUnauthenticated      = errors.New("unauthenticated")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrLoginThrottled       = errors.New("login throttled")
	ErrBootstrapUnavailable = errors.New("bootstrap unavailable")
	ErrBootstrapDenied      = errors.New("bootstrap denied")
	ErrCSRFInvalid          = errors.New("invalid CSRF token")
	ErrNotFound             = errors.New("not found")
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,31}$`)

// ValidationError describes one safe input-validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// User is the authenticated account projection used outside persistence.
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// Session is a current authenticated session and user projection.
type Session struct {
	ID        string
	User      User
	ExpiresAt time.Time
}

// AuditContext is trusted request metadata. It never contains request bodies or secrets.
type AuditContext struct {
	RequestID string
	ClientIP  string
	UserAgent string
}

// AuditEvent is an append-only security or business event.
type AuditEvent struct {
	ActorUserID string
	Action      string
	TargetType  string
	TargetID    string
	Outcome     string
	Context     AuditContext
	Details     map[string]any
}

// BootstrapInput creates the one and only initial Admin.
type BootstrapInput struct {
	BootstrapToken string `json:"bootstrap_token"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	Password       string `json:"password"`
}

// LoginInput authenticates by normalized username or email.
type LoginInput struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

// LoginResult contains the raw token only at the transport boundary.
type LoginResult struct {
	User         User
	SessionToken string
	CSRFToken    string
	ExpiresAt    time.Time
}

// NewUser is the already validated, hashed persistence input.
type NewUser struct {
	Username     string
	Email        string
	PasswordHash string
	Role         string
}

func normalizeIdentifier(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validateUsername(value string) (string, error) {
	value = normalizeIdentifier(value)
	if !usernamePattern.MatchString(value) {
		return "", &ValidationError{Field: "username", Message: "use 3-32 lowercase letters, numbers, dots, underscores, or hyphens; start with a letter or number"}
	}
	return value, nil
}

func validateEmail(value string) (string, error) {
	value = normalizeIdentifier(value)
	if len(value) > 254 || strings.ContainsAny(value, "\r\n") {
		return "", &ValidationError{Field: "email", Message: "enter a valid email address"}
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value || !strings.Contains(value, "@") {
		return "", &ValidationError{Field: "email", Message: "enter a valid email address"}
	}
	return value, nil
}

func validatePassword(value string) error {
	if !utf8.ValidString(value) {
		return &ValidationError{Field: "password", Message: "must be valid UTF-8 text"}
	}
	if utf8.RuneCountInString(value) < 12 {
		return &ValidationError{Field: "password", Message: "use at least 12 characters"}
	}
	if len([]byte(value)) > 128 {
		return &ValidationError{Field: "password", Message: "use at most 128 UTF-8 bytes"}
	}
	return nil
}
