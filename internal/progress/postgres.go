package progress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &PostgresRepository{pool: pool}, nil
}

const updateProjection = `
 SELECT pu.id::text, p.id::text, pu.task_id::text, pu.content_markdown,
        creator.id::text, creator.username,
        pu.location_status, pu.location_reason,
        (pu.reported_latitude IS NOT NULL), COALESCE(pu.reported_latitude::double precision,0), COALESCE(pu.reported_longitude::double precision,0), COALESCE(pu.reported_accuracy_metres::double precision,0), pu.browser_observed_at,
        pu.location_unavailable_reason,
        (pu.geofence_id IS NOT NULL), COALESCE(pu.geofence_id::text,''), COALESCE(pu.geofence_version,0), COALESCE(pu.geofence_latitude::double precision,0), COALESCE(pu.geofence_longitude::double precision,0), COALESCE(pu.geofence_radius_metres::double precision,0), COALESCE(pu.geofence_max_accuracy_metres::double precision,0),
        pu.computed_distance_metres::double precision, pu.created_at, pu.updated_at, pu.version
 FROM public.progress_updates pu
 JOIN public.tasks t ON t.id=pu.task_id
 JOIN public.projects p ON p.id=t.project_id
 JOIN public.users creator ON creator.id=pu.created_by`

type scanner interface{ Scan(...any) error }

func scanUpdate(row scanner) (Update, error) {
	var update Update
	var hasLocation, hasGeofence bool
	var latitude, longitude, accuracy float64
	var observedAt *time.Time
	var unavailable *string
	var geofence GeofenceSnapshot
	var distance *float64
	err := row.Scan(&update.ID, &update.ProjectID, &update.TaskID, &update.ContentMarkdown, &update.CreatedBy.UserID, &update.CreatedBy.Username, &update.Evidence.LocationStatus, &update.Evidence.LocationReason, &hasLocation, &latitude, &longitude, &accuracy, &observedAt, &unavailable, &hasGeofence, &geofence.ID, &geofence.Version, &geofence.Latitude, &geofence.Longitude, &geofence.RadiusMetres, &geofence.MaxAccuracyMetres, &distance, &update.CreatedAt, &update.UpdatedAt, &update.Version)
	if err != nil {
		return Update{}, err
	}
	if hasLocation {
		update.Evidence.ReportedLocation = &ReportedLocation{Latitude: latitude, Longitude: longitude, AccuracyMetres: accuracy, BrowserObservedAt: observedAt}
	}
	update.Evidence.LocationUnavailableReason = unavailable
	if hasGeofence {
		update.Evidence.Geofence = &geofence
	}
	update.Evidence.ComputedDistanceMetres = distance
	return update, nil
}

func (r *PostgresRepository) GetTaskPolicy(ctx context.Context, actorID string, admin bool, projectID, taskID string, requireActive bool) (TaskPolicy, error) {
	var policy TaskPolicy
	var geofence GeofenceSnapshot
	var hasGeofence bool
	err := r.pool.QueryRow(ctx, `
 SELECT p.active,(g.id IS NOT NULL),COALESCE(g.id::text,''),COALESCE(g.version,0),COALESCE(g.latitude::double precision,0),COALESCE(g.longitude::double precision,0),COALESCE(g.radius_metres::double precision,0),COALESCE(g.max_accuracy_metres::double precision,0)
 FROM public.tasks t JOIN public.projects p ON p.id=t.project_id
 LEFT JOIN public.project_geofences g ON g.project_id=p.id AND g.valid_to IS NULL
 WHERE p.id=$3::uuid AND t.id=$4::uuid AND ($2::boolean OR EXISTS(SELECT 1 FROM public.project_members pm WHERE pm.project_id=p.id AND pm.user_id=$1::uuid AND pm.removed_at IS NULL))`, actorID, admin, projectID, taskID).Scan(&policy.Active, &hasGeofence, &geofence.ID, &geofence.Version, &geofence.Latitude, &geofence.Longitude, &geofence.RadiusMetres, &geofence.MaxAccuracyMetres)
	if errors.Is(err, pgx.ErrNoRows) {
		return TaskPolicy{}, ErrNotFound
	}
	if err != nil {
		return TaskPolicy{}, fmt.Errorf("query task evidence policy: %w", err)
	}
	if requireActive && !policy.Active {
		return TaskPolicy{}, ErrInactiveProject
	}
	if hasGeofence {
		policy.Geofence = &geofence
	}
	return policy, nil
}

func (r *PostgresRepository) ListUpdates(ctx context.Context, actorID string, admin bool, projectID, taskID string) ([]Update, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if _, err := lockAuthorizedTask(ctx, tx, actorID, admin, projectID, taskID, false); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, updateProjection+` WHERE p.id=$1::uuid AND t.id=$2::uuid ORDER BY pu.created_at,pu.id`, projectID, taskID)
	if err != nil {
		return nil, fmt.Errorf("query progress updates: %w", err)
	}
	updates := make([]Update, 0)
	for rows.Next() {
		update, err := scanUpdate(rows)
		if err != nil {
			return nil, err
		}
		update.Revisions = []Revision{}
		updates = append(updates, update)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for index := range updates {
		if err := r.loadAttachments(ctx, tx, &updates[index]); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updates, nil
}

func (r *PostgresRepository) GetUpdate(ctx context.Context, actorID string, admin bool, projectID, taskID, updateID string) (Update, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Update{}, err
	}
	defer rollback(tx)
	if _, err := lockAuthorizedTask(ctx, tx, actorID, admin, projectID, taskID, false); err != nil {
		return Update{}, err
	}
	update, err := r.loadUpdate(ctx, tx, projectID, taskID, updateID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return Update{}, ErrNotFound
	}
	if err != nil {
		return Update{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Update{}, err
	}
	return update, nil
}

func (r *PostgresRepository) CreateUpdate(ctx context.Context, actorID string, admin bool, projectID, taskID string, input progressPersistence, attachments []attachmentPersistence, event auditEvent) (Update, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Update{}, false, err
	}
	defer rollback(tx)
	policy, err := lockAuthorizedTask(ctx, tx, actorID, admin, projectID, taskID, true)
	if err != nil {
		return Update{}, false, err
	}
	if policyGeofenceID(policy) != evidenceGeofenceID(input.Evidence) {
		return Update{}, false, ErrPolicyChanged
	}
	var existingID, existingHash string
	err = tx.QueryRow(ctx, `SELECT id::text,request_sha256 FROM public.progress_updates WHERE created_by=$1::uuid AND idempotency_key=$2 FOR SHARE`, actorID, input.IdempotencyKey).Scan(&existingID, &existingHash)
	if err == nil {
		if existingHash != input.RequestSHA256 {
			return Update{}, false, ErrConflict
		}
		update, loadErr := r.loadUpdate(ctx, tx, projectID, taskID, existingID, true)
		if loadErr != nil {
			return Update{}, false, loadErr
		}
		if err := tx.Commit(ctx); err != nil {
			return Update{}, false, err
		}
		return update, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Update{}, false, err
	}
	var updateID string
	e := input.Evidence
	var location *ReportedLocation = e.ReportedLocation
	var lat, lon, accuracy any
	var observed any
	if location != nil {
		lat, lon, accuracy, observed = location.Latitude, location.Longitude, location.AccuracyMetres, location.BrowserObservedAt
	}
	var unavailable any = e.LocationUnavailableReason
	var gfID, gfVersion, gfLat, gfLon, gfRadius, gfAccuracy any
	if e.Geofence != nil {
		gfID, gfVersion, gfLat, gfLon, gfRadius, gfAccuracy = e.Geofence.ID, e.Geofence.Version, e.Geofence.Latitude, e.Geofence.Longitude, e.Geofence.RadiusMetres, e.Geofence.MaxAccuracyMetres
	}
	err = tx.QueryRow(ctx, `
 INSERT INTO public.progress_updates(task_id,content_markdown,created_by,idempotency_key,request_sha256,location_status,location_reason,reported_latitude,reported_longitude,reported_accuracy_metres,browser_observed_at,location_unavailable_reason,geofence_id,geofence_version,geofence_latitude,geofence_longitude,geofence_radius_metres,geofence_max_accuracy_metres,computed_distance_metres)
	VALUES($1::uuid,$2,$3::uuid,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,'')::uuid,$14,$15,$16,$17,$18,$19)
	ON CONFLICT (created_by,idempotency_key) DO NOTHING
	RETURNING id::text`, taskID, input.ContentMarkdown, actorID, input.IdempotencyKey, input.RequestSHA256, e.LocationStatus, e.LocationReason, lat, lon, accuracy, observed, unavailable, gfID, gfVersion, gfLat, gfLon, gfRadius, gfAccuracy, e.ComputedDistanceMetres).Scan(&updateID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `SELECT id::text,request_sha256 FROM public.progress_updates WHERE created_by=$1::uuid AND idempotency_key=$2 FOR SHARE`, actorID, input.IdempotencyKey).Scan(&existingID, &existingHash); err != nil {
			return Update{}, false, err
		}
		if existingHash != input.RequestSHA256 {
			return Update{}, false, ErrConflict
		}
		update, loadErr := r.loadUpdate(ctx, tx, projectID, taskID, existingID, true)
		if loadErr != nil {
			return Update{}, false, loadErr
		}
		if err := tx.Commit(ctx); err != nil {
			return Update{}, false, err
		}
		return update, false, nil
	}
	if err != nil {
		return Update{}, false, fmt.Errorf("insert progress update: %w", err)
	}
	for index := range attachments {
		attachment := &attachments[index]
		var id string
		var created time.Time
		metadata, _ := json.Marshal(attachment.EmbeddedMetadata)
		err = tx.QueryRow(ctx, `
  INSERT INTO public.progress_attachments(progress_update_id,original_name,storage_key,reported_mime,detected_mime,media_kind,source,verification_status,verification_reason,size_bytes,sha256,browser_last_modified_at,embedded_metadata,uploaded_by)
  VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14::uuid) RETURNING id::text,created_at`, updateID, attachment.OriginalName, attachment.StorageKey, attachment.ReportedMIME, attachment.DetectedMIME, attachment.MediaKind, attachment.Source, attachment.VerificationStatus, attachment.VerificationReason, attachment.SizeBytes, attachment.SHA256, attachment.BrowserLastModifiedAt, metadata, actorID).Scan(&id, &created)
		if err != nil {
			return Update{}, false, fmt.Errorf("insert progress attachment: %w", err)
		}
		attachment.ID = id
		attachment.CreatedAt = created
		attachment.storageKey = attachment.StorageKey
	}
	event.TargetID = updateID
	if err := insertAudit(ctx, tx, event); err != nil {
		return Update{}, false, err
	}
	update, err := r.loadUpdate(ctx, tx, projectID, taskID, updateID, true)
	if err != nil {
		return Update{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Update{}, false, err
	}
	return update, true, nil
}

func (r *PostgresRepository) UpdateProgress(ctx context.Context, actorID string, admin bool, projectID, taskID, updateID string, input progressPersistence, event auditEvent) (Update, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Update{}, err
	}
	defer rollback(tx)
	if _, err := lockAuthorizedTask(ctx, tx, actorID, admin, projectID, taskID, true); err != nil {
		return Update{}, err
	}
	var currentContent string
	var currentVersion int64
	err = tx.QueryRow(ctx, `SELECT content_markdown,version FROM public.progress_updates WHERE id=$1::uuid AND task_id=$2::uuid AND ($4::boolean OR created_by=$3::uuid) FOR UPDATE`, updateID, taskID, actorID, admin).Scan(&currentContent, &currentVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return Update{}, ErrNotFound
	}
	if err != nil {
		return Update{}, err
	}
	if currentVersion != input.ExpectedVersion {
		return Update{}, ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.progress_update_revisions(progress_update_id,from_version,to_version,previous_content_markdown,new_content_markdown,edited_by) VALUES($1::uuid,$2,$3,$4,$5,$6::uuid)`, updateID, currentVersion, currentVersion+1, currentContent, input.ContentMarkdown, actorID)
	if err != nil {
		return Update{}, fmt.Errorf("insert progress revision: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE public.progress_updates SET content_markdown=$2,updated_at=clock_timestamp(),version=version+1 WHERE id=$1::uuid`, updateID, input.ContentMarkdown)
	if err != nil {
		return Update{}, err
	}
	if err := insertAudit(ctx, tx, event); err != nil {
		return Update{}, err
	}
	update, err := r.loadUpdate(ctx, tx, projectID, taskID, updateID, true)
	if err != nil {
		return Update{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Update{}, err
	}
	return update, nil
}

func (r *PostgresRepository) GetAttachment(ctx context.Context, actorID string, admin bool, projectID, taskID, updateID, attachmentID string) (Attachment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Attachment{}, err
	}
	defer rollback(tx)
	if _, err := lockAuthorizedTask(ctx, tx, actorID, admin, projectID, taskID, false); err != nil {
		return Attachment{}, err
	}
	var attachment Attachment
	var metadata []byte
	err = tx.QueryRow(ctx, attachmentProjection+` WHERE pa.id=$1::uuid AND pa.progress_update_id=$2::uuid AND pu.task_id=$3::uuid`, attachmentID, updateID, taskID).Scan(attachmentScanTargets(&attachment, &metadata)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, ErrNotFound
	}
	if err != nil {
		return Attachment{}, err
	}
	_ = json.Unmarshal(metadata, &attachment.EmbeddedMetadata)
	if err := tx.Commit(ctx); err != nil {
		return Attachment{}, err
	}
	return attachment, nil
}

func (r *PostgresRepository) MarkAttachmentAvailable(ctx context.Context, attachmentID string, event auditEvent) (time.Time, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer rollback(tx)
	var availableAt time.Time
	err = tx.QueryRow(ctx, `UPDATE public.progress_attachments SET storage_state='available',available_at=clock_timestamp(),failure_reason='' WHERE id=$1::uuid AND storage_state='pending' RETURNING available_at`, attachmentID).Scan(&availableAt)
	if errors.Is(err, pgx.ErrNoRows) {
		var state string
		if err := tx.QueryRow(ctx, `SELECT storage_state,available_at FROM public.progress_attachments WHERE id=$1::uuid`, attachmentID).Scan(&state, &availableAt); err != nil {
			return time.Time{}, err
		}
		if state == "available" {
			return availableAt, tx.Commit(ctx)
		}
		return time.Time{}, ErrConflict
	}
	if err != nil {
		return time.Time{}, err
	}
	if err := insertAudit(ctx, tx, event); err != nil {
		return time.Time{}, err
	}
	return availableAt, tx.Commit(ctx)
}
func (r *PostgresRepository) MarkAttachmentFailed(ctx context.Context, attachmentID, reason string, event auditEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if _, err := tx.Exec(ctx, `UPDATE public.progress_attachments SET storage_state='failed',failure_reason=$2,available_at=NULL WHERE id=$1::uuid AND storage_state='pending'`, attachmentID, reason); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *PostgresRepository) ListPendingAttachments(ctx context.Context) ([]pendingAttachment, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text,storage_key,uploaded_by::text FROM public.progress_attachments WHERE storage_state='pending' ORDER BY created_at,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []pendingAttachment{}
	for rows.Next() {
		var item pendingAttachment
		if err := rows.Scan(&item.ID, &item.StorageKey, &item.UploadedBy); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (r *PostgresRepository) AppendAudit(ctx context.Context, event auditEvent) error {
	return insertAudit(ctx, r.pool, event)
}

func lockAuthorizedTask(ctx context.Context, tx pgx.Tx, actorID string, admin bool, projectID, taskID string, requireActive bool) (TaskPolicy, error) {
	var policy TaskPolicy
	var has bool
	var gf GeofenceSnapshot
	err := tx.QueryRow(ctx, `
 SELECT p.active,(g.id IS NOT NULL),COALESCE(g.id::text,''),COALESCE(g.version,0),COALESCE(g.latitude::double precision,0),COALESCE(g.longitude::double precision,0),COALESCE(g.radius_metres::double precision,0),COALESCE(g.max_accuracy_metres::double precision,0)
 FROM public.tasks t JOIN public.projects p ON p.id=t.project_id LEFT JOIN public.project_geofences g ON g.project_id=p.id AND g.valid_to IS NULL
 WHERE p.id=$3::uuid AND t.id=$4::uuid AND ($2::boolean OR EXISTS(SELECT 1 FROM public.project_members pm WHERE pm.project_id=p.id AND pm.user_id=$1::uuid AND pm.removed_at IS NULL)) FOR SHARE OF p,t`, actorID, admin, projectID, taskID).Scan(&policy.Active, &has, &gf.ID, &gf.Version, &gf.Latitude, &gf.Longitude, &gf.RadiusMetres, &gf.MaxAccuracyMetres)
	if errors.Is(err, pgx.ErrNoRows) {
		return TaskPolicy{}, ErrNotFound
	}
	if err != nil {
		return TaskPolicy{}, err
	}
	if requireActive && !policy.Active {
		return TaskPolicy{}, ErrInactiveProject
	}
	if has {
		policy.Geofence = &gf
	}
	return policy, nil
}
func policyGeofenceID(policy TaskPolicy) string {
	if policy.Geofence == nil {
		return ""
	}
	return policy.Geofence.ID
}
func evidenceGeofenceID(e Evidence) string {
	if e.Geofence == nil {
		return ""
	}
	return e.Geofence.ID
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r *PostgresRepository) loadUpdate(ctx context.Context, q queryer, projectID, taskID, updateID string, revisions bool) (Update, error) {
	update, err := scanUpdate(q.QueryRow(ctx, updateProjection+` WHERE p.id=$1::uuid AND t.id=$2::uuid AND pu.id=$3::uuid`, projectID, taskID, updateID))
	if err != nil {
		return Update{}, err
	}
	if err := r.loadAttachments(ctx, q, &update); err != nil {
		return Update{}, err
	}
	if revisions {
		if err := r.loadRevisions(ctx, q, &update); err != nil {
			return Update{}, err
		}
	} else {
		update.Revisions = []Revision{}
	}
	return update, nil
}

const attachmentProjection = `SELECT pa.id::text,pa.original_name,pa.storage_key,pa.reported_mime,pa.detected_mime,pa.media_kind,pa.source,pa.verification_status,pa.verification_reason,pa.size_bytes,pa.sha256,pa.browser_last_modified_at,pa.embedded_metadata,pa.storage_state,pa.failure_reason,pa.created_at,pa.available_at FROM public.progress_attachments pa JOIN public.progress_updates pu ON pu.id=pa.progress_update_id`

func attachmentScanTargets(a *Attachment, metadata *[]byte) []any {
	return []any{&a.ID, &a.OriginalName, &a.storageKey, &a.ReportedMIME, &a.DetectedMIME, &a.MediaKind, &a.Source, &a.VerificationStatus, &a.VerificationReason, &a.SizeBytes, &a.SHA256, &a.BrowserLastModifiedAt, metadata, &a.StorageState, &a.FailureReason, &a.CreatedAt, &a.AvailableAt}
}
func (r *PostgresRepository) loadAttachments(ctx context.Context, q queryer, update *Update) error {
	rows, err := q.Query(ctx, attachmentProjection+` WHERE pa.progress_update_id=$1::uuid ORDER BY pa.created_at,pa.id`, update.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	update.Attachments = []Attachment{}
	for rows.Next() {
		var a Attachment
		var metadata []byte
		if err := rows.Scan(attachmentScanTargets(&a, &metadata)...); err != nil {
			return err
		}
		_ = json.Unmarshal(metadata, &a.EmbeddedMetadata)
		update.Attachments = append(update.Attachments, a)
	}
	return rows.Err()
}
func (r *PostgresRepository) loadRevisions(ctx context.Context, q queryer, update *Update) error {
	rows, err := q.Query(ctx, `SELECT pr.id::text,pr.from_version,pr.to_version,pr.previous_content_markdown,pr.new_content_markdown,u.id::text,u.username,pr.edited_at FROM public.progress_update_revisions pr JOIN public.users u ON u.id=pr.edited_by WHERE pr.progress_update_id=$1::uuid ORDER BY pr.to_version`, update.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	update.Revisions = []Revision{}
	for rows.Next() {
		var rev Revision
		if err := rows.Scan(&rev.ID, &rev.FromVersion, &rev.ToVersion, &rev.PreviousContentMarkdown, &rev.NewContentMarkdown, &rev.EditedBy.UserID, &rev.EditedBy.Username, &rev.EditedAt); err != nil {
			return err
		}
		update.Revisions = append(update.Revisions, rev)
	}
	return rows.Err()
}

type auditExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func insertAudit(ctx context.Context, execer auditExecer, event auditEvent) error {
	if event.Details == nil {
		event.Details = map[string]any{}
	}
	details, err := json.Marshal(event.Details)
	if err != nil {
		return err
	}
	_, err = execer.Exec(ctx, `INSERT INTO public.audit_events(actor_user_id,action,target_type,target_id,outcome,request_id,client_ip,user_agent,details) VALUES(NULLIF($1,'')::uuid,$2,$3,NULLIF($4,'')::uuid,$5,$6,$7::inet,$8,$9::jsonb)`, event.ActorUserID, event.Action, event.TargetType, event.TargetID, event.Outcome, event.Context.RequestID, event.Context.ClientIP, event.Context.UserAgent, details)
	return err
}
func rollback(tx pgx.Tx) { _ = tx.Rollback(context.Background()) }
