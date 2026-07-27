package projects

import (
	"strings"
	"time"
	"unicode/utf8"
)

type Assessment struct {
	ID             string    `json:"id"`
	TaskID         string    `json:"task_id"`
	Version        int64     `json:"version"`
	Verdict        string    `json:"verdict"`
	RemarkMarkdown string    `json:"remark_markdown"`
	RemarkHTML     string    `json:"remark_html"`
	AssessedBy     TaskActor `json:"assessed_by"`
	CreatedAt      time.Time `json:"created_at"`
}

type SetAssessmentInput struct {
	Verdict         string `json:"verdict"`
	RemarkMarkdown  string `json:"remark_markdown"`
	ExpectedVersion int64  `json:"expected_version"`
}

type assessmentPersistenceInput struct {
	Verdict         string
	RemarkMarkdown  string
	ExpectedVersion int64
}

func validateAssessment(input SetAssessmentInput) (assessmentPersistenceInput, error) {
	verdict := strings.TrimSpace(input.Verdict)
	switch verdict {
	case "on_track", "needs_attention", "blocked", "complete":
	default:
		return assessmentPersistenceInput{}, &ValidationError{Field: "verdict", Message: "must be on_track, needs_attention, blocked, or complete"}
	}
	if strings.TrimSpace(input.RemarkMarkdown) == "" || utf8.RuneCountInString(input.RemarkMarkdown) > 10000 || len(input.RemarkMarkdown) > 20000 {
		return assessmentPersistenceInput{}, &ValidationError{Field: "remark_markdown", Message: "must contain 1-10000 characters and at most 20000 UTF-8 bytes"}
	}
	if input.ExpectedVersion < 0 {
		return assessmentPersistenceInput{}, &ValidationError{Field: "expected_version", Message: "must be zero or a positive integer"}
	}
	return assessmentPersistenceInput{Verdict: verdict, RemarkMarkdown: input.RemarkMarkdown, ExpectedVersion: input.ExpectedVersion}, nil
}
