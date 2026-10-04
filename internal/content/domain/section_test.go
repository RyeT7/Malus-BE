package domain

import (
	"errors"
	"testing"
	"time"

	"malus-be/internal/kernel"
)

func mustContent(t *testing.T, title, body string) Content {
	t.Helper()
	c, err := NewContent(title, body)
	if err != nil {
		t.Fatalf("NewContent: %v", err)
	}
	return c
}

func TestNewContentValidates(t *testing.T) {
	if _, err := NewContent("   ", "body"); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid for blank title, got %v", err)
	}
}

func TestPublishCreatesSequentialVersions(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	s := NewSection(KindBiodata, mustContent(t, "About me", "v1"), now)

	v1, err := s.Publish(now)
	if err != nil || v1.Number != 1 {
		t.Fatalf("first publish: version=%d err=%v", v1.Number, err)
	}
	if _, err := s.Publish(now); !errors.Is(err, kernel.ErrConflict) {
		t.Fatalf("want ErrConflict when nothing changed, got %v", err)
	}

	s.EditDraft(mustContent(t, "About me", "v2"), now)
	if !s.HasUnpublishedChanges() {
		t.Fatal("want unpublished changes after edit")
	}
	v2, err := s.Publish(now)
	if err != nil || v2.Number != 2 {
		t.Fatalf("second publish: version=%d err=%v", v2.Number, err)
	}
}

func TestRollbackRepublishesOldContent(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	s := NewSection(KindWorkplan, mustContent(t, "Plan", "first"), now)
	_, _ = s.Publish(now)
	s.EditDraft(mustContent(t, "Plan", "second"), now)
	_, _ = s.Publish(now)

	v, err := s.Rollback(1, now)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if v.Number != 3 || v.Content.Body != "first" {
		t.Fatalf("want version 3 with body %q, got %d %q", "first", v.Number, v.Content.Body)
	}
	if _, err := s.Rollback(9, now); !errors.Is(err, kernel.ErrNotFound) {
		t.Fatalf("want ErrNotFound for missing version, got %v", err)
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	s := NewSection(KindWhyMe, mustContent(t, "Why", "because"), now)
	_, _ = s.Publish(now)

	r := Restore(s.Snapshot())
	if r.ID() != s.ID() || len(r.Versions()) != 1 || r.HasUnpublishedChanges() {
		t.Fatal("restored section does not match original")
	}
	if len(r.PullEvents()) != 0 {
		t.Fatal("restored section must not carry pending events")
	}
}
