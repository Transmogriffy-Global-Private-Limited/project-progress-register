package projects

import (
	"context"
	"errors"
	"testing"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

func TestMemberProjectReadsRemainRepositoryScoped(t *testing.T) {
	t.Parallel()
	repository := &fakeRepository{}
	service, _ := NewService(repository, fakeRenderer{})
	actor := identity.User{ID: "member-id", Role: identity.RoleMember, Enabled: true}
	_, err := service.GetProject(context.Background(), actor, "hidden-project", testAudit())
	if !errors.Is(err, ErrNotFound) || !repository.deniedAudit {
		t.Fatalf("GetProject error=%v deniedAudit=%t", err, repository.deniedAudit)
	}
	if repository.lastAdminFlag {
		t.Fatal("Member query was incorrectly elevated to Admin scope")
	}
}

func TestProjectAdministrationRequiresAdmin(t *testing.T) {
	t.Parallel()
	repository := &fakeRepository{}
	service, _ := NewService(repository, fakeRenderer{})
	_, err := service.CreateProject(context.Background(), identity.User{ID: "member-id", Role: identity.RoleMember, Enabled: true}, CreateProjectInput{Name: "Site"}, testAudit())
	if !errors.Is(err, ErrForbidden) || !repository.deniedAudit {
		t.Fatalf("CreateProject error=%v deniedAudit=%t", err, repository.deniedAudit)
	}
}

func TestGeofenceValidationAndVersionForwarding(t *testing.T) {
	t.Parallel()
	repository := &fakeRepository{}
	service, _ := NewService(repository, fakeRenderer{})
	admin := identity.User{ID: "admin-id", Role: identity.RoleAdmin, Enabled: true}
	_, err := service.ReplaceGeofence(context.Background(), admin, "project-id", ReplaceGeofenceInput{Latitude: 91, RadiusMetres: 100, MaxAccuracyMetres: 20}, testAudit())
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("invalid geofence error=%v", err)
	}
	geofence, err := service.ReplaceGeofence(context.Background(), admin, "project-id", ReplaceGeofenceInput{Latitude: 22.5, Longitude: 88.3, RadiusMetres: 100, MaxAccuracyMetres: 20, ExpectedVersion: 3}, testAudit())
	if err != nil || geofence.Version != 4 {
		t.Fatalf("ReplaceGeofence geofence=%#v error=%v", geofence, err)
	}
}

type fakeRepository struct {
	deniedAudit   bool
	lastAdminFlag bool
}

func (f *fakeRepository) ListProjects(_ context.Context, _ string, admin bool) ([]Project, error) {
	f.lastAdminFlag = admin
	return nil, nil
}
func (f *fakeRepository) GetProject(_ context.Context, _ string, admin bool, _ string) (Project, error) {
	f.lastAdminFlag = admin
	return Project{}, ErrNotFound
}
func (*fakeRepository) CreateProject(_ context.Context, input CreateProjectInput, _ auditEvent) (Project, error) {
	return Project{Name: input.Name, Active: true, Version: 1}, nil
}
func (*fakeRepository) UpdateProject(_ context.Context, _ string, input UpdateProjectInput, _ auditEvent) (Project, error) {
	return Project{Name: input.Name, Active: *input.Active, Version: input.ExpectedVersion + 1}, nil
}
func (*fakeRepository) ListMembers(context.Context, string) ([]Member, error) { return nil, nil }
func (*fakeRepository) AddMember(context.Context, string, string, auditEvent) (Member, error) {
	return Member{}, nil
}
func (*fakeRepository) RemoveMember(context.Context, string, string, auditEvent) error { return nil }
func (*fakeRepository) ReplaceGeofence(_ context.Context, _ string, input ReplaceGeofenceInput, _ auditEvent) (Geofence, error) {
	return Geofence{Version: input.ExpectedVersion + 1}, nil
}
func (*fakeRepository) ListTasks(context.Context, string, bool, string) ([]Task, error) {
	return nil, nil
}
func (*fakeRepository) GetTask(context.Context, string, bool, string, string) (Task, error) {
	return Task{}, ErrNotFound
}
func (*fakeRepository) CreateTask(_ context.Context, actorID string, _ bool, projectID string, input taskPersistenceInput, _ auditEvent) (Task, error) {
	return Task{ID: "task-id", ProjectID: projectID, Name: input.Name, GoalsMarkdown: input.GoalsMarkdown, DescriptionMarkdown: input.DescriptionMarkdown, CreatedBy: TaskActor{UserID: actorID}, Version: 1}, nil
}
func (*fakeRepository) UpdateTask(_ context.Context, actorID string, _ bool, projectID, taskID string, input taskPersistenceInput, _ auditEvent) (Task, error) {
	return Task{ID: taskID, ProjectID: projectID, Name: input.Name, GoalsMarkdown: input.GoalsMarkdown, DescriptionMarkdown: input.DescriptionMarkdown, CreatedBy: TaskActor{UserID: actorID}, Version: input.ExpectedVersion + 1}, nil
}
func (f *fakeRepository) AppendAudit(_ context.Context, event auditEvent) error {
	f.deniedAudit = event.Outcome == "denied"
	return nil
}

func testAudit() AuditContext {
	return AuditContext{RequestID: "request-12345678", ClientIP: "127.0.0.1"}
}

type fakeRenderer struct{}

func (fakeRenderer) Render(source string) (string, error) { return "<p>" + source + "</p>", nil }
