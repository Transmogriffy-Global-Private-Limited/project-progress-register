package projects

import (
	"context"
	"encoding/json"
	"fmt"
)

func (r *PostgresRepository) GetTaskTimeline(ctx context.Context, actorID string, admin bool, projectID, taskID string, query timelinePersistenceQuery) ([]TimelineEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if err := lockAuthorizedProject(ctx, tx, projectID, actorID, admin, false); err != nil {
		return nil, err
	}
	if err := requireScopedTask(ctx, tx, projectID, taskID, false); err != nil {
		return nil, err
	}
	var cursorTime any
	var cursorID string
	if query.Cursor != nil {
		cursorTime = query.Cursor.OccurredAt
		cursorID = query.Cursor.ID
	}
	rows, err := tx.Query(ctx, `
		WITH timeline AS (
			SELECT 'task:'||t.id::text||':created' AS event_id,'task.created' AS action,'task' AS entity_type,t.id AS entity_id,
			       t.created_by AS actor_id,t.created_at AS occurred_at,
			       jsonb_build_object('version',1,'name',COALESCE(first_revision.previous_name,t.name),
			         'goals_markdown',COALESCE(first_revision.previous_goals_markdown,t.goals_markdown),
			         'description_markdown',COALESCE(first_revision.previous_description_markdown,t.description_markdown),
			         'responsible_user_id',(COALESCE(first_revision.previous_responsible_user_ids,current_responsibilities.user_ids))[1],
			         'responsible_user_ids',COALESCE(first_revision.previous_responsible_user_ids,current_responsibilities.user_ids),
			         'target_date',COALESCE(first_revision.previous_target_date,t.target_date)) AS metadata
			FROM public.tasks t
			LEFT JOIN LATERAL (
				SELECT r.previous_name,r.previous_goals_markdown,r.previous_description_markdown,r.previous_responsible_user_ids,r.previous_target_date
				FROM public.task_revisions r WHERE r.task_id=t.id ORDER BY r.from_version LIMIT 1
			) first_revision ON true
			LEFT JOIN LATERAL (
				SELECT ARRAY(SELECT tr.user_id FROM public.task_responsibilities tr WHERE tr.task_id=t.id ORDER BY tr.user_id) AS user_ids
			) current_responsibilities ON true
			WHERE t.id=$1::uuid

			UNION ALL
			SELECT 'task_revision:'||r.id::text,'task.updated','task',r.task_id,r.edited_by,r.edited_at,
			       jsonb_build_object('from_version',r.from_version,'to_version',r.to_version,'change_reason',r.change_reason,
			         'before',jsonb_build_object('name',r.previous_name,'goals_markdown',r.previous_goals_markdown,'description_markdown',r.previous_description_markdown,'responsible_user_id',r.previous_responsible_user_ids[1],'responsible_user_ids',r.previous_responsible_user_ids,'target_date',r.previous_target_date),
			         'after',jsonb_build_object('name',r.new_name,'goals_markdown',r.new_goals_markdown,'description_markdown',r.new_description_markdown,'responsible_user_id',r.new_responsible_user_ids[1],'responsible_user_ids',r.new_responsible_user_ids,'target_date',r.new_target_date))
			FROM public.task_revisions r WHERE r.task_id=$1::uuid

			UNION ALL
			SELECT 'progress:'||pu.id::text||':created','progress.created','progress_update',pu.id,pu.created_by,pu.created_at,
			       jsonb_build_object('version',1,'content_markdown',COALESCE(first_revision.previous_content_markdown,pu.content_markdown),
			         'location_status',pu.location_status,'location_reason',pu.location_reason,
			         'reported_location',CASE WHEN pu.reported_latitude IS NULL THEN NULL ELSE jsonb_build_object('latitude',pu.reported_latitude,'longitude',pu.reported_longitude,'accuracy_metres',pu.reported_accuracy_metres,'browser_observed_at',pu.browser_observed_at) END)
			FROM public.progress_updates pu
			LEFT JOIN LATERAL (
				SELECT r.previous_content_markdown FROM public.progress_update_revisions r WHERE r.progress_update_id=pu.id ORDER BY r.from_version LIMIT 1
			) first_revision ON true
			WHERE pu.task_id=$1::uuid

			UNION ALL
			SELECT 'progress_revision:'||r.id::text,'progress.updated','progress_update',r.progress_update_id,r.edited_by,r.edited_at,
			       jsonb_build_object('from_version',r.from_version,'to_version',r.to_version,'previous_content_markdown',r.previous_content_markdown,'new_content_markdown',r.new_content_markdown)
			FROM public.progress_update_revisions r JOIN public.progress_updates pu ON pu.id=r.progress_update_id WHERE pu.task_id=$1::uuid

			UNION ALL
			SELECT 'attachment:'||pa.id::text||':added','attachment.added','progress_attachment',pa.id,pa.uploaded_by,pa.created_at,
			       jsonb_build_object('progress_update_id',pa.progress_update_id,'original_name',pa.original_name,'detected_mime',pa.detected_mime,'media_kind',pa.media_kind,
			         'source',pa.source,'verification_status',pa.verification_status,'verification_reason',pa.verification_reason,'size_bytes',pa.size_bytes,'sha256',pa.sha256,
			         'storage_state','pending','failure_reason','')
			FROM public.progress_attachments pa JOIN public.progress_updates pu ON pu.id=pa.progress_update_id WHERE pu.task_id=$1::uuid

			UNION ALL
			SELECT 'attachment_audit:'||a.id::text,a.action,'progress_attachment',pa.id,COALESCE(a.actor_user_id,pa.uploaded_by),a.occurred_at,
			       jsonb_build_object('progress_update_id',pa.progress_update_id,'storage_state',pa.storage_state,'failure_reason',pa.failure_reason)
			FROM public.audit_events a JOIN public.progress_attachments pa ON pa.id=a.target_id JOIN public.progress_updates pu ON pu.id=pa.progress_update_id
			WHERE pu.task_id=$1::uuid AND a.action IN ('attachment.available','attachment.failed','attachment.downloaded')

			UNION ALL
			SELECT 'comment:'||c.id::text,'comment.created','update_comment',c.id,c.created_by,c.created_at,
			       jsonb_build_object('progress_update_id',c.progress_update_id,'content_markdown',c.content_markdown)
			FROM public.update_comments c JOIN public.progress_updates pu ON pu.id=c.progress_update_id WHERE pu.task_id=$1::uuid

			UNION ALL
			SELECT 'suggestion:'||s.id::text,'suggestion.accepted','accepted_suggestion',s.id,s.accepted_by,s.accepted_at,
			       jsonb_build_object('comment_id',c.id,'progress_update_id',c.progress_update_id,'content_markdown',c.content_markdown)
			FROM public.accepted_suggestions s JOIN public.update_comments c ON c.id=s.comment_id JOIN public.progress_updates pu ON pu.id=c.progress_update_id WHERE pu.task_id=$1::uuid

			UNION ALL
			SELECT 'assessment:'||a.id::text,'assessment.created','task_assessment',a.id,a.assessed_by,a.created_at,
			       jsonb_build_object('version',a.version,'verdict',a.verdict,'remark_markdown',a.remark_markdown)
			FROM public.task_assessments a WHERE a.task_id=$1::uuid
		)
		SELECT e.event_id,e.action,e.entity_type,e.entity_id::text,u.id::text,u.username,e.occurred_at,e.metadata
		FROM timeline e JOIN public.users u ON u.id=e.actor_id
		WHERE ($2::timestamptz IS NULL OR e.occurred_at>$2 OR (e.occurred_at=$2 AND e.event_id>$3))
		ORDER BY e.occurred_at,e.event_id
		LIMIT $4`, taskID, cursorTime, cursorID, query.Limit+1)
	if err != nil {
		return nil, fmt.Errorf("query task timeline: %w", err)
	}
	events := []TimelineEvent{}
	for rows.Next() {
		var event TimelineEvent
		var metadata []byte
		if err := rows.Scan(&event.ID, &event.Action, &event.EntityType, &event.EntityID, &event.Actor.UserID, &event.Actor.Username, &event.OccurredAt, &metadata); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan task timeline: %w", err)
		}
		if err := json.Unmarshal(metadata, &event.Metadata); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode task timeline metadata: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate task timeline: %w", err)
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return events, nil
}
