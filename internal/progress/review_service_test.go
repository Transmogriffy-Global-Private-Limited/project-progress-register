package progress

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

func TestCreateCommentRendersMarkdown(t *testing.T) {
	service, _ := NewService(&fakeRepository{}, &fakeStorage{}, fakeRenderer{}, 10)
	comment, err := service.CreateComment(context.Background(), identity.User{ID: "actor", Role: identity.RoleMember, Enabled: true}, "project", "task", "update", CreateCommentInput{ContentMarkdown: "**Check**"}, testAudit())
	if err != nil {
		t.Fatal(err)
	}
	if comment.ContentHTML != "<p>**Check**</p>" {
		t.Fatalf("content_html=%q", comment.ContentHTML)
	}
}

func TestSuggestionAcceptanceRequiresAdmin(t *testing.T) {
	service, _ := NewService(&fakeRepository{}, &fakeStorage{}, fakeRenderer{}, 10)
	_, _, err := service.AcceptSuggestion(context.Background(), identity.User{ID: "actor", Role: identity.RoleMember, Enabled: true}, "project", "task", "update", "comment", testAudit())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error=%v", err)
	}
}

func TestReviewMarkdownValidation(t *testing.T) {
	if _, err := validateReviewMarkdown("content_markdown", strings.Repeat("x", 10001)); err == nil {
		t.Fatal("oversized review Markdown was accepted")
	}
}
