package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"malus-be/internal/content/domain"
	"malus-be/internal/kernel"
)

type AttachmentRepository struct {
	db *sql.DB
}

func NewAttachmentRepository(db *sql.DB) *AttachmentRepository {
	return &AttachmentRepository{db: db}
}

const selectAttachments = `
SELECT id, file_name, content_type, size_bytes, status, created_at, ready_at
FROM dbo.attachments`

func (r *AttachmentRepository) Get(ctx context.Context, id kernel.ID) (*domain.Attachment, error) {
	list, err := r.query(ctx, selectAttachments+` WHERE id = @id`, sql.Named("id", id.String()))
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, kernel.NotFound("attachment %s", id)
	}
	return list[0], nil
}

func (r *AttachmentRepository) GetMany(ctx context.Context, ids []kernel.ID) (map[kernel.ID]*domain.Attachment, error) {
	found := make(map[kernel.ID]*domain.Attachment, len(ids))
	if len(ids) == 0 {
		return found, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		name := fmt.Sprintf("id%d", i)
		placeholders[i] = "@" + name
		args[i] = sql.Named(name, id.String())
	}
	list, err := r.query(ctx, selectAttachments+` WHERE id IN (`+strings.Join(placeholders, ", ")+`)`, args...)
	if err != nil {
		return nil, err
	}
	for _, a := range list {
		found[a.ID()] = a
	}
	return found, nil
}

func (r *AttachmentRepository) List(ctx context.Context) ([]*domain.Attachment, error) {
	return r.query(ctx, selectAttachments+` ORDER BY created_at DESC`)
}

func (r *AttachmentRepository) Save(ctx context.Context, a *domain.Attachment) error {
	snap := a.Snapshot()
	var readyAt any
	if snap.ReadyAt != nil {
		readyAt = *snap.ReadyAt
	}
	_, err := r.db.ExecContext(ctx, `
MERGE dbo.attachments WITH (HOLDLOCK) AS target
USING (SELECT @id AS id) AS source ON target.id = source.id
WHEN MATCHED THEN
    UPDATE SET status = @status, ready_at = @ready_at
WHEN NOT MATCHED THEN
    INSERT (id, file_name, content_type, size_bytes, status, created_at, ready_at)
    VALUES (@id, @file_name, @content_type, @size, @status, @created_at, @ready_at);`,
		sql.Named("id", snap.ID.String()),
		sql.Named("file_name", snap.FileName),
		sql.Named("content_type", snap.ContentType),
		sql.Named("size", snap.Size),
		sql.Named("status", string(snap.Status)),
		sql.Named("created_at", snap.CreatedAt),
		sql.Named("ready_at", readyAt),
	)
	if err != nil {
		return fmt.Errorf("save attachment: %w", err)
	}
	return nil
}

func (r *AttachmentRepository) query(ctx context.Context, query string, args ...any) ([]*domain.Attachment, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query attachments: %w", err)
	}
	defer rows.Close()

	var list []*domain.Attachment
	for rows.Next() {
		var (
			snap      domain.AttachmentSnapshot
			id        string
			status    string
			createdAt time.Time
			readyAt   sql.NullTime
		)
		if err := rows.Scan(&id, &snap.FileName, &snap.ContentType, &snap.Size, &status, &createdAt, &readyAt); err != nil {
			return nil, err
		}
		snap.ID = kernel.ID(strings.TrimSpace(id))
		snap.Status = domain.AttachmentStatus(status)
		snap.CreatedAt = createdAt.UTC()
		if readyAt.Valid {
			t := readyAt.Time.UTC()
			snap.ReadyAt = &t
		}
		list = append(list, domain.RestoreAttachment(snap))
	}
	return list, rows.Err()
}
