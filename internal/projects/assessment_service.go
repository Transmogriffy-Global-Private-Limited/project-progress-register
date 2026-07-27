package projects

import (
	"context"
	"errors"
	"fmt"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

func (s *Service) GetCurrentAssessment(ctx context.Context, actor identity.User, projectID, taskID string, audit AuditContext) (*Assessment, error) {
	if err := requireApplicationAccess(actor); err != nil {
		return nil, err
	}
	assessment, err := s.repository.GetCurrentAssessment(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID)
	if errors.Is(err, ErrNotFound) {
		return nil, s.auditTaskDenied(ctx, actor, projectID, taskID, audit, "assessment_not_accessible")
	}
	if err != nil {
		return nil, fmt.Errorf("get current assessment: %w", err)
	}
	if assessment != nil {
		if err := s.renderAssessment(assessment); err != nil {
			return nil, err
		}
	}
	return assessment, nil
}

func (s *Service) ListAssessments(ctx context.Context, actor identity.User, projectID, taskID string, audit AuditContext) ([]Assessment, error) {
	if err := s.requireAssessmentAdmin(ctx, actor, projectID, taskID, audit); err != nil {
		return nil, err
	}
	items, err := s.repository.ListAssessments(ctx, actor.ID, true, projectID, taskID)
	if errors.Is(err, ErrNotFound) {
		return nil, s.auditTaskDenied(ctx, actor, projectID, taskID, audit, "assessment_history_not_accessible")
	}
	if err != nil {
		return nil, err
	}
	for index := range items {
		if err := s.renderAssessment(&items[index]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Service) SetAssessment(ctx context.Context, actor identity.User, projectID, taskID string, input SetAssessmentInput, audit AuditContext) (Assessment, error) {
	if err := s.requireAssessmentAdmin(ctx, actor, projectID, taskID, audit); err != nil {
		return Assessment{}, err
	}
	persistence, err := validateAssessment(input)
	if err != nil {
		return Assessment{}, err
	}
	assessment, err := s.repository.CreateAssessment(ctx, actor.ID, true, projectID, taskID, persistence, auditEvent{ActorUserID: actor.ID, Action: "assessment.created", TargetType: "task_assessment", Outcome: "succeeded", Context: audit, Details: map[string]any{"project_id": projectID, "task_id": taskID, "verdict": persistence.Verdict, "from_version": persistence.ExpectedVersion, "to_version": persistence.ExpectedVersion + 1}})
	if errors.Is(err, ErrNotFound) {
		return Assessment{}, s.auditTaskDenied(ctx, actor, projectID, taskID, audit, "assessment_task_not_accessible")
	}
	if errors.Is(err, ErrConflict) || errors.Is(err, ErrInactiveProject) {
		return Assessment{}, err
	}
	if err != nil {
		return Assessment{}, err
	}
	if err := s.renderAssessment(&assessment); err != nil {
		return Assessment{}, err
	}
	return assessment, nil
}

func (s *Service) renderAssessment(assessment *Assessment) error {
	html, err := s.renderer.Render(assessment.RemarkMarkdown)
	if err != nil {
		return fmt.Errorf("render assessment Markdown: %w", err)
	}
	assessment.RemarkHTML = html
	return nil
}

func (s *Service) requireAssessmentAdmin(ctx context.Context, actor identity.User, projectID, taskID string, audit AuditContext) error {
	if err := requireApplicationAccess(actor); err != nil {
		return err
	}
	if actor.Role == identity.RoleAdmin {
		return nil
	}
	if err := s.repository.AppendAudit(ctx, auditEvent{ActorUserID: actor.ID, Action: "authorization.assessment_denied", TargetType: "task", TargetID: taskID, Outcome: "denied", Context: audit, Details: map[string]any{"project_id": projectID, "reason": "admin_required"}}); err != nil {
		return fmt.Errorf("audit denied assessment operation: %w", err)
	}
	return ErrForbidden
}
