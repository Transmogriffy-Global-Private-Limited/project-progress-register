package projects

import (
	"context"
	"fmt"
	"time"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

type AssessmentCounts struct {
	OnTrack        int64 `json:"on_track"`
	NeedsAttention int64 `json:"needs_attention"`
	Blocked        int64 `json:"blocked"`
	Complete       int64 `json:"complete"`
}

type DashboardProject struct {
	ID                      string           `json:"id"`
	Name                    string           `json:"name"`
	Active                  bool             `json:"active"`
	TaskCount               int64            `json:"task_count"`
	ProgressUpdateCount     int64            `json:"progress_update_count"`
	AcceptedSuggestionCount int64            `json:"accepted_suggestion_count"`
	LatestProgressAt        *time.Time       `json:"latest_progress_at"`
	CurrentAssessments      AssessmentCounts `json:"current_assessments"`
}

type DashboardTotals struct {
	ProjectCount            int64            `json:"project_count"`
	ActiveProjectCount      int64            `json:"active_project_count"`
	InactiveProjectCount    int64            `json:"inactive_project_count"`
	TaskCount               int64            `json:"task_count"`
	ProgressUpdateCount     int64            `json:"progress_update_count"`
	AcceptedSuggestionCount int64            `json:"accepted_suggestion_count"`
	CurrentAssessments      AssessmentCounts `json:"current_assessments"`
}

type Dashboard struct {
	Totals   DashboardTotals    `json:"totals"`
	Projects []DashboardProject `json:"projects"`
}

func (s *Service) GetDashboard(ctx context.Context, actor identity.User, _ AuditContext) (Dashboard, error) {
	if err := requireApplicationAccess(actor); err != nil {
		return Dashboard{}, err
	}
	items, err := s.repository.GetDashboard(ctx, actor.ID, actor.Role == identity.RoleAdmin)
	if err != nil {
		return Dashboard{}, fmt.Errorf("get dashboard: %w", err)
	}
	result := Dashboard{Projects: items}
	for _, item := range items {
		result.Totals.ProjectCount++
		if item.Active {
			result.Totals.ActiveProjectCount++
		} else {
			result.Totals.InactiveProjectCount++
		}
		result.Totals.TaskCount += item.TaskCount
		result.Totals.ProgressUpdateCount += item.ProgressUpdateCount
		result.Totals.AcceptedSuggestionCount += item.AcceptedSuggestionCount
		result.Totals.CurrentAssessments.OnTrack += item.CurrentAssessments.OnTrack
		result.Totals.CurrentAssessments.NeedsAttention += item.CurrentAssessments.NeedsAttention
		result.Totals.CurrentAssessments.Blocked += item.CurrentAssessments.Blocked
		result.Totals.CurrentAssessments.Complete += item.CurrentAssessments.Complete
	}
	return result, nil
}
