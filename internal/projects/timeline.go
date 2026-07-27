package projects

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

type TimelineEvent struct {
	ID         string         `json:"id"`
	Action     string         `json:"action"`
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	Actor      TaskActor      `json:"actor"`
	OccurredAt time.Time      `json:"occurred_at"`
	Metadata   map[string]any `json:"metadata"`
}

type TimelineQuery struct {
	Limit  int
	Cursor string
}

type TimelinePage struct {
	Timeline   []TimelineEvent `json:"timeline"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

type timelineCursor struct {
	OccurredAt time.Time `json:"occurred_at"`
	ID         string    `json:"id"`
}

type timelinePersistenceQuery struct {
	Limit  int
	Cursor *timelineCursor
}

func (s *Service) GetTaskTimeline(ctx context.Context, actor identity.User, projectID, taskID string, input TimelineQuery, audit AuditContext) (TimelinePage, error) {
	if err := requireApplicationAccess(actor); err != nil {
		return TimelinePage{}, err
	}
	query, err := validateTimelineQuery(input)
	if err != nil {
		return TimelinePage{}, err
	}
	events, err := s.repository.GetTaskTimeline(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID, query)
	if errors.Is(err, ErrNotFound) {
		return TimelinePage{}, s.auditTaskDenied(ctx, actor, projectID, taskID, audit, "timeline_not_accessible")
	}
	if err != nil {
		return TimelinePage{}, fmt.Errorf("get task timeline: %w", err)
	}
	page := TimelinePage{Timeline: events}
	if len(events) > query.Limit {
		page.Timeline = events[:query.Limit]
		last := page.Timeline[len(page.Timeline)-1]
		page.NextCursor, err = encodeTimelineCursor(timelineCursor{OccurredAt: last.OccurredAt, ID: last.ID})
		if err != nil {
			return TimelinePage{}, fmt.Errorf("encode timeline cursor: %w", err)
		}
	}
	for i := range page.Timeline {
		if err := s.renderTimelineMetadata(page.Timeline[i].Metadata); err != nil {
			return TimelinePage{}, err
		}
	}
	return page, nil
}

func validateTimelineQuery(input TimelineQuery) (timelinePersistenceQuery, error) {
	query := timelinePersistenceQuery{Limit: input.Limit}
	if query.Limit == 0 {
		query.Limit = 100
	}
	if query.Limit < 1 || query.Limit > 200 {
		return timelinePersistenceQuery{}, &ValidationError{Field: "limit", Message: "must be between 1 and 200"}
	}
	value := strings.TrimSpace(input.Cursor)
	if value != "" {
		if len(value) > 1024 {
			return timelinePersistenceQuery{}, &ValidationError{Field: "cursor", Message: "is invalid"}
		}
		cursor, err := decodeTimelineCursor(value)
		if err != nil {
			return timelinePersistenceQuery{}, &ValidationError{Field: "cursor", Message: "is invalid"}
		}
		query.Cursor = &cursor
	}
	return query, nil
}

func encodeTimelineCursor(cursor timelineCursor) (string, error) {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeTimelineCursor(value string) (timelineCursor, error) {
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return timelineCursor{}, err
	}
	var cursor timelineCursor
	if err := json.Unmarshal(payload, &cursor); err != nil || cursor.OccurredAt.IsZero() || strings.TrimSpace(cursor.ID) == "" || len(cursor.ID) > 200 {
		return timelineCursor{}, fmt.Errorf("invalid cursor")
	}
	return cursor, nil
}

func (s *Service) renderTimelineMetadata(value map[string]any) error {
	for key, item := range value {
		switch typed := item.(type) {
		case map[string]any:
			if err := s.renderTimelineMetadata(typed); err != nil {
				return err
			}
		case string:
			if strings.HasSuffix(key, "_markdown") {
				html, err := s.renderer.Render(typed)
				if err != nil {
					return fmt.Errorf("render timeline Markdown: %w", err)
				}
				value[strings.TrimSuffix(key, "_markdown")+"_html"] = html
			}
		}
	}
	return nil
}
