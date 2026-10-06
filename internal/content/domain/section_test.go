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
	c, err := NewContent(title, body, items)
	if err != nil {
		t.Fatalf("NewContent: %v", err)
	}
	return c
}

func mustSection(t *testing.T, kind Kind, c Content) *Section {
	t.Helper()
	s, err := NewSection(kind, c, testNow)
	if err != nil {
		t.Fatalf("NewSection: %v", err)
	}
	return s
}

func facts(pairs ...string) []Item {
	items := make([]Item, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		items = append(items, Item{Heading: pairs[i], Detail: pairs[i+1]})
	}
	return items
}

func TestNewContentValidates(t *testing.T) {
	if _, err := NewContent("   ", "body", nil); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid for blank title, got %v", err)
	}
	if _, err := NewContent("Title", "", []Item{{Heading: "  ", Detail: "x"}}); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid for blank item heading, got %v", err)
	}
	if _, err := NewContent("Title", "", []Item{{Heading: "Plan", Semester: 13}}); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid for semester out of range, got %v", err)
	}
	c, err := NewContent("Title", "", []Item{{Heading: "  Name ", Detail: " Ryuu  "}})
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
}

func TestPublishCreatesSequentialVersions(t *testing.T) {
	s := mustSection(t, KindBiodata, mustContent(t, "About me", "", facts("Name", "Ryuu")...))

	v1, err := s.Publish(testNow)
	if err != nil || v1.Number != 1 {
		t.Fatalf("first publish: version=%d err=%v", v1.Number, err)
	}
	if _, err := s.Publish(testNow); !errors.Is(err, kernel.ErrConflict) {
		t.Fatalf("want ErrConflict when nothing changed, got %v", err)
	}

	if err := s.EditDraft(mustContent(t, "About me", "", facts("Name", "Ryuu", "Major", "Computer Science")...), testNow); err != nil {
		t.Fatal(err)
	}
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
	s := mustSection(t, KindWorkplan, plan("first"))
	if _, err := s.Publish(testNow); err != nil {
		t.Fatal(err)
	}
	if err := s.EditDraft(plan("second"), testNow); err != nil {
		t.Fatal(err)
	}
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

func TestPublishRulesPerKind(t *testing.T) {
	three := []Item{{Heading: "One"}, {Heading: "Two"}, {Heading: "Three"}}
	cases := []struct {
		name    string
		kind    Kind
		content Content
		ok      bool
	}{
		{"strengths with three items", KindStrengths, mustContent(t, "Strengths", "", three...), true},
		{"strengths with two items", KindStrengths, mustContent(t, "Strengths", "", three[:2]...), false},
		{"weaknesses with four items", KindWeaknesses, mustContent(t, "Weaknesses", "", append(three, Item{Heading: "Four"})...), false},
		{"workplan across two semesters", KindWorkplan, mustContent(t, "Workplan", "", Item{Heading: "A", Semester: 1}, Item{Heading: "B", Semester: 2}), true},
		{"workplan in one semester", KindWorkplan, mustContent(t, "Workplan", "", Item{Heading: "A", Semester: 1}, Item{Heading: "B", Semester: 1}), false},
		{"workplan item without semester", KindWorkplan, mustContent(t, "Workplan", "", Item{Heading: "A", Semester: 1}, Item{Heading: "B"}), false},
		{"biodata without facts", KindBiodata, mustContent(t, "About me", "intro only"), false},
		{"innovations with one item", KindInnovations, mustContent(t, "Innovations", "", Item{Heading: "Idea"}), true},
		{"proposed changes without items", KindProposedChanges, mustContent(t, "Changes", "text"), false},
		{"why me with body", KindWhyMe, mustContent(t, "Why me", "Because"), true},
		{"why me without body", KindWhyMe, mustContent(t, "Why me", "  ", Item{Heading: "Reason"}), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := mustSection(t, tc.kind, tc.content)
			_, err := s.Publish(testNow)
			if tc.ok && err != nil {
				t.Fatalf("want publish to succeed, got %v", err)
			}
			if !tc.ok && !errors.Is(err, kernel.ErrConflict) {
				t.Fatalf("want ErrConflict, got %v", err)
			}
		})
	}
}

func TestDraftsMayBeIncomplete(t *testing.T) {
	s := mustSection(t, KindStrengths, mustContent(t, "Strengths", "", Item{Heading: "Only one so far"}))
	if !s.HasUnpublishedChanges() || len(s.Draft().Items) != 1 {
		t.Fatal("an incomplete draft must be accepted")
	}
}

func TestOnlyWorkplanItemsHaveSemesters(t *testing.T) {
	c := mustContent(t, "Strengths", "", Item{Heading: "Focus", Semester: 1})
	if _, err := NewSection(KindStrengths, c, testNow); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid on create, got %v", err)
	}
	s := mustSection(t, KindInnovations, mustContent(t, "Innovations", ""))
	if err := s.EditDraft(mustContent(t, "Innovations", "", Item{Heading: "Idea", Semester: 2}), testNow); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid on edit, got %v", err)
	}
}

func TestSectionDoesNotShareItemsWithCallers(t *testing.T) {
	items := []Item{{Heading: "Name", Detail: "Ryuu"}}
	s := mustSection(t, KindBiodata, mustContent(t, "About me", "", items...))
	draft := s.Draft()
	draft.Items[0].Detail = "changed"
	if s.Draft().Items[0].Detail != "Ryuu" {
		t.Fatal("mutating a returned draft must not change the aggregate")
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	s := mustSection(t, KindWhyMe, mustContent(t, "Why", "because", Item{Heading: "Track record"}))
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
