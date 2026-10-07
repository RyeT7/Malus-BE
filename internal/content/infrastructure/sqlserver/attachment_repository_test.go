package sqlserver

import (
	"context"
	"errors"
	"testing"
	"time"

	"malus-be/internal/content/domain"
	"malus-be/internal/kernel"
)

func TestSectionRepositoryPersistsSourcesAndAttachmentRefs(t *testing.T) {
	repo := NewSectionRepository(openTestDB(t))
	ctx := context.Background()
	now := time.Now().UTC()

	item := domain.Item{
		Heading:     "Certified",
		Sources:     []domain.Source{{Label: "Issuer", URL: "https://example.com/verify"}},
		Attachments: []kernel.ID{"11111111-2222-4333-8444-555555555555"},
	}
	s := mustSection(t, domain.KindInnovations, "Innovations", "", now, item)
	if err := repo.Save(ctx, s); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, s.ID())
	if err != nil {
		t.Fatal(err)
	}
	stored := got.Draft().Items[0]
	if len(stored.Sources) != 1 || stored.Sources[0] != item.Sources[0] || len(stored.Attachments) != 1 || stored.Attachments[0] != item.Attachments[0] {
		t.Fatalf("sources or attachment refs not persisted: %+v", stored)
	}
}

func TestAttachmentRepositoryRoundTrip(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `DELETE FROM dbo.attachments`); err != nil {
		t.Fatal(err)
	}
	repo := NewAttachmentRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)

	a, err := domain.NewAttachment("award.png", "image/png", 12, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, a); err != nil {
		t.Fatalf("insert: %v", err)
	}
	pending, err := repo.Get(ctx, a.ID())
	if err != nil || pending.Ready() {
		t.Fatalf("want pending attachment after insert, got %+v err=%v", pending, err)
	}

	if err := a.MarkReady(12, []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x00"), now); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, a); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := repo.Get(ctx, a.ID())
	if err != nil || !got.Ready() || got.FileName() != "award.png" || got.Size() != 12 || got.ContentType() != "image/png" {
		t.Fatalf("unexpected attachment after reload: ready=%v name=%q size=%d err=%v", got.Ready(), got.FileName(), got.Size(), err)
	}

	many, err := repo.GetMany(ctx, []kernel.ID{a.ID(), "missing"})
	if err != nil || len(many) != 1 {
		t.Fatalf("GetMany: %d found, err=%v", len(many), err)
	}
	if _, err := repo.Get(ctx, "missing"); !errors.Is(err, kernel.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
