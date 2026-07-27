package progress

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/filestore"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
)

type Repository interface {
	GetTaskPolicy(context.Context, string, bool, string, string, bool) (TaskPolicy, error)
	ListUpdates(context.Context, string, bool, string, string) ([]Update, error)
	GetUpdate(context.Context, string, bool, string, string, string) (Update, error)
	CreateUpdate(context.Context, string, bool, string, string, progressPersistence, []attachmentPersistence, auditEvent) (Update, bool, error)
	UpdateProgress(context.Context, string, bool, string, string, string, progressPersistence, auditEvent) (Update, error)
	GetAttachment(context.Context, string, bool, string, string, string, string) (Attachment, error)
	MarkAttachmentAvailable(context.Context, string, auditEvent) (time.Time, error)
	MarkAttachmentFailed(context.Context, string, string, auditEvent) error
	ListPendingAttachments(context.Context) ([]pendingAttachment, error)
	ListComments(context.Context, string, bool, string, string, string) ([]Comment, error)
	CreateComment(context.Context, string, bool, string, string, string, string, auditEvent) (Comment, error)
	AcceptSuggestion(context.Context, string, bool, string, string, string, string, auditEvent) (AcceptedSuggestion, bool, error)
	ListAcceptedSuggestions(context.Context, string, bool, string, string) ([]AcceptedSuggestion, error)
	AppendAudit(context.Context, auditEvent) error
}

type FileStore interface {
	Stage(context.Context, io.Reader, string, string) (filestore.StagedFile, error)
	Finalize(string) error
	Open(string) (filestore.ReadSeekCloser, error)
	DiscardStaged(string) error
	CleanupOrphans(map[string]bool, time.Time) (int, error)
}

type MarkdownRenderer interface{ Render(string) (string, error) }

type Service struct {
	repository Repository
	storage    FileStore
	renderer   MarkdownRenderer
	maxFiles   int
}

func NewService(repository Repository, storage FileStore, renderer MarkdownRenderer, maxFiles int) (*Service, error) {
	if repository == nil || storage == nil || renderer == nil || maxFiles < 1 {
		return nil, errors.New("progress repository, storage, renderer, and positive file limit are required")
	}
	return &Service{repository: repository, storage: storage, renderer: renderer, maxFiles: maxFiles}, nil
}

func (s *Service) List(ctx context.Context, actor identity.User, projectID, taskID string, audit AuditContext) ([]Update, error) {
	if err := requireAccess(actor); err != nil {
		return nil, err
	}
	updates, err := s.repository.ListUpdates(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID)
	if errors.Is(err, ErrNotFound) {
		return nil, s.denied(ctx, actor, projectID, taskID, "", "task_not_accessible", audit)
	}
	if err != nil {
		return nil, fmt.Errorf("list progress updates: %w", err)
	}
	for i := range updates {
		if err := s.renderUpdate(&updates[i]); err != nil {
			return nil, err
		}
	}
	return updates, nil
}

func (s *Service) Get(ctx context.Context, actor identity.User, projectID, taskID, updateID string, audit AuditContext) (Update, error) {
	if err := requireAccess(actor); err != nil {
		return Update{}, err
	}
	update, err := s.repository.GetUpdate(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID, updateID)
	if errors.Is(err, ErrNotFound) {
		return Update{}, s.denied(ctx, actor, projectID, taskID, updateID, "update_not_accessible", audit)
	}
	if err != nil {
		return Update{}, fmt.Errorf("get progress update: %w", err)
	}
	if err := s.renderUpdate(&update); err != nil {
		return Update{}, err
	}
	return update, nil
}

func (s *Service) Create(ctx context.Context, actor identity.User, projectID, taskID, idempotencyKey string, metadata CreateMetadata, files []UploadFile, audit AuditContext) (Update, error) {
	if err := requireAccess(actor); err != nil {
		return Update{}, err
	}
	content, err := validateContent(metadata.ContentMarkdown)
	if err != nil {
		return Update{}, err
	}
	if err := validateLocation(metadata.Location, metadata.LocationUnavailableReason); err != nil {
		return Update{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		return Update{}, &ValidationError{Field: "Idempotency-Key", Message: "must contain 16-128 characters"}
	}
	if len(files) > s.maxFiles {
		return Update{}, &ValidationError{Field: "files", Message: fmt.Sprintf("must contain at most %d files", s.maxFiles)}
	}
	if metadata.Attachments == nil {
		return Update{}, &ValidationError{Field: "attachments", Message: "is required and may be an empty array"}
	}
	if len(metadata.Attachments) != len(files) {
		return Update{}, &ValidationError{Field: "attachments", Message: "must contain one descriptor for each file in order"}
	}
	if len(files) > 0 && metadata.Location == nil {
		return Update{}, &ValidationError{Field: "location", Message: "is required whenever files are uploaded"}
	}
	policy, err := s.repository.GetTaskPolicy(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID, true)
	if errors.Is(err, ErrNotFound) {
		return Update{}, s.denied(ctx, actor, projectID, taskID, "", "task_not_accessible", audit)
	}
	if err != nil {
		return Update{}, err
	}

	staged := make([]filestore.StagedFile, 0, len(files))
	persisted := make([]attachmentPersistence, 0, len(files))
	ownedByDatabase := false
	defer func() {
		if !ownedByDatabase {
			for _, file := range staged {
				_ = s.storage.DiscardStaged(file.StorageKey)
			}
		}
	}()
	for index, upload := range files {
		descriptor := metadata.Attachments[index]
		if descriptor.Source != "camera" && descriptor.Source != "upload" {
			return Update{}, &ValidationError{Field: fmt.Sprintf("attachments[%d].source", index), Message: "must be camera or upload"}
		}
		if upload.Open == nil {
			return Update{}, &ValidationError{Field: "files", Message: "contains an unreadable file"}
		}
		reader, openErr := upload.Open()
		if openErr != nil {
			return Update{}, fmt.Errorf("open attachment: %w", openErr)
		}
		file, stageErr := s.storage.Stage(ctx, reader, upload.OriginalName, upload.ReportedMIME)
		closeErr := reader.Close()
		if stageErr != nil {
			return Update{}, mapStorageError(index, stageErr)
		}
		if closeErr != nil {
			_ = s.storage.DiscardStaged(file.StorageKey)
			return Update{}, fmt.Errorf("close attachment input: %w", closeErr)
		}
		if descriptor.Source == "camera" && file.MediaKind != "image" {
			_ = s.storage.DiscardStaged(file.StorageKey)
			return Update{}, &ValidationError{Field: fmt.Sprintf("attachments[%d].source", index), Message: "camera source is allowed only for images"}
		}
		staged = append(staged, file)
		persisted = append(persisted, attachmentPersistence{Attachment: Attachment{OriginalName: file.OriginalName, ReportedMIME: file.ReportedMIME, DetectedMIME: file.DetectedMIME, MediaKind: file.MediaKind, Source: descriptor.Source, SizeBytes: file.SizeBytes, SHA256: file.SHA256, BrowserLastModifiedAt: descriptor.BrowserLastModifiedAt, EmbeddedMetadata: map[string]any{}, StorageState: "pending"}, StorageKey: file.StorageKey})
	}

	var update Update
	var created bool
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			policy, err = s.repository.GetTaskPolicy(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID, true)
			if errors.Is(err, ErrNotFound) {
				return Update{}, s.denied(ctx, actor, projectID, taskID, "", "task_not_accessible", audit)
			}
			if err != nil {
				return Update{}, err
			}
		}
		evidence := evaluateEvidence(metadata.Location, metadata.LocationUnavailableReason, policy.Geofence)
		for index := range persisted {
			persisted[index].VerificationStatus, persisted[index].VerificationReason = attachmentVerification(persisted[index].MediaKind, persisted[index].Source, evidence.LocationStatus)
		}
		requestHash, hashErr := createRequestHash(projectID, taskID, content, evidence, persisted)
		if hashErr != nil {
			return Update{}, hashErr
		}
		input := progressPersistence{ContentMarkdown: content, IdempotencyKey: idempotencyKey, RequestSHA256: requestHash, Evidence: evidence}
		update, created, err = s.repository.CreateUpdate(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID, input, persisted, auditEvent{ActorUserID: actor.ID, Action: "progress.created", TargetType: "progress_update", Outcome: "succeeded", Context: audit, Details: map[string]any{"project_id": projectID, "task_id": taskID, "attachment_count": len(files), "location_status": evidence.LocationStatus}})
		if !errors.Is(err, ErrPolicyChanged) {
			break
		}
	}
	if errors.Is(err, ErrNotFound) {
		return Update{}, s.denied(ctx, actor, projectID, taskID, "", "task_not_accessible", audit)
	}
	if errors.Is(err, ErrPolicyChanged) {
		return Update{}, ErrConflict
	}
	if err != nil {
		return Update{}, err
	}
	if !created {
		for _, file := range staged {
			_ = s.storage.DiscardStaged(file.StorageKey)
		}
		staged = nil
		if err := s.completeAttachments(ctx, &update, audit); err != nil {
			return Update{}, err
		}
		if err := s.renderUpdate(&update); err != nil {
			return Update{}, err
		}
		return update, nil
	}
	ownedByDatabase = true
	if err := s.completeAttachments(ctx, &update, audit); err != nil {
		return Update{}, err
	}
	if err := s.renderUpdate(&update); err != nil {
		return Update{}, err
	}
	return update, nil
}

func (s *Service) Update(ctx context.Context, actor identity.User, projectID, taskID, updateID string, input UpdateInput, audit AuditContext) (Update, error) {
	if err := requireAccess(actor); err != nil {
		return Update{}, err
	}
	content, err := validateContent(input.ContentMarkdown)
	if err != nil {
		return Update{}, err
	}
	if input.ExpectedVersion < 1 {
		return Update{}, &ValidationError{Field: "expected_version", Message: "must be a positive integer"}
	}
	update, err := s.repository.UpdateProgress(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID, updateID, progressPersistence{ContentMarkdown: content, ExpectedVersion: input.ExpectedVersion}, auditEvent{ActorUserID: actor.ID, Action: "progress.updated", TargetType: "progress_update", TargetID: updateID, Outcome: "succeeded", Context: audit, Details: map[string]any{"project_id": projectID, "task_id": taskID, "from_version": input.ExpectedVersion, "to_version": input.ExpectedVersion + 1}})
	if errors.Is(err, ErrNotFound) {
		return Update{}, s.denied(ctx, actor, projectID, taskID, updateID, "update_not_accessible_or_not_owner", audit)
	}
	if err != nil {
		return Update{}, err
	}
	if err := s.renderUpdate(&update); err != nil {
		return Update{}, err
	}
	return update, nil
}

type Download struct {
	Attachment Attachment
	Content    filestore.ReadSeekCloser
}

func (s *Service) Download(ctx context.Context, actor identity.User, projectID, taskID, updateID, attachmentID string, audit AuditContext) (Download, error) {
	if err := requireAccess(actor); err != nil {
		return Download{}, err
	}
	attachment, err := s.repository.GetAttachment(ctx, actor.ID, actor.Role == identity.RoleAdmin, projectID, taskID, updateID, attachmentID)
	if errors.Is(err, ErrNotFound) {
		return Download{}, s.deniedDownload(ctx, actor, projectID, taskID, updateID, attachmentID, audit)
	}
	if err != nil {
		return Download{}, err
	}
	if attachment.StorageState == "failed" {
		return Download{}, ErrAttachmentUnavailable
	}
	if attachment.StorageState != "available" {
		return Download{}, ErrAttachmentPending
	}
	content, err := s.storage.Open(attachment.storageKey)
	if err != nil {
		return Download{}, ErrAttachmentUnavailable
	}
	if err := s.repository.AppendAudit(ctx, auditEvent{ActorUserID: actor.ID, Action: "attachment.downloaded", TargetType: "progress_attachment", TargetID: attachment.ID, Outcome: "succeeded", Context: audit, Details: map[string]any{"project_id": projectID, "task_id": taskID, "progress_update_id": updateID}}); err != nil {
		content.Close()
		return Download{}, err
	}
	return Download{Attachment: attachment, Content: content}, nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	pending, err := s.repository.ListPendingAttachments(ctx)
	if err != nil {
		return fmt.Errorf("list pending attachments: %w", err)
	}
	keep := make(map[string]bool, len(pending))
	for _, item := range pending {
		keep[item.StorageKey] = true
	}
	for _, item := range pending {
		audit := AuditContext{RequestID: "startup-reconcile", ClientIP: "127.0.0.1", UserAgent: "ppr-startup"}
		if err := s.storage.Finalize(item.StorageKey); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("finalize pending attachment %s: %w", item.ID, err)
			}
			if markErr := s.repository.MarkAttachmentFailed(ctx, item.ID, "staged_bytes_unavailable", auditEvent{ActorUserID: item.UploadedBy, Action: "attachment.failed", TargetType: "progress_attachment", TargetID: item.ID, Outcome: "failed", Context: audit, Details: map[string]any{"reason": "staged_bytes_unavailable"}}); markErr != nil {
				return markErr
			}
			continue
		}
		if _, err := s.repository.MarkAttachmentAvailable(ctx, item.ID, auditEvent{ActorUserID: item.UploadedBy, Action: "attachment.available", TargetType: "progress_attachment", TargetID: item.ID, Outcome: "succeeded", Context: audit, Details: map[string]any{"reconciled": true}}); err != nil {
			return err
		}
	}
	if _, err := s.storage.CleanupOrphans(keep, time.Now().Add(-24*time.Hour)); err != nil {
		return err
	}
	return nil
}

func (s *Service) completeAttachments(ctx context.Context, update *Update, audit AuditContext) error {
	for index := range update.Attachments {
		attachment := &update.Attachments[index]
		if attachment.StorageState != "pending" {
			continue
		}
		if err := s.storage.Finalize(attachment.storageKey); err != nil {
			return ErrAttachmentPending
		}
		availableAt, err := s.repository.MarkAttachmentAvailable(ctx, attachment.ID, auditEvent{ActorUserID: update.CreatedBy.UserID, Action: "attachment.available", TargetType: "progress_attachment", TargetID: attachment.ID, Outcome: "succeeded", Context: audit, Details: map[string]any{"progress_update_id": update.ID}})
		if err != nil {
			return ErrAttachmentPending
		}
		attachment.StorageState = "available"
		attachment.AvailableAt = &availableAt
	}
	return nil
}

func (s *Service) renderUpdate(update *Update) error {
	html, err := s.renderer.Render(update.ContentMarkdown)
	if err != nil {
		return fmt.Errorf("render progress Markdown: %w", err)
	}
	update.ContentHTML = html
	for i := range update.Revisions {
		previous, err := s.renderer.Render(update.Revisions[i].PreviousContentMarkdown)
		if err != nil {
			return err
		}
		current, err := s.renderer.Render(update.Revisions[i].NewContentMarkdown)
		if err != nil {
			return err
		}
		update.Revisions[i].PreviousContentHTML = previous
		update.Revisions[i].NewContentHTML = current
	}
	if update.Attachments == nil {
		update.Attachments = []Attachment{}
	}
	if update.Revisions == nil {
		update.Revisions = []Revision{}
	}
	for index := range update.Attachments {
		attachment := &update.Attachments[index]
		attachment.SourceTrust = "browser_reported"
		attachment.EmbeddedMetadataTrust = "untrusted"
	}
	return nil
}

func createRequestHash(projectID, taskID, content string, evidence Evidence, attachments []attachmentPersistence) (string, error) {
	type hashAttachment struct {
		OriginalName          string     `json:"original_name"`
		ReportedMIME          string     `json:"reported_mime"`
		DetectedMIME          string     `json:"detected_mime"`
		MediaKind             string     `json:"media_kind"`
		Source                string     `json:"source"`
		VerificationStatus    string     `json:"verification_status"`
		VerificationReason    string     `json:"verification_reason"`
		SizeBytes             int64      `json:"size_bytes"`
		SHA256                string     `json:"sha256"`
		BrowserLastModifiedAt *time.Time `json:"browser_last_modified_at"`
	}
	stableAttachments := make([]hashAttachment, len(attachments))
	for index, attachment := range attachments {
		stableAttachments[index] = hashAttachment{
			OriginalName: attachment.OriginalName, ReportedMIME: attachment.ReportedMIME, DetectedMIME: attachment.DetectedMIME,
			MediaKind: attachment.MediaKind, Source: attachment.Source, VerificationStatus: attachment.VerificationStatus,
			VerificationReason: attachment.VerificationReason, SizeBytes: attachment.SizeBytes, SHA256: attachment.SHA256,
			BrowserLastModifiedAt: attachment.BrowserLastModifiedAt,
		}
	}
	payload := struct {
		ProjectID   string           `json:"project_id"`
		TaskID      string           `json:"task_id"`
		Content     string           `json:"content"`
		Evidence    Evidence         `json:"evidence"`
		Attachments []hashAttachment `json:"attachments"`
	}{projectID, taskID, content, evidence, stableAttachments}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode idempotent request: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func requireAccess(actor identity.User) error {
	if !actor.Enabled || strings.TrimSpace(actor.ID) == "" {
		return identity.ErrUnauthenticated
	}
	if actor.MustChangePassword {
		return ErrForbidden
	}
	return nil
}
func mapStorageError(index int, err error) error {
	switch {
	case errors.Is(err, filestore.ErrFileTooLarge):
		return &ValidationError{Field: fmt.Sprintf("files[%d]", index), Message: "exceeds the configured size limit"}
	case errors.Is(err, filestore.ErrFileEmpty):
		return &ValidationError{Field: fmt.Sprintf("files[%d]", index), Message: "must not be empty"}
	case errors.Is(err, filestore.ErrTypeNotAllowed):
		return &ValidationError{Field: fmt.Sprintf("files[%d]", index), Message: "type is not allowed"}
	default:
		return err
	}
}
func (s *Service) denied(ctx context.Context, actor identity.User, projectID, taskID, updateID, reason string, audit AuditContext) error {
	targetID, targetType := updateID, "progress_update"
	if targetID == "" {
		targetID = taskID
		targetType = "task"
	}
	if err := s.repository.AppendAudit(ctx, auditEvent{ActorUserID: actor.ID, Action: "authorization.progress_denied", TargetType: targetType, TargetID: targetID, Outcome: "denied", Context: audit, Details: map[string]any{"project_id": projectID, "task_id": taskID, "reason": reason}}); err != nil {
		return err
	}
	return ErrNotFound
}
func (s *Service) deniedDownload(ctx context.Context, actor identity.User, projectID, taskID, updateID, attachmentID string, audit AuditContext) error {
	if err := s.repository.AppendAudit(ctx, auditEvent{ActorUserID: actor.ID, Action: "authorization.attachment_denied", TargetType: "progress_attachment", TargetID: attachmentID, Outcome: "denied", Context: audit, Details: map[string]any{"project_id": projectID, "task_id": taskID, "progress_update_id": updateID}}); err != nil {
		return err
	}
	return ErrNotFound
}
