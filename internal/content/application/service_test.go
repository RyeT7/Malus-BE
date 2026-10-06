package application

import (
	"context"
	"errors"
	"testing"

	"malus-be/internal/kernel"
)

func TestCreateReturnsPersistedRevision(t *testing.T) {
	svc := newTestService(t)
	view := create(t, svc, "why_me", "Why me", "body", false)
	if view.Revision != 1 {
		t.Fatalf("want revision 1 after create, got %d", view.Revision)
	}
}

func TestChangesRequireCurrentRevision(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	view := create(t, svc, "why_me", "Why me", "first", false)
	id := kernel.ID(view.ID)

	edited, err := svc.EditDraft(ctx, EditDraft{ID: id, ExpectedRevision: view.Revision, Title: "Why me", Body: "second"})
	if err != nil {
		t.Fatalf("edit with current revision: %v", err)
	}
	if edited.Revision != view.Revision+1 || edited.Draft.Body != "second" {
		t.Fatalf("want revision %d with new body, got %d %q", view.Revision+1, edited.Revision, edited.Draft.Body)
	}

	if _, err := svc.EditDraft(ctx, EditDraft{ID: id, ExpectedRevision: view.Revision, Title: "Why me", Body: "stale tab"}); !errors.Is(err, kernel.ErrPrecondition) {
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
