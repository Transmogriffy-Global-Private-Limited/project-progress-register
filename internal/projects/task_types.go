package projects

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var taskUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type TaskActor struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

type ResponsibleMember struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Enabled  bool   `json:"enabled"`
}

type Task struct {
	ID                  string             `json:"id"`
	ProjectID           string             `json:"project_id"`
	Name                string             `json:"name"`
	GoalsMarkdown       string             `json:"goals_markdown"`
	GoalsHTML           string             `json:"goals_html"`
	DescriptionMarkdown string             `json:"description_markdown"`
	DescriptionHTML     string             `json:"description_html"`
	CreatedBy           TaskActor          `json:"created_by"`
	ResponsibleMember   *ResponsibleMember `json:"responsible_member"`
	TargetDate          *string            `json:"target_date"`
	CreatedAt           time.Time          `json:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at"`
	Version             int64              `json:"version"`
}

type CreateTaskInput struct {
	Name                string  `json:"name"`
	GoalsMarkdown       string  `json:"goals_markdown"`
	DescriptionMarkdown string  `json:"description_markdown"`
	ResponsibleUserID   *string `json:"responsible_user_id"`
	TargetDate          *string `json:"target_date"`
}

type UpdateTaskInput struct {
	Name                string         `json:"name"`
	GoalsMarkdown       string         `json:"goals_markdown"`
	DescriptionMarkdown string         `json:"description_markdown"`
	ResponsibleUserID   NullableString `json:"responsible_user_id"`
	TargetDate          NullableString `json:"target_date"`
	ExpectedVersion     int64          `json:"expected_version"`
}

// NullableString distinguishes a required JSON null from an omitted update field.
type NullableString struct {
	Present bool
	Value   *string
}

func (value *NullableString) UnmarshalJSON(data []byte) error {
	value.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		value.Value = nil
		return nil
	}
	var decoded string
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	value.Value = &decoded
	return nil
}

type taskPersistenceInput struct {
	Name                string
	GoalsMarkdown       string
	DescriptionMarkdown string
	ResponsibleUserID   string
	TargetDate          string
	ExpectedVersion     int64
}

func validateTask(name, goals, description string, responsibleUserID, targetDate *string) (taskPersistenceInput, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 160 {
		return taskPersistenceInput{}, &ValidationError{Field: "name", Message: "must contain 1-160 characters"}
	}
	if len(goals) > 20000 {
		return taskPersistenceInput{}, &ValidationError{Field: "goals_markdown", Message: "must not exceed 20000 UTF-8 bytes"}
	}
	if len(description) > 50000 {
		return taskPersistenceInput{}, &ValidationError{Field: "description_markdown", Message: "must not exceed 50000 UTF-8 bytes"}
	}
	result := taskPersistenceInput{Name: name, GoalsMarkdown: goals, DescriptionMarkdown: description}
	if responsibleUserID != nil {
		result.ResponsibleUserID = strings.TrimSpace(*responsibleUserID)
		if !taskUUIDPattern.MatchString(result.ResponsibleUserID) {
			return taskPersistenceInput{}, &ValidationError{Field: "responsible_user_id", Message: "must be a UUID or null"}
		}
	}
	if targetDate != nil {
		result.TargetDate = strings.TrimSpace(*targetDate)
		if _, err := time.Parse(time.DateOnly, result.TargetDate); err != nil {
			return taskPersistenceInput{}, &ValidationError{Field: "target_date", Message: "must be a valid YYYY-MM-DD date or null"}
		}
	}
	return result, nil
}
