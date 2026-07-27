package projects

import (
	"context"
	"testing"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

func TestTaskTimelineRendersMarkdownMetadata(t *testing.T) {
	service, err := NewService(&fakeRepository{}, fakeRenderer{})
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.GetTaskTimeline(context.Background(), identity.User{ID: "member", Role: identity.RoleMember, Enabled: true}, "project", "task", TimelineQuery{Limit: 1}, AuditContext{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Timeline) != 1 || page.Timeline[0].Metadata["content_html"] != "<p>**Review**</p>" {
		t.Fatalf("timeline page = %#v", page)
	}
	if page.NextCursor == "" {
		t.Fatal("next cursor is empty for a truncated page")
	}
	cursor, err := decodeTimelineCursor(page.NextCursor)
	if err != nil || cursor.ID != page.Timeline[0].ID || !cursor.OccurredAt.Equal(page.Timeline[0].OccurredAt) {
		t.Fatalf("decoded cursor = %#v, %v", cursor, err)
	}
}

func TestTimelineQueryValidation(t *testing.T) {
	for _, input := range []TimelineQuery{{Limit: 201}, {Cursor: "not-a-cursor"}} {
		if _, err := validateTimelineQuery(input); err == nil {
			t.Fatalf("validateTimelineQuery(%#v) succeeded", input)
		}
	}
	query, err := validateTimelineQuery(TimelineQuery{})
	if err != nil || query.Limit != 100 {
		t.Fatalf("default query = %#v, %v", query, err)
	}
}
