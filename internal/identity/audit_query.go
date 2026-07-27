package identity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	auditNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	auditUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
)

type AuditQuery struct {
	Limit       int
	Cursor      string
	Action      string
	Outcome     string
	ActorUserID string
	TargetType  string
}

type AuditPage struct {
	AuditEvents []AuditRecord `json:"audit_events"`
	NextCursor  string        `json:"next_cursor,omitempty"`
}

type auditCursor struct {
	OccurredAt time.Time `json:"occurred_at"`
	ID         string    `json:"id"`
}

type auditPersistenceQuery struct {
	Limit       int
	Action      string
	Outcome     string
	ActorUserID string
	TargetType  string
	Cursor      *auditCursor
}

func (s *Service) ListAudit(ctx context.Context, actor User, input AuditQuery, audit AuditContext) (AuditPage, error) {
	if err := s.requireAuditAdmin(ctx, actor, audit); err != nil {
		return AuditPage{}, err
	}
	query, err := validateAuditQuery(input)
	if err != nil {
		return AuditPage{}, err
	}
	records, err := s.repository.ListAudit(ctx, query)
	if err != nil {
		return AuditPage{}, fmt.Errorf("list audit: %w", err)
	}
	page := AuditPage{AuditEvents: records}
	if len(records) > query.Limit {
		page.AuditEvents = records[:query.Limit]
		last := page.AuditEvents[len(page.AuditEvents)-1]
		page.NextCursor, err = encodeAuditCursor(auditCursor{OccurredAt: last.OccurredAt, ID: last.ID})
		if err != nil {
			return AuditPage{}, fmt.Errorf("encode audit cursor: %w", err)
		}
	}
	return page, nil
}

func (s *Service) requireAuditAdmin(ctx context.Context, actor User, audit AuditContext) error {
	if actor.Enabled && actor.Role == RoleAdmin && !actor.MustChangePassword {
		return nil
	}
	event := AuditEvent{ActorUserID: actor.ID, Action: "authorization.audit_denied", TargetType: "audit", Outcome: "denied", Context: cleanAuditContext(audit), Details: map[string]any{"reason": "admin_required"}}
	if err := s.repository.AppendAudit(ctx, event); err != nil {
		return fmt.Errorf("audit denied audit access: %w", err)
	}
	if actor.MustChangePassword {
		return ErrPasswordChangeNeeded
	}
	return ErrForbidden
}

func validateAuditQuery(input AuditQuery) (auditPersistenceQuery, error) {
	query := auditPersistenceQuery{Limit: input.Limit, Action: strings.TrimSpace(input.Action), Outcome: strings.TrimSpace(input.Outcome), ActorUserID: strings.TrimSpace(input.ActorUserID), TargetType: strings.TrimSpace(input.TargetType)}
	if query.Limit == 0 {
		query.Limit = 100
	}
	if query.Limit < 1 || query.Limit > 200 {
		return auditPersistenceQuery{}, &ValidationError{Field: "limit", Message: "must be between 1 and 200"}
	}
	if query.Action != "" && (len(query.Action) > 100 || !auditNamePattern.MatchString(query.Action)) {
		return auditPersistenceQuery{}, &ValidationError{Field: "action", Message: "must be a valid exact audit action"}
	}
	if query.Outcome != "" && query.Outcome != "succeeded" && query.Outcome != "failed" && query.Outcome != "denied" {
		return auditPersistenceQuery{}, &ValidationError{Field: "outcome", Message: "must be succeeded, failed, or denied"}
	}
	if query.ActorUserID != "" && !auditUUIDPattern.MatchString(query.ActorUserID) {
		return auditPersistenceQuery{}, &ValidationError{Field: "actor_user_id", Message: "must be a UUID"}
	}
	if query.TargetType != "" && (len(query.TargetType) > 50 || !auditNamePattern.MatchString(query.TargetType)) {
		return auditPersistenceQuery{}, &ValidationError{Field: "target_type", Message: "must be a valid exact target type"}
	}
	if strings.TrimSpace(input.Cursor) != "" {
		cursor, err := decodeAuditCursor(input.Cursor)
		if err != nil {
			return auditPersistenceQuery{}, &ValidationError{Field: "cursor", Message: "is invalid"}
		}
		query.Cursor = &cursor
	}
	return query, nil
}

func encodeAuditCursor(cursor auditCursor) (string, error) {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeAuditCursor(value string) (auditCursor, error) {
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return auditCursor{}, err
	}
	var cursor auditCursor
	if err := json.Unmarshal(payload, &cursor); err != nil || cursor.OccurredAt.IsZero() || !auditUUIDPattern.MatchString(cursor.ID) {
		return auditCursor{}, fmt.Errorf("invalid cursor")
	}
	return cursor, nil
}
