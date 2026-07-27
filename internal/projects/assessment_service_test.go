package projects

import (
	"context"
	"errors"
	"testing"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

func TestSetAssessmentAppendsRenderedVersion(t *testing.T) {
	service, _ := NewService(&fakeRepository{}, fakeRenderer{})
	assessment, err := service.SetAssessment(context.Background(), identity.User{ID: "admin", Role: identity.RoleAdmin, Enabled: true}, "project", "task", SetAssessmentInput{Verdict: "needs_attention", RemarkMarkdown: "**Drainage**", ExpectedVersion: 2}, testAudit())
	if err != nil {
		t.Fatal(err)
	}
	if assessment.Version != 3 || assessment.RemarkHTML != "<p>**Drainage**</p>" {
		t.Fatalf("assessment=%#v", assessment)
	}
}

func TestAssessmentHistoryRequiresAdmin(t *testing.T) {
	service, _ := NewService(&fakeRepository{}, fakeRenderer{})
	_, err := service.ListAssessments(context.Background(), identity.User{ID: "member", Role: identity.RoleMember, Enabled: true}, "project", "task", testAudit())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error=%v", err)
	}
}
