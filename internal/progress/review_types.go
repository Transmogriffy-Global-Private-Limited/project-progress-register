package progress

import (
	"strings"
	"time"
	"unicode/utf8"
)

type SuggestionAcceptance struct {
	ID         string    `json:"id"`
	AcceptedBy Actor     `json:"accepted_by"`
	AcceptedAt time.Time `json:"accepted_at"`
}

type Comment struct {
	ID                 string                `json:"id"`
	ProgressUpdateID   string                `json:"progress_update_id"`
	ContentMarkdown    string                `json:"content_markdown"`
	ContentHTML        string                `json:"content_html"`
	CreatedBy          Actor                 `json:"created_by"`
	CreatedAt          time.Time             `json:"created_at"`
	AcceptedSuggestion *SuggestionAcceptance `json:"accepted_suggestion"`
}

type AcceptedSuggestion struct {
	ID               string    `json:"id"`
	CommentID        string    `json:"comment_id"`
	ProgressUpdateID string    `json:"progress_update_id"`
	TaskID           string    `json:"task_id"`
	ContentMarkdown  string    `json:"content_markdown"`
	ContentHTML      string    `json:"content_html"`
	CommentAuthor    Actor     `json:"comment_author"`
	CommentedAt      time.Time `json:"commented_at"`
	AcceptedBy       Actor     `json:"accepted_by"`
	AcceptedAt       time.Time `json:"accepted_at"`
}

type CreateCommentInput struct {
	ContentMarkdown string `json:"content_markdown"`
}

func validateReviewMarkdown(field, value string) (string, error) {
	if strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > 10000 || len(value) > 20000 {
		return "", &ValidationError{Field: field, Message: "must contain 1-10000 characters and at most 20000 UTF-8 bytes"}
	}
	return value, nil
}
