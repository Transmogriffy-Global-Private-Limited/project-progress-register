package projects

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

type Repository interface {
	ListProjects(context.Context, string, bool) ([]Project, error)
	GetProject(context.Context, string, bool, string) (Project, error)
	CreateProject(context.Context, CreateProjectInput, auditEvent) (Project, error)
	UpdateProject(context.Context, string, UpdateProjectInput, auditEvent) (Project, error)
	ListMembers(context.Context, string) ([]Member, error)
	AddMember(context.Context, string, string, auditEvent) (Member, error)
	RemoveMember(context.Context, string, string, auditEvent) error
	ReplaceGeofence(context.Context, string, ReplaceGeofenceInput, auditEvent) (Geofence, error)
	ListTasks(context.Context, string, bool, string) ([]Task, error)
	GetTask(context.Context, string, bool, string, string) (Task, error)
	CreateTask(context.Context, string, bool, string, taskPersistenceInput, auditEvent) (Task, error)
	UpdateTask(context.Context, string, bool, string, string, taskPersistenceInput, auditEvent) (Task, error)
	GetCurrentAssessment(context.Context, string, bool, string, string) (*Assessment, error)
	ListAssessments(context.Context, string, bool, string, string) ([]Assessment, error)
	CreateAssessment(context.Context, string, bool, string, string, assessmentPersistenceInput, auditEvent) (Assessment, error)
	GetDashboard(context.Context, string, bool) ([]DashboardProject, error)
	GetTaskTimeline(context.Context, string, bool, string, string, timelinePersistenceQuery) ([]TimelineEvent, error)
	AppendAudit(context.Context, auditEvent) error
}

type MarkdownRenderer interface {
	Render(string) (string, error)
}

type Service struct {
	repository Repository
	renderer   MarkdownRenderer
}

func NewService(repository Repository, renderer MarkdownRenderer) (*Service, error) {
	if repository == nil || renderer == nil {
		return nil, errors.New("project repository and Markdown renderer are required")
	}
	return &Service{repository: repository, renderer: renderer}, nil
}

func (s *Service) ListProjects(ctx context.Context, actor identity.User, audit AuditContext) ([]Project, error) {
	if err := requireApplicationAccess(actor); err != nil {
		return nil, err
	}
	projects, err := s.repository.ListProjects(ctx, actor.ID, actor.Role == identity.RoleAdmin)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	for index := range projects {
		if err := s.renderProject(&projects[index]); err != nil {
			return nil, err
		}
	}
	return projects, nil
}

func (s *Service) GetProject(ctx context.Context, actor identity.User, projectID string, audit AuditContext) (Project, error) {
	if err := requireApplicationAccess(actor); err != nil {
		return Project{}, err
	}
	project, err := s.repository.GetProject(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID)
	if errors.Is(err, ErrNotFound) {
		if auditErr := s.repository.AppendAudit(ctx, auditEvent{ActorUserID: actor.ID, Action: "authorization.project_denied", TargetType: "project", TargetID: projectID, Outcome: "denied", Context: audit, Details: map[string]any{"reason": "not_accessible"}}); auditErr != nil {
			return Project{}, fmt.Errorf("audit denied project read: %w", auditErr)
		}
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("get project: %w", err)
	}
	if err := s.renderProject(&project); err != nil {
		return Project{}, err
	}
	return project, nil
}

func (s *Service) CreateProject(ctx context.Context, actor identity.User, input CreateProjectInput, audit AuditContext) (Project, error) {
	if err := s.requireAdmin(ctx, actor, "project", "", audit); err != nil {
		return Project{}, err
	}
	name, description, err := validateProject(input.Name, input.DescriptionMarkdown)
	if err != nil {
		return Project{}, err
	}
	descriptionHTML, err := s.renderer.Render(description)
	if err != nil {
		return Project{}, fmt.Errorf("render project description: %w", err)
	}
	project, err := s.repository.CreateProject(ctx, CreateProjectInput{Name: name, DescriptionMarkdown: description}, auditEvent{ActorUserID: actor.ID, Action: "project.created", TargetType: "project", Outcome: "succeeded", Context: audit})
	if err != nil {
		return Project{}, fmt.Errorf("create project: %w", err)
	}
	project.DescriptionHTML = descriptionHTML
	return project, nil
}

func (s *Service) UpdateProject(ctx context.Context, actor identity.User, projectID string, input UpdateProjectInput, audit AuditContext) (Project, error) {
	if err := s.requireAdmin(ctx, actor, "project", projectID, audit); err != nil {
		return Project{}, err
	}
	name, description, err := validateProject(input.Name, input.DescriptionMarkdown)
	if err != nil {
		return Project{}, err
	}
	if input.Active == nil {
		return Project{}, &ValidationError{Field: "active", Message: "is required"}
	}
	if input.ExpectedVersion < 1 {
		return Project{}, &ValidationError{Field: "expected_version", Message: "must be a positive integer"}
	}
	descriptionHTML, err := s.renderer.Render(description)
	if err != nil {
		return Project{}, fmt.Errorf("render project description: %w", err)
	}
	project, err := s.repository.UpdateProject(ctx, projectID, UpdateProjectInput{Name: name, DescriptionMarkdown: description, Active: input.Active, ExpectedVersion: input.ExpectedVersion}, auditEvent{ActorUserID: actor.ID, Action: "project.updated", TargetType: "project", TargetID: projectID, Outcome: "succeeded", Context: audit, Details: map[string]any{"active": *input.Active}})
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) {
		return Project{}, err
	}
	if err != nil {
		return Project{}, fmt.Errorf("update project: %w", err)
	}
	project.DescriptionHTML = descriptionHTML
	return project, nil
}

func (s *Service) ListMembers(ctx context.Context, actor identity.User, projectID string, audit AuditContext) ([]Member, error) {
	if err := s.requireAdmin(ctx, actor, "project", projectID, audit); err != nil {
		return nil, err
	}
	members, err := s.repository.ListMembers(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return members, nil
}

func (s *Service) AddMember(ctx context.Context, actor identity.User, projectID, userID string, audit AuditContext) (Member, error) {
	if err := s.requireAdmin(ctx, actor, "project", projectID, audit); err != nil {
		return Member{}, err
	}
	member, err := s.repository.AddMember(ctx, projectID, userID, auditEvent{ActorUserID: actor.ID, Action: "project.membership_added", TargetType: "project", TargetID: projectID, Outcome: "succeeded", Context: audit, Details: map[string]any{"user_id": userID}})
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) || errors.Is(err, ErrInvalidMember) {
		return Member{}, err
	}
	if err != nil {
		return Member{}, fmt.Errorf("add project member: %w", err)
	}
	return member, nil
}

func (s *Service) RemoveMember(ctx context.Context, actor identity.User, projectID, userID string, audit AuditContext) error {
	if err := s.requireAdmin(ctx, actor, "project", projectID, audit); err != nil {
		return err
	}
	err := s.repository.RemoveMember(ctx, projectID, userID, auditEvent{ActorUserID: actor.ID, Action: "project.membership_removed", TargetType: "project", TargetID: projectID, Outcome: "succeeded", Context: audit, Details: map[string]any{"user_id": userID}})
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("remove project member: %w", err)
	}
	return nil
}

func (s *Service) ReplaceGeofence(ctx context.Context, actor identity.User, projectID string, input ReplaceGeofenceInput, audit AuditContext) (Geofence, error) {
	if err := s.requireAdmin(ctx, actor, "project", projectID, audit); err != nil {
		return Geofence{}, err
	}
	if err := validateGeofence(input); err != nil {
		return Geofence{}, err
	}
	geofence, err := s.repository.ReplaceGeofence(ctx, projectID, input, auditEvent{ActorUserID: actor.ID, Action: "project.geofence_updated", TargetType: "project", TargetID: projectID, Outcome: "succeeded", Context: audit, Details: map[string]any{"version": input.ExpectedVersion + 1}})
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) {
		return Geofence{}, err
	}
	if err != nil {
		return Geofence{}, fmt.Errorf("replace project geofence: %w", err)
	}
	return geofence, nil
}

func requireApplicationAccess(actor identity.User) error {
	if !actor.Enabled || strings.TrimSpace(actor.ID) == "" {
		return identity.ErrUnauthenticated
	}
	if actor.MustChangePassword {
		return ErrPasswordChangeRequired
	}
	return nil
}

func (s *Service) requireAdmin(ctx context.Context, actor identity.User, targetType, targetID string, audit AuditContext) error {
	if requireApplicationAccess(actor) == nil && actor.Role == identity.RoleAdmin {
		return nil
	}
	event := auditEvent{ActorUserID: actor.ID, Action: "authorization.project_admin_denied", TargetType: targetType, TargetID: targetID, Outcome: "denied", Context: audit, Details: map[string]any{"reason": "admin_required"}}
	if err := s.repository.AppendAudit(ctx, event); err != nil {
		return fmt.Errorf("audit denied project administration: %w", err)
	}
	if actor.MustChangePassword {
		return ErrPasswordChangeRequired
	}
	if !actor.Enabled || actor.ID == "" {
		return identity.ErrUnauthenticated
	}
	return ErrForbidden
}

func (s *Service) renderProject(project *Project) error {
	rendered, err := s.renderer.Render(project.DescriptionMarkdown)
	if err != nil {
		return fmt.Errorf("render project description: %w", err)
	}
	project.DescriptionHTML = rendered
	return nil
}
