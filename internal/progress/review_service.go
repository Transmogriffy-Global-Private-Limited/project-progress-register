package progress

import (
	"context"
	"errors"
	"fmt"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

func (s *Service) ListComments(ctx context.Context, actor identity.User, projectID, taskID, updateID string, audit AuditContext) ([]Comment, error) {
	if err := requireAccess(actor); err != nil {
		return nil, err
	}
	comments, err := s.repository.ListComments(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID, updateID)
	if errors.Is(err, ErrNotFound) {
		return nil, s.denied(ctx, actor, projectID, taskID, updateID, "comment_scope_not_accessible", audit)
	}
	if err != nil {
		return nil, fmt.Errorf("list progress comments: %w", err)
	}
	for index := range comments {
		if err := s.renderComment(&comments[index]); err != nil {
			return nil, err
		}
	}
	return comments, nil
}

func (s *Service) CreateComment(ctx context.Context, actor identity.User, projectID, taskID, updateID string, input CreateCommentInput, audit AuditContext) (Comment, error) {
	if err := requireAccess(actor); err != nil {
		return Comment{}, err
	}
	content, err := validateReviewMarkdown("content_markdown", input.ContentMarkdown)
	if err != nil {
		return Comment{}, err
	}
	comment, err := s.repository.CreateComment(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID, updateID, content, auditEvent{ActorUserID: actor.ID, Action: "comment.created", TargetType: "update_comment", Outcome: "succeeded", Context: audit, Details: map[string]any{"project_id": projectID, "task_id": taskID, "progress_update_id": updateID}})
	if errors.Is(err, ErrNotFound) {
		return Comment{}, s.denied(ctx, actor, projectID, taskID, updateID, "comment_scope_not_accessible", audit)
	}
	if err != nil {
		return Comment{}, err
	}
	if err := s.renderComment(&comment); err != nil {
		return Comment{}, err
	}
	return comment, nil
}

func (s *Service) AcceptSuggestion(ctx context.Context, actor identity.User, projectID, taskID, updateID, commentID string, audit AuditContext) (AcceptedSuggestion, bool, error) {
	if err := requireAccess(actor); err != nil {
		return AcceptedSuggestion{}, false, err
	}
	if actor.Role != identity.RoleAdmin {
		if err := s.repository.AppendAudit(ctx, auditEvent{ActorUserID: actor.ID, Action: "authorization.suggestion_denied", TargetType: "update_comment", TargetID: commentID, Outcome: "denied", Context: audit, Details: map[string]any{"project_id": projectID, "task_id": taskID, "progress_update_id": updateID, "reason": "admin_required"}}); err != nil {
			return AcceptedSuggestion{}, false, fmt.Errorf("audit denied suggestion acceptance: %w", err)
		}
		return AcceptedSuggestion{}, false, ErrForbidden
	}
	suggestion, created, err := s.repository.AcceptSuggestion(ctx, actor.ID, true, projectID, taskID, updateID, commentID, auditEvent{ActorUserID: actor.ID, Action: "suggestion.accepted", TargetType: "accepted_suggestion", Outcome: "succeeded", Context: audit, Details: map[string]any{"project_id": projectID, "task_id": taskID, "progress_update_id": updateID, "comment_id": commentID}})
	if errors.Is(err, ErrNotFound) {
		return AcceptedSuggestion{}, false, s.deniedSuggestion(ctx, actor, projectID, taskID, updateID, commentID, audit)
	}
	if err != nil {
		return AcceptedSuggestion{}, false, err
	}
	if err := s.renderSuggestion(&suggestion); err != nil {
		return AcceptedSuggestion{}, false, err
	}
	return suggestion, created, nil
}

func (s *Service) ListAcceptedSuggestions(ctx context.Context, actor identity.User, projectID, taskID string, audit AuditContext) ([]AcceptedSuggestion, error) {
	if err := requireAccess(actor); err != nil {
		return nil, err
	}
	items, err := s.repository.ListAcceptedSuggestions(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID)
	if errors.Is(err, ErrNotFound) {
		return nil, s.denied(ctx, actor, projectID, taskID, "", "task_suggestions_not_accessible", audit)
	}
	if err != nil {
		return nil, err
	}
	for index := range items {
		if err := s.renderSuggestion(&items[index]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Service) renderComment(comment *Comment) error {
	html, err := s.renderer.Render(comment.ContentMarkdown)
	if err != nil {
		return fmt.Errorf("render comment Markdown: %w", err)
	}
	comment.ContentHTML = html
	return nil
}

func (s *Service) renderSuggestion(suggestion *AcceptedSuggestion) error {
	html, err := s.renderer.Render(suggestion.ContentMarkdown)
	if err != nil {
		return fmt.Errorf("render suggestion Markdown: %w", err)
	}
	suggestion.ContentHTML = html
	return nil
}

func (s *Service) deniedSuggestion(ctx context.Context, actor identity.User, projectID, taskID, updateID, commentID string, audit AuditContext) error {
	if err := s.repository.AppendAudit(ctx, auditEvent{ActorUserID: actor.ID, Action: "authorization.suggestion_denied", TargetType: "update_comment", TargetID: commentID, Outcome: "denied", Context: audit, Details: map[string]any{"project_id": projectID, "task_id": taskID, "progress_update_id": updateID, "reason": "comment_not_accessible"}}); err != nil {
		return fmt.Errorf("audit denied suggestion acceptance: %w", err)
	}
	return ErrNotFound
}
