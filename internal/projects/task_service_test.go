package projects

import (
	"context"
	"errors"
	"testing"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

func TestTaskValidationAndDerivedHTML(t *testing.T) {
	t.Parallel()
	repository := &fakeRepository{}
	service, _ := NewService(repository, fakeRenderer{})
	actor := identity.User{ID: "admin-id", Username: "admin", Role: identity.RoleAdmin, Enabled: true}
	task, err := service.CreateTask(context.Background(), actor, "project-id", CreateTaskInput{Name: " Task ", GoalsMarkdown: "**Goal**", DescriptionMarkdown: "Description", TargetDate: stringPointer("2026-08-01")}, testAudit())
	if err != nil || task.Name != "Task" || task.GoalsHTML == "" || task.DescriptionHTML == "" {
		t.Fatalf("CreateTask task=%#v error=%v", task, err)
	}
	_, err = service.CreateTask(context.Background(), actor, "project-id", CreateTaskInput{Name: "Task", TargetDate: stringPointer("2026-02-30")}, testAudit())
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("invalid target date error=%v", err)
	}
}

func TestTaskUpdateRequiresPositiveVersion(t *testing.T) {
	t.Parallel()
	service, _ := NewService(&fakeRepository{}, fakeRenderer{})
	actor := identity.User{ID: "member-id", Role: identity.RoleMember, Enabled: true}
	_, err := service.UpdateTask(context.Background(), actor, "project-id", "task-id", UpdateTaskInput{Name: "Task", ResponsibleUserID: NullableString{Present: true}, TargetDate: NullableString{Present: true}}, testAudit())
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Field != "expected_version" {
		t.Fatalf("UpdateTask error=%v", err)
	}
}

func TestTaskV2ValidatesAndForwardsMultipleResponsibilities(t *testing.T) {
	t.Parallel()
	service, _ := NewService(&fakeRepository{}, fakeRenderer{})
	actor := identity.User{ID: "admin-id", Role: identity.RoleAdmin, Enabled: true}
	first := "22222222-2222-4222-8222-222222222222"
	second := "33333333-3333-4333-8333-333333333333"

	task, err := service.CreateTaskV2(context.Background(), actor, "project-id", CreateTaskV2Input{Name: "Task", ResponsibleUserIDs: []string{second, first}}, testAudit())
	if err != nil || len(task.ResponsibleMembers) != 2 || task.ResponsibleMembers[0].UserID != first || task.ResponsibleMembers[1].UserID != second {
		t.Fatalf("CreateTaskV2 task=%#v error=%v", task, err)
	}

	_, err = service.CreateTaskV2(context.Background(), actor, "project-id", CreateTaskV2Input{Name: "Task", ResponsibleUserIDs: []string{first, first}}, testAudit())
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Field != "responsible_user_ids" {
		t.Fatalf("duplicate responsibilities error=%v", err)
	}
}

func stringPointer(value string) *string { return &value }
