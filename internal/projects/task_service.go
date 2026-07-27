package projects

import (
	"context"
	"errors"
	"fmt"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

func (s *Service) ListTasks(ctx context.Context, actor identity.User, projectID string, audit AuditContext) ([]Task, error) {
	if err := requireApplicationAccess(actor); err != nil {
		return nil, err
	}
	tasks, err := s.repository.ListTasks(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID)
	if errors.Is(err, ErrNotFound) {
		return nil, s.auditTaskDenied(ctx, actor, projectID, "", audit, "project_not_accessible")
	}
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	for index := range tasks {
		if err := s.renderTask(&tasks[index]); err != nil {
			return nil, err
		}
	}
	return tasks, nil
}

func (s *Service) GetTask(ctx context.Context, actor identity.User, projectID, taskID string, audit AuditContext) (Task, error) {
	if err := requireApplicationAccess(actor); err != nil {
		return Task{}, err
	}
	task, err := s.repository.GetTask(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID)
	if errors.Is(err, ErrNotFound) {
		return Task{}, s.auditTaskDenied(ctx, actor, projectID, taskID, audit, "task_not_accessible")
	}
	if err != nil {
		return Task{}, fmt.Errorf("get task: %w", err)
	}
	if err := s.renderTask(&task); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Service) CreateTask(ctx context.Context, actor identity.User, projectID string, input CreateTaskInput, audit AuditContext) (Task, error) {
	if err := requireApplicationAccess(actor); err != nil {
		return Task{}, err
	}
	persistence, err := validateTask(input.Name, input.GoalsMarkdown, input.DescriptionMarkdown, input.ResponsibleUserID, input.TargetDate)
	if err != nil {
		return Task{}, err
	}
	goalsHTML, descriptionHTML, err := s.renderTaskMarkdown(persistence.GoalsMarkdown, persistence.DescriptionMarkdown)
	if err != nil {
		return Task{}, err
	}
	task, err := s.repository.CreateTask(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, persistence, auditEvent{ActorUserID: actor.ID, Action: "task.created", TargetType: "task", Outcome: "succeeded", Context: audit, Details: map[string]any{"project_id": projectID}})
	if errors.Is(err, ErrNotFound) {
		return Task{}, s.auditTaskDenied(ctx, actor, projectID, "", audit, "project_not_accessible")
	}
	if errors.Is(err, ErrInactiveProject) || errors.Is(err, ErrInvalidResponsible) {
		return Task{}, err
	}
	if err != nil {
		return Task{}, fmt.Errorf("create task: %w", err)
	}
	task.GoalsHTML, task.DescriptionHTML = goalsHTML, descriptionHTML
	return task, nil
}

func (s *Service) UpdateTask(ctx context.Context, actor identity.User, projectID, taskID string, input UpdateTaskInput, audit AuditContext) (Task, error) {
	if err := requireApplicationAccess(actor); err != nil {
		return Task{}, err
	}
	if !input.ResponsibleUserID.Present {
		return Task{}, &ValidationError{Field: "responsible_user_id", Message: "is required and may be null"}
	}
	if !input.TargetDate.Present {
		return Task{}, &ValidationError{Field: "target_date", Message: "is required and may be null"}
	}
	persistence, err := validateTask(input.Name, input.GoalsMarkdown, input.DescriptionMarkdown, input.ResponsibleUserID.Value, input.TargetDate.Value)
	if err != nil {
		return Task{}, err
	}
	if input.ExpectedVersion < 1 {
		return Task{}, &ValidationError{Field: "expected_version", Message: "must be a positive integer"}
	}
	persistence.ExpectedVersion = input.ExpectedVersion
	goalsHTML, descriptionHTML, err := s.renderTaskMarkdown(persistence.GoalsMarkdown, persistence.DescriptionMarkdown)
	if err != nil {
		return Task{}, err
	}
	task, err := s.repository.UpdateTask(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID, persistence, auditEvent{ActorUserID: actor.ID, Action: "task.updated", TargetType: "task", TargetID: taskID, Outcome: "succeeded", Context: audit, Details: map[string]any{"project_id": projectID}})
	if errors.Is(err, ErrNotFound) {
		return Task{}, s.auditTaskDenied(ctx, actor, projectID, taskID, audit, "task_not_accessible_or_not_owner")
	}
	if errors.Is(err, ErrConflict) || errors.Is(err, ErrInactiveProject) || errors.Is(err, ErrInvalidResponsible) {
		return Task{}, err
	}
	if err != nil {
		return Task{}, fmt.Errorf("update task: %w", err)
	}
	task.GoalsHTML, task.DescriptionHTML = goalsHTML, descriptionHTML
	return task, nil
}

func (s *Service) renderTask(task *Task) error {
	goals, description, err := s.renderTaskMarkdown(task.GoalsMarkdown, task.DescriptionMarkdown)
	if err != nil {
		return err
	}
	task.GoalsHTML, task.DescriptionHTML = goals, description
	return nil
}

func (s *Service) renderTaskMarkdown(goals, description string) (string, string, error) {
	goalsHTML, err := s.renderer.Render(goals)
	if err != nil {
		return "", "", fmt.Errorf("render task goals: %w", err)
	}
	descriptionHTML, err := s.renderer.Render(description)
	if err != nil {
		return "", "", fmt.Errorf("render task description: %w", err)
	}
	return goalsHTML, descriptionHTML, nil
}

func (s *Service) auditTaskDenied(ctx context.Context, actor identity.User, projectID, taskID string, audit AuditContext, reason string) error {
	targetID := taskID
	targetType := "task"
	if targetID == "" {
		targetID = projectID
		targetType = "project"
	}
	if err := s.repository.AppendAudit(ctx, auditEvent{ActorUserID: actor.ID, Action: "authorization.task_denied", TargetType: targetType, TargetID: targetID, Outcome: "denied", Context: audit, Details: map[string]any{"project_id": projectID, "reason": reason}}); err != nil {
		return fmt.Errorf("audit denied task operation: %w", err)
	}
	return ErrNotFound
}
