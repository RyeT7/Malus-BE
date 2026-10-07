package domain

import (
	"errors"
	"strings"
	"testing"

	"malus-be/internal/kernel"
)

func TestNewAttachmentValidates(t *testing.T) {
	cases := []struct {
		name, file, ctype string
		size              int64
	}{
		{"executable", "setup.exe", "application/x-msdownload", 100},
		{"empty", "a.pdf", "application/pdf", 0},
		{"too big", "a.pdf", "application/pdf", MaxAttachmentBytes + 1},
		{"no name", "   ", "application/pdf", 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewAttachment(tc.file, tc.ctype, tc.size, testNow); !errors.Is(err, kernel.ErrInvalid) {
				t.Fatalf("want ErrInvalid, got %v", err)
			}
		})
	}
}

func TestAttachmentFileNameIsSanitized(t *testing.T) {
	a, err := NewAttachment(`C:\Users\me\..\"Certificate"\Award 2026.pdf`, "Application/PDF", 1024, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if a.FileName() != "Award 2026.pdf" || a.ContentType() != "application/pdf" {
		t.Fatalf("got name %q type %q", a.FileName(), a.ContentType())
	}
	if want := a.ID().String() + "/Award-2026.pdf"; a.BlobName() != want {
		t.Fatalf("blob name %q, want %q", a.BlobName(), want)
	}
	if strings.ContainsAny(a.BlobName(), ` \"`) {
		t.Fatal("blob name must not contain spaces, backslashes or quotes")
	}
}

func TestMarkReadyChecksSizeAndContent(t *testing.T) {
	pdf := []byte("%PDF-1.7\n%....")
	a, _ := NewAttachment("plan.pdf", "application/pdf", 2048, testNow)

	if err := a.MarkReady(1000, pdf, testNow); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("size mismatch: want ErrInvalid, got %v", err)
	}
	if err := a.MarkReady(2048, []byte("MZ\x90\x00 not a pdf"), testNow); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("disguised file: want ErrInvalid, got %v", err)
	}
	if a.Ready() {
		t.Fatal("must stay pending after failed checks")
	}
	if err := a.MarkReady(2048, pdf, testNow); err != nil || !a.Ready() {
		t.Fatalf("valid pdf: ready=%v err=%v", a.Ready(), err)
	}
}

func TestSniffersRecogniseImages(t *testing.T) {
	cases := map[string][]byte{
		"image/png":  []byte("\x89PNG\r\n\x1a\n\x00\x00"),
		"image/jpeg": {0xFF, 0xD8, 0xFF, 0xE0},
		"image/webp": []byte("RIFF\x10\x00\x00\x00WEBPVP8 "),
	}
	for ctype, head := range cases {
		a, _ := NewAttachment("img", ctype, 10, testNow)
		if err := a.MarkReady(10, head, testNow); err != nil {
			t.Errorf("%s: %v", ctype, err)
		}
	}
}

func TestItemSourcesValidated(t *testing.T) {
	ok := []Item{{Heading: "Plan", Sources: []Source{{Label: " Docs ", URL: " https://example.com/a "}}}}
	c, err := NewContent("T", "", ok)
	if err != nil || c.Items[0].Sources[0] != (Source{Label: "Docs", URL: "https://example.com/a"}) {
		t.Fatalf("want trimmed source, got %+v err=%v", c.Items, err)
	}
	for _, bad := range []string{"javascript:alert(1)", "ftp://x", "not a url", "https://"} {
		if _, err := NewContent("T", "", []Item{{Heading: "P", Sources: []Source{{URL: bad}}}}); !errors.Is(err, kernel.ErrInvalid) {
			t.Errorf("%q: want ErrInvalid, got %v", bad, err)
		}
	}
	if _, err := NewContent("T", "", []Item{{Heading: "P", Attachments: []kernel.ID{"a", "a"}}}); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("duplicate attachment: want ErrInvalid, got %v", err)
	}
}

func TestContentEqualAndCloneCoverNestedLists(t *testing.T) {
	item := Item{Heading: "P", Sources: []Source{{URL: "https://a.test"}}, Attachments: []kernel.ID{"x"}}
	a := mustContent(t, "T", "", item)
	b := mustContent(t, "T", "", item)
	if !a.Equal(b) {
		t.Fatal("identical content must be equal")
	}
	b.Items[0].Attachments = []kernel.ID{"y"}
	if a.Equal(b) {
		t.Fatal("different attachments must not be equal")
	}
	clone := a.clone()
	clone.Items[0].Sources[0].URL = "https://changed.test"
	if a.Items[0].Sources[0].URL != "https://a.test" {
		t.Fatal("clone must not share source slices")
	}
	if ids := mustContent(t, "T", "", item, Item{Heading: "Q", Attachments: []kernel.ID{"x", "z"}}).AttachmentIDs(); len(ids) != 2 {
		t.Fatalf("want unique attachment ids [x z], got %v", ids)
	}
}
