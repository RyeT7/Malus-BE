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

func mustSection(t *testing.T, kind domain.Kind, title, body string, now time.Time, items ...domain.Item) *domain.Section {
	t.Helper()
	content, err := domain.NewContent(title, body, items)
	if err != nil {
		t.Fatal(err)
	}
	s, err := domain.NewSection(kind, content, now)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func mustEdit(t *testing.T, s *domain.Section, title, body string, now time.Time, items ...domain.Item) {
	t.Helper()
	content, err := domain.NewContent(title, body, items)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EditDraft(content, now); err != nil {
		t.Fatal(err)
	}
}

func TestSectionRepositoryRoundTrip(t *testing.T) {
	repo := NewSectionRepository(openTestDB(t))
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	name := domain.Item{Heading: "Name", Detail: "Ryuu"}
	s := mustSection(t, domain.KindBiodata, "About me", "first", now, name)
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
	major := domain.Item{Heading: "Major", Detail: "Computer Science"}
	mustEdit(t, loaded, "About me", "second", now, name, major)
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
	v := got.Versions()
	if len(v) != 2 || v[1].Content.Body != "second" || got.Draft().Body != "second" {
		t.Fatalf("unexpected section after reload: draft=%q versions=%+v", got.Draft().Body, v)
	}
	if len(v[0].Content.Items) != 1 || len(v[1].Content.Items) != 2 || v[1].Content.Items[1] != major {
		t.Fatalf("version items not persisted: v1=%+v v2=%+v", v[0].Content.Items, v[1].Content.Items)
	}
	if items := got.Draft().Items; len(items) != 2 || items[0] != name {
		t.Fatalf("draft items not persisted: %+v", items)
	}

	all, err := repo.List(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("list: %d sections, err=%v", len(all), err)
	}
}

func TestSectionRepositoryPersistsWorkplanSemesters(t *testing.T) {
	repo := NewSectionRepository(openTestDB(t))
	ctx := context.Background()
	now := time.Now().UTC()

	s := mustSection(t, domain.KindWorkplan, "Workplan", "", now,
		domain.Item{Heading: "Onboarding", Detail: "Meet the team", Semester: 1},
		domain.Item{Heading: "Review", Semester: 2},
	)
	if err := repo.Save(ctx, s); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, s.ID())
	if err != nil {
		t.Fatal(err)
	}
	items := got.Draft().Items
	if len(items) != 2 || items[0].Semester != 1 || items[1].Semester != 2 || items[1].Detail != "" {
		t.Fatalf("unexpected workplan items: %+v", items)
	}
}

func TestSectionRepositoryDetectsConcurrentUpdate(t *testing.T) {
	repo := NewSectionRepository(openTestDB(t))
	ctx := context.Background()
	now := time.Now().UTC()

	if err := repo.Save(ctx, mustSection(t, domain.KindWorkplan, "Plan", "v1", now)); err != nil {
		t.Fatal(err)
	}
	first, _ := repo.FindByKind(ctx, domain.KindWorkplan)
	second, _ := repo.FindByKind(ctx, domain.KindWorkplan)

	mustEdit(t, first, "Plan", "from tab A", now)
	if err := repo.Save(ctx, first); err != nil {
		t.Fatalf("first writer: %v", err)
	}

	mustEdit(t, second, "Plan", "from tab B", now)
	if err := repo.Save(ctx, second); !errors.Is(err, kernel.ErrConcurrentUpdate) {
		t.Fatalf("second writer: want ErrConcurrentUpdate, got %v", err)
	}
}

func TestSectionRepositoryEnforcesUniqueKind(t *testing.T) {
	repo := NewSectionRepository(openTestDB(t))
	ctx := context.Background()
	now := time.Now().UTC()

	if err := repo.Save(ctx, mustSection(t, domain.KindWhyMe, "Why", "x", now)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, mustSection(t, domain.KindWhyMe, "Why", "x", now)); !errors.Is(err, kernel.ErrConflict) {
		t.Fatalf("want ErrConflict for duplicate kind, got %v", err)
	}
}
