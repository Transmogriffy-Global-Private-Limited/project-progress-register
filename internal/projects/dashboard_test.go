package projects

import (
	"context"
	"testing"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

func TestDashboardTotalsUseAuthorizedProjectRows(t *testing.T) {
	service, err := NewService(&fakeRepository{}, fakeRenderer{})
	if err != nil {
		t.Fatal(err)
	}
	dashboard, err := service.GetDashboard(context.Background(), identity.User{ID: "actor", Role: identity.RoleMember, Enabled: true}, AuditContext{})
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.Totals.ProjectCount != 1 || dashboard.Totals.ActiveProjectCount != 1 || dashboard.Totals.TaskCount != 2 || dashboard.Totals.ProgressUpdateCount != 3 || dashboard.Totals.AcceptedSuggestionCount != 1 || dashboard.Totals.CurrentAssessments.OnTrack != 1 {
		t.Fatalf("unexpected dashboard totals: %+v", dashboard.Totals)
	}
}
