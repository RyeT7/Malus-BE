package application

import (
	"context"
	"errors"
	"testing"

	"malus-be/internal/kernel"
)

func TestCreateReturnsPersistedRevision(t *testing.T) {
	svc := newTestService(t)
	view := create(t, svc, "list", "Why me", "body", false)
	if view.Revision != 1 {
		t.Fatalf("want revision 1 after create, got %d", view.Revision)
	}
}

func TestChangesRequireCurrentRevision(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	view := create(t, svc, "list", "Why me", "first", false)
	id := kernel.ID(view.ID)

	edited, err := svc.EditDraft(ctx, EditDraft{ID: id, ExpectedRevision: view.Revision, Title: "Why me", Body: "second", Layout: "list"})
	if err != nil {
		t.Fatalf("edit with current revision: %v", err)
	}
	if edited.Revision != view.Revision+1 || edited.Draft.Body != "second" {
		t.Fatalf("want revision %d with new body, got %d %q", view.Revision+1, edited.Revision, edited.Draft.Body)
	}

	if _, err := svc.EditDraft(ctx, EditDraft{ID: id, ExpectedRevision: view.Revision, Title: "Why me", Body: "stale tab", Layout: "list"}); !errors.Is(err, kernel.ErrPrecondition) {
		t.Fatalf("stale edit: want ErrPrecondition, got %v", err)
	}
	if _, err := svc.Publish(ctx, id, view.Revision); !errors.Is(err, kernel.ErrPrecondition) {
		t.Fatalf("stale publish: want ErrPrecondition, got %v", err)
	}

	published, err := svc.Publish(ctx, id, edited.Revision)
	if err != nil || published.Published == nil || published.Published.Number != 1 {
		t.Fatalf("publish with current revision: %+v err=%v", published.Published, err)
	}
	if _, err := svc.Rollback(ctx, id, 1, edited.Revision); !errors.Is(err, kernel.ErrPrecondition) {
		t.Fatalf("stale rollback: want ErrPrecondition, got %v", err)
	}

	current, _ := svc.GetSection(ctx, id)
	if current.Draft.Body != "second" {
		t.Fatalf("stale requests must not change the section, got body %q", current.Draft.Body)
	}
}

func TestCreateRejectsUnknownLayout(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.CreateSection(context.Background(), CreateSection{Title: "T", Layout: "grid"}); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
}

func TestDeleteRequiresCurrentRevision(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	view := create(t, svc, "list", "Temporary", "", true)
	id := kernel.ID(view.ID)

	if err := svc.DeleteSection(ctx, id, view.Revision+1); !errors.Is(err, kernel.ErrPrecondition) {
		t.Fatalf("stale delete: want ErrPrecondition, got %v", err)
	}
	if err := svc.DeleteSection(ctx, id, view.Revision); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.GetSection(ctx, id); !errors.Is(err, kernel.ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
	if got, _ := svc.GetPresentation(ctx); len(got.Sections) != 0 {
		t.Fatalf("deleted section still in presentation: %+v", got.Sections)
	}
}

func TestReorderValidatesTheFullList(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	a := create(t, svc, "list", "A", "", false)
	b := create(t, svc, "list", "B", "", false)

	for name, ids := range map[string][]string{
		"missing one": {a.ID},
		"duplicate":   {a.ID, a.ID},
		"unknown":     {a.ID, "nope"},
	} {
		if _, err := svc.ReorderSections(ctx, ids); !errors.Is(err, kernel.ErrInvalid) {
			t.Fatalf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	views, err := svc.ReorderSections(ctx, []string{b.ID, a.ID})
	if err != nil || len(views) != 2 || views[0].ID != b.ID {
		t.Fatalf("want B first, got %+v err=%v", views, err)
	}
}
