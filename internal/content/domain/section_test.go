package domain

import (
	"errors"
	"testing"
	"time"

	"malus-be/internal/kernel"
)

var testNow = time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)

func mustContent(t *testing.T, title, body string, items ...Item) Content {
	t.Helper()
	c, err := NewContent(title, body, LayoutList, items)
	if err != nil {
		t.Fatalf("NewContent: %v", err)
	}
	return c
}

func facts(pairs ...string) []Item {
	items := make([]Item, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		items = append(items, Item{Heading: pairs[i], Detail: pairs[i+1]})
	}
	return items
}

func TestNewContentValidates(t *testing.T) {
	if _, err := NewContent("   ", "body", LayoutList, nil); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid for blank title, got %v", err)
	}
	if _, err := NewContent("Title", "", LayoutList, []Item{{Heading: "  ", Detail: "x"}}); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid for blank item heading, got %v", err)
	}
	if _, err := NewContent("Title", "", LayoutList, []Item{{Heading: "Plan", Semester: 13}}); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid for semester out of range, got %v", err)
	}
	if _, err := NewContent("Title", "", Layout("grid"), nil); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid for unknown layout, got %v", err)
	}
	c, err := NewContent("Title", "", LayoutFacts, []Item{{Heading: "  Name ", Detail: " Ryuu  "}})
	if err != nil || c.Items[0].Heading != "Name" || c.Items[0].Detail != "Ryuu" {
		t.Fatalf("want trimmed item, got %+v err=%v", c.Items, err)
	}
}

func TestContentEqualComparesItems(t *testing.T) {
	a := mustContent(t, "T", "b", facts("Name", "Ryuu")...)
	b := mustContent(t, "T", "b", facts("Name", "Ryuu")...)
	c := mustContent(t, "T", "b", facts("Name", "Stanley")...)
	if !a.Equal(b) || a.Equal(c) {
		t.Fatal("Equal must compare items by value")
	}
	d := b
	d.Layout = LayoutFacts
	if a.Equal(d) {
		t.Fatal("Equal must compare layouts")
	}
}

func TestParseLayout(t *testing.T) {
	for _, in := range []string{"list", "facts", " timeline "} {
		if _, err := ParseLayout(in); err != nil {
			t.Fatalf("ParseLayout(%q): %v", in, err)
		}
	}
	if _, err := ParseLayout(""); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid for empty layout, got %v", err)
	}
}

func TestPublishCreatesSequentialVersions(t *testing.T) {
	s := NewSection(mustContent(t, "About me", "", facts("Name", "Ryuu")...), testNow)

	v1, err := s.Publish(testNow)
	if err != nil || v1.Number != 1 {
		t.Fatalf("first publish: version=%d err=%v", v1.Number, err)
	}
	if _, err := s.Publish(testNow); !errors.Is(err, kernel.ErrConflict) {
		t.Fatalf("want ErrConflict when nothing changed, got %v", err)
	}

	s.EditDraft(mustContent(t, "About me", "", facts("Name", "Ryuu", "Major", "Computer Science")...), testNow)
	if !s.HasUnpublishedChanges() {
		t.Fatal("want unpublished changes after adding an item")
	}
	v2, err := s.Publish(testNow)
	if err != nil || v2.Number != 2 || len(v2.Content.Items) != 2 {
		t.Fatalf("second publish: version=%d items=%d err=%v", v2.Number, len(v2.Content.Items), err)
	}
}

func TestRollbackRepublishesOldContent(t *testing.T) {
	plan := func(detail string) Content {
		return mustContent(t, "Plan", "", Item{Heading: "Onboarding", Detail: detail, Semester: 1}, Item{Heading: "Review", Detail: detail, Semester: 2})
	}
	s := NewSection(plan("first"), testNow)
	if _, err := s.Publish(testNow); err != nil {
		t.Fatal(err)
	}
	s.EditDraft(plan("second"), testNow)
	if _, err := s.Publish(testNow); err != nil {
		t.Fatal(err)
	}

	v, err := s.Rollback(1, testNow)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if v.Number != 3 || v.Content.Items[0].Detail != "first" {
		t.Fatalf("want version 3 with first detail, got %d %+v", v.Number, v.Content.Items)
	}
	if _, err := s.Rollback(9, testNow); !errors.Is(err, kernel.ErrNotFound) {
		t.Fatalf("want ErrNotFound for missing version, got %v", err)
	}
}

func TestAnyContentCanBePublished(t *testing.T) {
	for _, c := range []Content{
		mustContent(t, "Empty", ""),
		mustContent(t, "One item", "", Item{Heading: "Only one"}),
		mustContent(t, "Mixed", "intro", Item{Heading: "A", Semester: 3}, Item{Heading: "B"}),
	} {
		s := NewSection(c, testNow)
		if _, err := s.Publish(testNow); err != nil {
			t.Fatalf("publish %q: %v", c.Title, err)
		}
	}
}

func TestDeleteRecordsEvent(t *testing.T) {
	s := NewSection(mustContent(t, "Gone", ""), testNow)
	s.PullEvents()
	s.Delete(testNow)
	events := s.PullEvents()
	if len(events) != 1 || events[0].EventType() != "content.section.deleted" {
		t.Fatalf("want one deleted event, got %+v", events)
	}
}

func TestSectionDoesNotShareItemsWithCallers(t *testing.T) {
	items := []Item{{Heading: "Name", Detail: "Ryuu"}}
	s := NewSection(mustContent(t, "About me", "", items...), testNow)
	draft := s.Draft()
	draft.Items[0].Detail = "changed"
	if s.Draft().Items[0].Detail != "Ryuu" {
		t.Fatal("mutating a returned draft must not change the aggregate")
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	s := NewSection(mustContent(t, "Why", "because", Item{Heading: "Track record"}), testNow)
	if _, err := s.Publish(testNow); err != nil {
		t.Fatal(err)
	}

	r := Restore(s.Snapshot())
	if r.ID() != s.ID() || len(r.Versions()) != 1 || r.HasUnpublishedChanges() || len(r.Draft().Items) != 1 {
		t.Fatal("restored section does not match original")
	}
	if len(r.PullEvents()) != 0 {
		t.Fatal("restored section must not carry pending events")
	}
}
