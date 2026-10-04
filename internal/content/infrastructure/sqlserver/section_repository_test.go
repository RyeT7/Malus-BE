package sqlserver

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"malus-be/internal/content/domain"
	"malus-be/internal/kernel"
	"malus-be/internal/platform/sqldb"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MALUS_TEST_SQL_DSN")
	if dsn == "" {
		t.Skip("MALUS_TEST_SQL_DSN not set")
	}
	if !strings.HasSuffix(databaseName(dsn), "_test") {
		t.Fatal("MALUS_TEST_SQL_DSN must name a database ending in _test, because the test deletes all rows")
	}

	db, err := sqldb.Open(dsn, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if err := sqldb.Migrate(ctx, db, Migrations(), slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM dbo.section_versions; DELETE FROM dbo.sections;`); err != nil {
		t.Fatalf("clean tables: %v", err)
	}
	return db
}

func databaseName(dsn string) string {
	if u, err := url.Parse(dsn); err == nil && strings.EqualFold(u.Scheme, "sqlserver") {
		return u.Query().Get("database")
	}
	for _, part := range strings.Split(dsn, ";") {
		key, value, _ := strings.Cut(part, "=")
		if strings.EqualFold(strings.TrimSpace(key), "database") {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func TestSectionRepositoryRoundTrip(t *testing.T) {
	repo := NewSectionRepository(openTestDB(t))
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	content, _ := domain.NewContent("About me", "first")
	s := domain.NewSection(domain.KindBiodata, content, now)
	if _, err := s.Publish(now); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, s); err != nil {
		t.Fatalf("insert: %v", err)
	}

	loaded, err := repo.Get(ctx, s.ID())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	edited, _ := domain.NewContent("About me", "second")
	loaded.EditDraft(edited, now)
	if _, err := loaded.Publish(now); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, loaded); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.FindByKind(ctx, domain.KindBiodata)
	if err != nil {
		t.Fatalf("find by kind: %v", err)
	}
	if v := got.Versions(); len(v) != 2 || v[1].Content.Body != "second" || got.Draft().Body != "second" {
		t.Fatalf("unexpected section after reload: draft=%q versions=%+v", got.Draft().Body, v)
	}

	all, err := repo.List(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("list: %d sections, err=%v", len(all), err)
	}
}

func TestSectionRepositoryDetectsConcurrentUpdate(t *testing.T) {
	repo := NewSectionRepository(openTestDB(t))
	ctx := context.Background()
	now := time.Now().UTC()

	content, _ := domain.NewContent("Plan", "v1")
	if err := repo.Save(ctx, domain.NewSection(domain.KindWorkplan, content, now)); err != nil {
		t.Fatal(err)
	}
	first, _ := repo.FindByKind(ctx, domain.KindWorkplan)
	second, _ := repo.FindByKind(ctx, domain.KindWorkplan)

	a, _ := domain.NewContent("Plan", "from tab A")
	first.EditDraft(a, now)
	if err := repo.Save(ctx, first); err != nil {
		t.Fatalf("first writer: %v", err)
	}

	b, _ := domain.NewContent("Plan", "from tab B")
	second.EditDraft(b, now)
	if err := repo.Save(ctx, second); !errors.Is(err, kernel.ErrConcurrentUpdate) {
		t.Fatalf("second writer: want ErrConcurrentUpdate, got %v", err)
	}
}

func TestSectionRepositoryEnforcesUniqueKind(t *testing.T) {
	repo := NewSectionRepository(openTestDB(t))
	ctx := context.Background()
	now := time.Now().UTC()

	content, _ := domain.NewContent("Why", "x")
	if err := repo.Save(ctx, domain.NewSection(domain.KindWhyMe, content, now)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, domain.NewSection(domain.KindWhyMe, content, now)); !errors.Is(err, kernel.ErrConflict) {
		t.Fatalf("want ErrConflict for duplicate kind, got %v", err)
	}
}
