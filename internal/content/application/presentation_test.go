package application

import (
	"context"
	"testing"
	"time"

	"malus-be/internal/content/infrastructure/memory"
	"malus-be/internal/kernel"
	"malus-be/internal/platform/messaging"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	now := time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)
	return NewService(memory.NewSectionRepository(), memory.NewAttachmentRepository(), newFakeBlobs(), messaging.NewMemoryBus("/test"), func() time.Time { return now })
}

func create(t *testing.T, svc *Service, layout, title, body string, publish bool, items ...ItemInput) SectionView {
	t.Helper()
	ctx := context.Background()
	view, err := svc.CreateSection(ctx, CreateSection{Layout: layout, Title: title, Body: body, Items: items})
	if err != nil {
		t.Fatalf("create %s: %v", title, err)
	}
	if publish {
		if view, err = svc.Publish(ctx, kernel.ID(view.ID), view.Revision); err != nil {
			t.Fatalf("publish %s: %v", title, err)
		}
	}
	return view
}

func TestPresentationShowsOnlyPublishedContentInSectionOrder(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	whyMe := create(t, svc, "list", "Why me", "published body", true)
	plan := create(t, svc, "timeline", "Workplan", "never published", false)
	bio := create(t, svc, "facts", "About me", "hello", true, ItemInput{Heading: "Name", Detail: "Ryuu"})

	if _, err := svc.EditDraft(ctx, EditDraft{ID: kernel.ID(whyMe.ID), ExpectedRevision: whyMe.Revision, Title: "Why me", Body: "unpublished edit", Layout: "list"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReorderSections(ctx, []string{bio.ID, plan.ID, whyMe.ID}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetPresentation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sections) != 2 {
		t.Fatalf("want 2 published sections, got %d", len(got.Sections))
	}
	if got.Sections[0].ID != bio.ID || got.Sections[1].ID != whyMe.ID {
		t.Fatalf("want About me then Why me, got %s then %s", got.Sections[0].Title, got.Sections[1].Title)
	}
	if got.Sections[0].Layout != "facts" || got.Sections[1].Layout != "list" {
		t.Fatalf("want published layouts, got %s and %s", got.Sections[0].Layout, got.Sections[1].Layout)
	}
	if got.Sections[1].Body != "published body" || got.Sections[1].Version != 1 {
		t.Fatalf("draft leaked into presentation: %+v", got.Sections[1])
	}
	if items := got.Sections[0].Items; len(items) != 1 || items[0].Heading != "Name" || items[0].Detail != "Ryuu" {
		t.Fatalf("want biodata items in presentation, got %+v", items)
	}
}

func TestPresentationRevisionChangesOnlyWhenPublishedContentChanges(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	name := ItemInput{Heading: "Name", Detail: "Ryuu"}
	bio := create(t, svc, "facts", "About me", "v1", true, name)
	first, err := svc.GetPresentation(ctx)
	if err != nil {
		t.Fatal(err)
	}

	edited, err := svc.EditDraft(ctx, EditDraft{ID: kernel.ID(bio.ID), ExpectedRevision: bio.Revision, Title: "About me", Body: "v2", Layout: "facts", Items: []ItemInput{name}})
	if err != nil {
		t.Fatal(err)
	}
	afterDraft, _ := svc.GetPresentation(ctx)
	if afterDraft.Revision != first.Revision {
		t.Fatal("revision changed although only the draft changed")
	}

	if _, err := svc.Publish(ctx, kernel.ID(bio.ID), edited.Revision); err != nil {
		t.Fatal(err)
	}
	afterPublish, _ := svc.GetPresentation(ctx)
	if afterPublish.Revision == first.Revision {
		t.Fatal("revision did not change after publishing")
	}

	other := create(t, svc, "list", "Other", "", true)
	beforeReorder, _ := svc.GetPresentation(ctx)
	if _, err := svc.ReorderSections(ctx, []string{other.ID, bio.ID}); err != nil {
		t.Fatal(err)
	}
	afterReorder, _ := svc.GetPresentation(ctx)
	if afterReorder.Revision == beforeReorder.Revision {
		t.Fatal("revision did not change after reordering")
	}
}
