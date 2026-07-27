package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/progress"
)

func progressUpdatesAPIHandler(options Options, projectID, taskID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, session, ok := authenticate(r, options)
			if !ok {
				writeError(w, 401, "unauthenticated", "Authentication is required.")
				return
			}
			updates, err := options.Progress.List(r.Context(), session.User, projectID, taskID, auditContext(r))
			if err != nil {
				writeProgressError(w, err)
				return
			}
			writeJSON(w, 200, map[string]any{"progress_updates": updates})
			return
		}
		session, ok := authenticatedWrite(w, r, options)
		if !ok {
			return
		}
		metadata, files, ok := decodeProgressMultipart(w, r, options)
		if !ok {
			return
		}
		defer r.MultipartForm.RemoveAll()
		update, err := options.Progress.Create(r.Context(), session.User, projectID, taskID, r.Header.Get("Idempotency-Key"), metadata, files, auditContext(r))
		if err != nil {
			writeProgressError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"progress_update": update})
	}
}

func progressUpdateAPIHandler(options Options, projectID, taskID, updateID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, session, ok := authenticate(r, options)
			if !ok {
				writeError(w, 401, "unauthenticated", "Authentication is required.")
				return
			}
			update, err := options.Progress.Get(r.Context(), session.User, projectID, taskID, updateID, auditContext(r))
			if err != nil {
				writeProgressError(w, err)
				return
			}
			writeJSON(w, 200, map[string]any{"progress_update": update})
			return
		}
		session, ok := authenticatedWrite(w, r, options)
		if !ok {
			return
		}
		var input progress.UpdateInput
		if !decodeJSON(w, r, &input) {
			return
		}
		update, err := options.Progress.Update(r.Context(), session.User, projectID, taskID, updateID, input, auditContext(r))
		if err != nil {
			writeProgressError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"progress_update": update})
	}
}

func progressAttachmentDownloadHandler(options Options, projectID, taskID, updateID, attachmentID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, session, ok := authenticate(r, options)
		if !ok {
			writeError(w, 401, "unauthenticated", "Authentication is required.")
			return
		}
		download, err := options.Progress.Download(r.Context(), session.User, projectID, taskID, updateID, attachmentID, auditContext(r))
		if err != nil {
			writeProgressError(w, err)
			return
		}
		defer download.Content.Close()
		disposition := mime.FormatMediaType("attachment", map[string]string{"filename": download.Attachment.OriginalName})
		if disposition == "" {
			writeError(w, 500, "internal_error", "The request could not be completed.")
			return
		}
		w.Header().Set("Content-Disposition", disposition)
		w.Header().Set("Content-Type", download.Attachment.DetectedMIME)
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(w, r, download.Attachment.OriginalName, time.Time{}, download.Content)
	}
}

func decodeProgressMultipart(w http.ResponseWriter, r *http.Request, options Options) (progress.CreateMetadata, []progress.UploadFile, bool) {
	media := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	if media != "multipart/form-data" {
		writeError(w, 415, "unsupported_media_type", "Content-Type must be multipart/form-data.")
		return progress.CreateMetadata{}, nil, false
	}
	maximum := options.AttachmentMaxBytes*int64(options.AttachmentMaxCount) + (2 << 20)
	r.Body = http.MaxBytesReader(w, r.Body, maximum)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, 413, "request_too_large", "The multipart request exceeds the configured limit.")
		} else {
			writeError(w, 400, "invalid_request", "The multipart request is invalid.")
		}
		return progress.CreateMetadata{}, nil, false
	}
	for key := range r.MultipartForm.Value {
		if key != "metadata" {
			writeError(w, 400, "invalid_request", "Only the metadata form field is allowed.")
			return progress.CreateMetadata{}, nil, false
		}
	}
	for key := range r.MultipartForm.File {
		if key != "files" {
			writeError(w, 400, "invalid_request", "Only repeated files form fields are allowed.")
			return progress.CreateMetadata{}, nil, false
		}
	}
	values := r.MultipartForm.Value["metadata"]
	if len(values) != 1 || len(values[0]) > 64<<10 {
		writeError(w, 400, "invalid_request", "Exactly one bounded metadata JSON field is required.")
		return progress.CreateMetadata{}, nil, false
	}
	var metadata progress.CreateMetadata
	decoder := json.NewDecoder(strings.NewReader(values[0]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metadata); err != nil {
		writeError(w, 400, "invalid_request", "The metadata JSON is invalid.")
		return progress.CreateMetadata{}, nil, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, 400, "invalid_request", "The metadata field must contain one JSON value.")
		return progress.CreateMetadata{}, nil, false
	}
	headers := r.MultipartForm.File["files"]
	files := make([]progress.UploadFile, 0, len(headers))
	for _, header := range headers {
		header := header
		files = append(files, progress.UploadFile{OriginalName: header.Filename, ReportedMIME: header.Header.Get("Content-Type"), Open: func() (io.ReadCloser, error) { return header.Open() }})
	}
	return metadata, files, true
}

func writeProgressError(w http.ResponseWriter, err error) {
	var validation *progress.ValidationError
	switch {
	case errors.As(err, &validation):
		writeError(w, 422, "validation_failed", validation.Error())
	case errors.Is(err, identity.ErrUnauthenticated):
		writeError(w, 401, "unauthenticated", "Authentication is required.")
	case errors.Is(err, progress.ErrForbidden):
		writeError(w, 403, "forbidden", "This operation is not permitted.")
	case errors.Is(err, progress.ErrNotFound):
		writeError(w, 404, "not_found", "The requested resource was not found in the authorized task scope.")
	case errors.Is(err, progress.ErrConflict):
		writeError(w, 409, "conflict", "The progress update already exists or changed since it was loaded.")
	case errors.Is(err, progress.ErrInactiveProject):
		writeError(w, 409, "project_inactive", "Progress cannot be changed in an inactive project.")
	case errors.Is(err, progress.ErrAttachmentPending):
		writeError(w, 503, "attachment_pending", "Attachment finalization is pending recovery; retry with the same Idempotency-Key.")
	case errors.Is(err, progress.ErrAttachmentUnavailable):
		writeError(w, 410, "attachment_unavailable", "The attachment metadata exists, but its bytes are unavailable.")
	default:
		writeError(w, 500, "internal_error", "The request could not be completed.")
	}
}
