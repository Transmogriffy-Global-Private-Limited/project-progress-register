package progress

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const commentProjection = `
	SELECT c.id::text, c.progress_update_id::text, c.content_markdown,
	       author.id::text, author.username, c.created_at,
	       COALESCE(s.id::text, ''), COALESCE(acceptor.id::text, ''), COALESCE(acceptor.username, ''), s.accepted_at
	FROM public.update_comments c
	JOIN public.users author ON author.id = c.created_by
	LEFT JOIN public.accepted_suggestions s ON s.comment_id = c.id
	LEFT JOIN public.users acceptor ON acceptor.id = s.accepted_by`

func scanComment(row scanner) (Comment, error) {
	var comment Comment
	var acceptanceID, acceptorID, acceptorUsername string
	var acceptedAt *time.Time
	err := row.Scan(&comment.ID, &comment.ProgressUpdateID, &comment.ContentMarkdown, &comment.CreatedBy.UserID, &comment.CreatedBy.Username, &comment.CreatedAt, &acceptanceID, &acceptorID, &acceptorUsername, &acceptedAt)
	if err != nil {
		return Comment{}, err
	}
	if acceptanceID != "" && acceptedAt != nil {
		comment.AcceptedSuggestion = &SuggestionAcceptance{ID: acceptanceID, AcceptedBy: Actor{UserID: acceptorID, Username: acceptorUsername}, AcceptedAt: *acceptedAt}
	}
	return comment, nil
}

func (r *PostgresRepository) ListComments(ctx context.Context, actorID string, admin bool, projectID, taskID, updateID string) ([]Comment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if _, err := lockAuthorizedTask(ctx, tx, actorID, admin, projectID, taskID, false); err != nil {
		return nil, err
	}
	if err := requireScopedUpdate(ctx, tx, taskID, updateID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, commentProjection+` WHERE c.progress_update_id=$1::uuid ORDER BY c.created_at,c.id`, updateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	comments := []Comment{}
	for rows.Next() {
		comment, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *PostgresRepository) CreateComment(ctx context.Context, actorID string, admin bool, projectID, taskID, updateID, content string, event auditEvent) (Comment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Comment{}, err
	}
	defer rollback(tx)
	if _, err := lockAuthorizedTask(ctx, tx, actorID, admin, projectID, taskID, true); err != nil {
		return Comment{}, err
	}
	if err := requireScopedUpdate(ctx, tx, taskID, updateID); err != nil {
		return Comment{}, err
	}
	var commentID string
	if err := tx.QueryRow(ctx, `INSERT INTO public.update_comments(progress_update_id,content_markdown,created_by) VALUES($1::uuid,$2,$3::uuid) RETURNING id::text`, updateID, content, actorID).Scan(&commentID); err != nil {
		return Comment{}, err
	}
	event.TargetID = commentID
	if err := insertAudit(ctx, tx, event); err != nil {
		return Comment{}, err
	}
	comment, err := scanComment(tx.QueryRow(ctx, commentProjection+` WHERE c.id=$1::uuid`, commentID))
	if err != nil {
		return Comment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Comment{}, err
	}
	return comment, nil
}

func (r *PostgresRepository) AcceptSuggestion(ctx context.Context, actorID string, admin bool, projectID, taskID, updateID, commentID string, event auditEvent) (AcceptedSuggestion, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return AcceptedSuggestion{}, false, err
	}
	defer rollback(tx)
	if _, err := lockAuthorizedTask(ctx, tx, actorID, admin, projectID, taskID, true); err != nil {
		return AcceptedSuggestion{}, false, err
	}
	var lockedCommentID string
	err = tx.QueryRow(ctx, `SELECT c.id::text FROM public.update_comments c JOIN public.progress_updates pu ON pu.id=c.progress_update_id WHERE c.id=$1::uuid AND c.progress_update_id=$2::uuid AND pu.task_id=$3::uuid FOR UPDATE OF c`, commentID, updateID, taskID).Scan(&lockedCommentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return AcceptedSuggestion{}, false, ErrNotFound
	}
	if err != nil {
		return AcceptedSuggestion{}, false, err
	}
	var suggestionID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM public.accepted_suggestions WHERE comment_id=$1::uuid`, commentID).Scan(&suggestionID)
	created := false
	if errors.Is(err, pgx.ErrNoRows) {
		created = true
		if err := tx.QueryRow(ctx, `INSERT INTO public.accepted_suggestions(comment_id,accepted_by) VALUES($1::uuid,$2::uuid) RETURNING id::text`, commentID, actorID).Scan(&suggestionID); err != nil {
			return AcceptedSuggestion{}, false, err
		}
		event.TargetID = suggestionID
		if err := insertAudit(ctx, tx, event); err != nil {
			return AcceptedSuggestion{}, false, err
		}
	} else if err != nil {
		return AcceptedSuggestion{}, false, err
	}
	suggestion, err := scanAcceptedSuggestion(tx.QueryRow(ctx, acceptedSuggestionProjection+` WHERE s.id=$1::uuid`, suggestionID))
	if err != nil {
		return AcceptedSuggestion{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AcceptedSuggestion{}, false, err
	}
	return suggestion, created, nil
}

func (r *PostgresRepository) ListAcceptedSuggestions(ctx context.Context, actorID string, admin bool, projectID, taskID string) ([]AcceptedSuggestion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if _, err := lockAuthorizedTask(ctx, tx, actorID, admin, projectID, taskID, false); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, acceptedSuggestionProjection+` WHERE pu.task_id=$1::uuid ORDER BY s.accepted_at,s.id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AcceptedSuggestion{}
	for rows.Next() {
		item, err := scanAcceptedSuggestion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

const acceptedSuggestionProjection = `
	SELECT s.id::text,c.id::text,pu.id::text,pu.task_id::text,c.content_markdown,
	       author.id::text,author.username,c.created_at,acceptor.id::text,acceptor.username,s.accepted_at
	FROM public.accepted_suggestions s
	JOIN public.update_comments c ON c.id=s.comment_id
	JOIN public.progress_updates pu ON pu.id=c.progress_update_id
	JOIN public.users author ON author.id=c.created_by
	JOIN public.users acceptor ON acceptor.id=s.accepted_by`

func scanAcceptedSuggestion(row scanner) (AcceptedSuggestion, error) {
	var item AcceptedSuggestion
	err := row.Scan(&item.ID, &item.CommentID, &item.ProgressUpdateID, &item.TaskID, &item.ContentMarkdown, &item.CommentAuthor.UserID, &item.CommentAuthor.Username, &item.CommentedAt, &item.AcceptedBy.UserID, &item.AcceptedBy.Username, &item.AcceptedAt)
	return item, err
}

func requireScopedUpdate(ctx context.Context, tx pgx.Tx, taskID, updateID string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT true FROM public.progress_updates WHERE id=$1::uuid AND task_id=$2::uuid`, updateID, taskID).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("resolve progress update: %w", err)
	}
	return nil
}
