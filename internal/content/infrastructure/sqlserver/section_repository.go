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

type SectionRepository struct {
	db *sql.DB
}

func NewSectionRepository(db *sql.DB) *SectionRepository {
	return &SectionRepository{db: db}
}

const selectSections = `
SELECT id, draft_title, draft_body, draft_layout, draft_items, created_at, updated_at, revision
FROM dbo.sections`

func (r *SectionRepository) Get(ctx context.Context, id kernel.ID) (*domain.Section, error) {
	sections, err := r.load(ctx, selectSections+` WHERE id = @id`, sql.Named("id", id.String()))
	if err != nil {
		return nil, err
	}
	if len(sections) == 0 {
		return nil, kernel.NotFound("section %s", id)
	}
	return sections[0], nil
}

func (r *SectionRepository) List(ctx context.Context) ([]*domain.Section, error) {
	return r.load(ctx, selectSections+` ORDER BY position, created_at`)
}

func (r *SectionRepository) Save(ctx context.Context, s *domain.Section) error {
	snap := s.Snapshot()
	draftItems, err := encodeItems(snap.Draft.Items)
	if err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if snap.Revision == 0 {
		_, err := tx.ExecContext(ctx, `
INSERT INTO dbo.sections (id, position, draft_title, draft_body, draft_layout, draft_items, created_at, updated_at, revision)
SELECT @id, COALESCE(MAX(position), 0) + 1, @title, @body, @layout, @items, @created_at, @updated_at, 1
FROM dbo.sections WITH (UPDLOCK, HOLDLOCK)`,
			sql.Named("id", snap.ID.String()),
			sql.Named("title", snap.Draft.Title),
			sql.Named("body", snap.Draft.Body),
			sql.Named("layout", string(snap.Draft.Layout)),
			sql.Named("items", draftItems),
			sql.Named("created_at", snap.CreatedAt),
			sql.Named("updated_at", snap.UpdatedAt),
		)
		if err != nil {
			return fmt.Errorf("insert section: %w", err)
		}
	} else {
		res, err := tx.ExecContext(ctx, `
UPDATE dbo.sections
SET draft_title = @title, draft_body = @body, draft_layout = @layout, draft_items = @items, updated_at = @updated_at, revision = revision + 1
WHERE id = @id AND revision = @revision`,
			sql.Named("id", snap.ID.String()),
			sql.Named("title", snap.Draft.Title),
			sql.Named("body", snap.Draft.Body),
			sql.Named("layout", string(snap.Draft.Layout)),
			sql.Named("items", draftItems),
			sql.Named("updated_at", snap.UpdatedAt),
			sql.Named("revision", snap.Revision),
		)
		if err != nil {
			return fmt.Errorf("update section: %w", err)
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return kernel.ErrConcurrentUpdate
		}
	}

	var stored int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(number), 0) FROM dbo.section_versions WHERE section_id = @id`,
		sql.Named("id", snap.ID.String()),
	).Scan(&stored); err != nil {
		return fmt.Errorf("read latest version: %w", err)
	}
	for _, v := range snap.Versions {
		if v.Number <= stored {
			continue
		}
		versionItems, err := encodeItems(v.Content.Items)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO dbo.section_versions (section_id, number, title, body, layout, items, published_at)
VALUES (@id, @number, @title, @body, @layout, @items, @published_at)`,
			sql.Named("id", snap.ID.String()),
			sql.Named("number", v.Number),
			sql.Named("title", v.Content.Title),
			sql.Named("body", v.Content.Body),
			sql.Named("layout", string(v.Content.Layout)),
			sql.Named("items", versionItems),
			sql.Named("published_at", v.PublishedAt),
		); err != nil {
			return fmt.Errorf("insert version %d: %w", v.Number, err)
		}
	}

	return tx.Commit()
}

func (r *SectionRepository) Delete(ctx context.Context, s *domain.Section) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	id := sql.Named("id", s.ID().String())
	if _, err := tx.ExecContext(ctx, `DELETE FROM dbo.section_versions WHERE section_id = @id`, id); err != nil {
		return fmt.Errorf("delete versions: %w", err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM dbo.sections WHERE id = @id AND revision = @revision`, id, sql.Named("revision", s.Revision()))
	if err != nil {
		return fmt.Errorf("delete section: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return kernel.ErrConcurrentUpdate
	}
	return tx.Commit()
}

func (r *SectionRepository) Reorder(ctx context.Context, ids []kernel.ID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	changed := kernel.Conflict("the sections changed while reordering; reload and try again")
	var total int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM dbo.sections WITH (UPDLOCK, HOLDLOCK)`).Scan(&total); err != nil {
		return fmt.Errorf("count sections: %w", err)
	}
	if total != len(ids) {
		return changed
	}
	for i, id := range ids {
		res, err := tx.ExecContext(ctx, `UPDATE dbo.sections SET position = @position WHERE id = @id`,
			sql.Named("position", i+1), sql.Named("id", id.String()))
		if err != nil {
			return fmt.Errorf("reorder section %s: %w", id, err)
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return changed
		}
	}
	return tx.Commit()
}

func (r *SectionRepository) load(ctx context.Context, query string, args ...any) ([]*domain.Section, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query sections: %w", err)
	}
	defer rows.Close()

	var snaps []domain.Snapshot
	index := make(map[string]int)
	for rows.Next() {
		var (
			snap               domain.Snapshot
			id, layout, items  string
			createdAt, updated time.Time
		)
		if err := rows.Scan(&id, &snap.Draft.Title, &snap.Draft.Body, &layout, &items, &createdAt, &updated, &snap.Revision); err != nil {
			return nil, err
		}
		if snap.Draft.Items, err = decodeItems(items); err != nil {
			return nil, err
		}
		id = strings.TrimSpace(id)
		snap.ID = kernel.ID(id)
		snap.Draft.Layout = domain.Layout(layout)
		snap.CreatedAt = createdAt.UTC()
		snap.UpdatedAt = updated.UTC()
		index[id] = len(snaps)
		snaps = append(snaps, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(snaps) == 0 {
		return nil, nil
	}

	if err := r.loadVersions(ctx, snaps, index); err != nil {
		return nil, err
	}

	sections := make([]*domain.Section, len(snaps))
	for i, snap := range snaps {
		sections[i] = domain.Restore(snap)
	}
	return sections, nil
}

func (r *SectionRepository) loadVersions(ctx context.Context, snaps []domain.Snapshot, index map[string]int) error {
	placeholders := make([]string, len(snaps))
	args := make([]any, len(snaps))
	for i, snap := range snaps {
		name := fmt.Sprintf("id%d", i)
		placeholders[i] = "@" + name
		args[i] = sql.Named(name, snap.ID.String())
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT section_id, number, title, body, layout, items, published_at
FROM dbo.section_versions
WHERE section_id IN (`+strings.Join(placeholders, ", ")+`)
ORDER BY section_id, number`, args...)
	if err != nil {
		return fmt.Errorf("query versions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			sectionID, layout, items string
			v                        domain.Version
		)
		if err := rows.Scan(&sectionID, &v.Number, &v.Content.Title, &v.Content.Body, &layout, &items, &v.PublishedAt); err != nil {
			return err
		}
		v.Content.Layout = domain.Layout(layout)
		if v.Content.Items, err = decodeItems(items); err != nil {
			return err
		}
		v.PublishedAt = v.PublishedAt.UTC()
		i := index[strings.TrimSpace(sectionID)]
		snaps[i].Versions = append(snaps[i].Versions, v)
	}
	return rows.Err()
}
