package projects

import (
	"context"
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

func (s *Service) GetTaskTimeline(ctx context.Context, actor identity.User, projectID, taskID string, audit AuditContext) ([]TimelineEvent, error) {
	if err := requireApplicationAccess(actor); err != nil {
		return nil, err
	}
	events, err := s.repository.GetTaskTimeline(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID)
	if errors.Is(err, ErrNotFound) {
		return nil, s.auditTaskDenied(ctx, actor, projectID, taskID, audit, "timeline_not_accessible")
	}
	if err != nil {
		return nil, fmt.Errorf("get task timeline: %w", err)
	}
	for i := range events {
		if err := s.renderTimelineMetadata(events[i].Metadata); err != nil {
			return nil, err
		}
	}
	return events, nil
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
