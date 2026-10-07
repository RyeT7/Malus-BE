package application

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"malus-be/internal/content/infrastructure/memory"
	"malus-be/internal/kernel"
	"malus-be/internal/platform/messaging"
)

func newAttachmentService(t *testing.T) (*Service, *fakeBlobs) {
	t.Helper()
	blobs := newFakeBlobs()
	now := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	return NewService(memory.NewSectionRepository(), memory.NewAttachmentRepository(), blobs, messaging.NewMemoryBus("/test"), func() time.Time { return now }), blobs
}

func pdfBytes(size int) []byte {
	return append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte("x"), size-9)...)
}

func uploadReady(t *testing.T, svc *Service, blobs *fakeBlobs) AttachmentView {
	t.Helper()
	ctx := context.Background()
	ticket, err := svc.RequestUpload(ctx, RequestUpload{FileName: "certificate.pdf", ContentType: "application/pdf", Size: 64})
	if err != nil {
		t.Fatal(err)
	}
	blobs.put(strings.TrimPrefix(ticket.UploadURL, "https://blob.test/upload/"), pdfBytes(64))
	view, err := svc.CompleteUpload(ctx, kernel.ID(ticket.Attachment.ID))
	if err != nil || !view.Ready {
		t.Fatalf("complete: %+v err=%v", view, err)
	}
	return view
}

func TestUploadTicketAndCompletion(t *testing.T) {
	svc, blobs := newAttachmentService(t)
	ctx := context.Background()

	ticket, err := svc.RequestUpload(ctx, RequestUpload{FileName: "certificate.pdf", ContentType: "application/pdf", Size: 64})
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Attachment.Ready || ticket.Headers["x-ms-blob-type"] != "BlockBlob" || ticket.ExpiresAt.IsZero() {
		t.Fatalf("unexpected ticket: %+v", ticket)
	}
	id := kernel.ID(ticket.Attachment.ID)

	if _, err := svc.CompleteUpload(ctx, id); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("complete before upload: want ErrInvalid, got %v", err)
	}
	blob := strings.TrimPrefix(ticket.UploadURL, "https://blob.test/upload/")
	blobs.put(blob, pdfBytes(64))
	view, err := svc.CompleteUpload(ctx, id)
	if err != nil || !view.Ready {
		t.Fatalf("complete after upload: %+v err=%v", view, err)
	}
}

func TestDisguisedUploadIsRejectedAndDeleted(t *testing.T) {
	svc, blobs := newAttachmentService(t)
	ctx := context.Background()
	ticket, _ := svc.RequestUpload(ctx, RequestUpload{FileName: "cv.pdf", ContentType: "application/pdf", Size: 32})
	blob := strings.TrimPrefix(ticket.UploadURL, "https://blob.test/upload/")
	blobs.put(blob, append([]byte("MZ"), bytes.Repeat([]byte{0}, 30)...))

	if _, err := svc.CompleteUpload(ctx, kernel.ID(ticket.Attachment.ID)); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid for a disguised file, got %v", err)
	}
	if len(blobs.deleted) != 1 || blobs.deleted[0] != blob {
		t.Fatalf("rejected upload must be deleted, deleted=%v", blobs.deleted)
	}
}

func TestDraftsOnlyAcceptReadyAttachments(t *testing.T) {
	svc, blobs := newAttachmentService(t)
	ctx := context.Background()
	pending, _ := svc.RequestUpload(ctx, RequestUpload{FileName: "a.pdf", ContentType: "application/pdf", Size: 64})

	item := func(id string) []ItemInput {
		return []ItemInput{{Heading: "Fact", Detail: "x", Attachments: []string{id}}}
	}
	if _, err := svc.CreateSection(ctx, CreateSection{Kind: "biodata", Title: "Bio", Items: item(pending.Attachment.ID)}); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("pending attachment: want ErrInvalid, got %v", err)
	}
	if _, err := svc.CreateSection(ctx, CreateSection{Kind: "biodata", Title: "Bio", Items: item("does-not-exist")}); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("unknown attachment: want ErrInvalid, got %v", err)
	}

	ready := uploadReady(t, svc, blobs)
	view, err := svc.CreateSection(ctx, CreateSection{Kind: "biodata", Title: "Bio", Items: []ItemInput{{
		Heading: "Fact", Detail: "x",
		Sources:     []SourceInput{{Label: "Registry", URL: "https://example.com/registry"}},
		Attachments: []string{ready.ID},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	got := view.Draft.Items[0]
	if len(got.Attachments) != 1 || got.Attachments[0].FileName != "certificate.pdf" || got.Sources[0].URL != "https://example.com/registry" {
		t.Fatalf("draft item missing resolved attachment or source: %+v", got)
	}
}

func TestPublicDownloadsOnlyPublishedAttachments(t *testing.T) {
	svc, blobs := newAttachmentService(t)
	ctx := context.Background()
	file := uploadReady(t, svc, blobs)
	id := kernel.ID(file.ID)

	section, err := svc.CreateSection(ctx, CreateSection{Kind: "biodata", Title: "Bio", Items: []ItemInput{{Heading: "Fact", Detail: "x", Attachments: []string{file.ID}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AttachmentLink(ctx, id, false); !errors.Is(err, kernel.ErrNotFound) {
		t.Fatalf("draft-only attachment must be hidden from the public, got %v", err)
	}
	if link, err := svc.AttachmentLink(ctx, id, true); err != nil || !strings.Contains(link, "/download/") {
		t.Fatalf("admin must get a link for draft attachments: %q err=%v", link, err)
	}

	if _, err := svc.Publish(ctx, kernel.ID(section.ID), section.Revision); err != nil {
		t.Fatal(err)
	}
	if link, err := svc.AttachmentLink(ctx, id, false); err != nil || link == "" {
		t.Fatalf("published attachment must be downloadable: %q err=%v", link, err)
	}
	presentation, _ := svc.GetPresentation(ctx)
	if a := presentation.Sections[0].Items[0].Attachments; len(a) != 1 || a[0].ID != file.ID {
		t.Fatalf("presentation must list the attachment, got %+v", a)
	}
}
