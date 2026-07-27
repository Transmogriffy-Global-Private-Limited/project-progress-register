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
	events, err := service.GetTaskTimeline(context.Background(), identity.User{ID: "member", Role: identity.RoleMember, Enabled: true}, "project", "task", AuditContext{})
	if err != nil {
		t.Fatal(err)
	}
	if events[0].Metadata["content_html"] != "<p>**Review**</p>" {
		t.Fatalf("timeline metadata = %#v", events[0].Metadata)
	}
}
