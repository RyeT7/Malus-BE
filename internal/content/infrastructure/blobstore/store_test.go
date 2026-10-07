package blobstore

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
)

const azuriteKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

func localStore(t *testing.T, public string) *Store {
	t.Helper()
	s, err := New(Config{
		ConnectionString: "DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=" + azuriteKey + ";BlobEndpoint=http://azurite:10000/devstoreaccount1;",
		Container:        "attachments",
		PublicEndpoint:   public,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC) }
	return s
}

func TestConnectionStringRefusedOutsideLocal(t *testing.T) {
	if _, err := New(Config{ConnectionString: "AccountName=a;AccountKey=" + azuriteKey, Container: "c"}, false); err == nil {
		t.Fatal("want error when a storage key is used outside local")
	}
	if _, err := New(Config{Container: "c"}, true); err == nil {
		t.Fatal("want error without any blob configuration")
	}
}

func TestUploadLinkUsesPublicEndpointAndWritePermissions(t *testing.T) {
	s := localStore(t, "http://localhost:10000/devstoreaccount1/")
	link, err := s.UploadURL(context.Background(), "abc/My File.pdf", "application/pdf", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "localhost:10000" || u.EscapedPath() != "/devstoreaccount1/attachments/abc/My%20File.pdf" {
		t.Fatalf("unexpected link location: %s %s", u.Host, u.EscapedPath())
	}
	q := u.Query()
	if q.Get("sp") != "cw" || q.Get("sr") != "b" || q.Get("sig") == "" || q.Get("se") != "2026-10-07T09:10:00Z" {
		t.Fatalf("unexpected SAS parameters: %v", q)
	}
}

func TestDownloadLinkIsReadOnlyWithFileName(t *testing.T) {
	s := localStore(t, "")
	link, err := s.DownloadURL(context.Background(), "abc/award.png", "Award “2026”.png", "image/png", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(link)
	q := u.Query()
	if u.Host != "azurite:10000" || q.Get("sp") != "r" || q.Get("rsct") != "image/png" {
		t.Fatalf("unexpected download link: %s %v", u.Host, q)
	}
	if d := q.Get("rscd"); !strings.HasPrefix(d, `inline; filename="Award _2026_.png"`) || !strings.Contains(d, "filename*=UTF-8''Award%20%E2%80%9C2026%E2%80%9D.png") {
		t.Fatalf("unexpected content disposition: %q", d)
	}
}
