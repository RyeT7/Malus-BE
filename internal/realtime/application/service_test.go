package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"malus-be/internal/kernel"
	"malus-be/internal/platform/messaging"
	"malus-be/internal/realtime/infrastructure/memory"
)

type fakeBroadcaster struct {
	mu        sync.Mutex
	published []LiveState
	fail      bool
}

func (f *fakeBroadcaster) Publish(_ context.Context, state LiveState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("broadcast down")
	}
	f.published = append(f.published, state)
	return nil
}

func (f *fakeBroadcaster) Connection(context.Context) (Connection, error) {
	return Connection{Kind: "fake", URL: "/fake"}, nil
}

func (f *fakeBroadcaster) last() LiveState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.published[len(f.published)-1]
}

func newTestService(t *testing.T) (*Service, *fakeBroadcaster) {
	t.Helper()
	b := &fakeBroadcaster{}
	now := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	svc := NewService(memory.NewSessionRepository(), b, messaging.NewMemoryBus("/test"), slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return now })
	return svc, b
}

func TestStartingSessionEndsThePreviousOne(t *testing.T) {
	svc, b := newTestService(t)
	ctx := context.Background()

	first, err := svc.StartSession(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.StartSession(ctx, 12)
	if err != nil {
		t.Fatal(err)
	}

	live, err := svc.LiveSession(ctx)
	if err != nil || live.ID != second.ID {
		t.Fatalf("want the second session live, got %+v err=%v", live, err)
	}
	old, _ := svc.GetSession(ctx, kernel.ID(first.ID))
	if old.Active {
		t.Fatal("previous session must be ended")
	}
	if len(b.published) != 3 || b.published[1].SessionID != first.ID || b.published[1].Active {
		t.Fatalf("want start, end-of-first, start-of-second broadcasts, got %+v", b.published)
	}
}

func TestSlideChangesBroadcastWithIncreasingVersion(t *testing.T) {
	svc, b := newTestService(t)
	ctx := context.Background()
	session, _ := svc.StartSession(ctx, 10)
	id := kernel.ID(session.ID)

	if _, err := svc.GoToSlide(ctx, id, 3); err != nil {
		t.Fatal(err)
	}
	afterThree := b.last()
	if afterThree.Slide != 3 || afterThree.Version != 2 {
		t.Fatalf("want slide 3 at version 2, got %+v", afterThree)
	}

	if _, err := svc.GoToSlide(ctx, id, 3); err != nil {
		t.Fatal(err)
	}
	resent := b.last()
	if resent.Version != 2 || len(b.published) != 3 {
		t.Fatalf("re-sending the same slide must rebroadcast without a new version, got %+v (%d broadcasts)", resent, len(b.published))
	}

	if _, err := svc.EndSession(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ended := b.last(); ended.Active || ended.Version != 3 {
		t.Fatalf("want inactive state at version 3, got %+v", ended)
	}
	if _, err := svc.LiveSession(ctx); !errors.Is(err, kernel.ErrNotFound) {
		t.Fatalf("want no live session after end, got %v", err)
	}
}

func TestBroadcastFailureDoesNotLoseTheSlide(t *testing.T) {
	svc, b := newTestService(t)
	ctx := context.Background()
	session, _ := svc.StartSession(ctx, 10)
	b.fail = true

	view, err := svc.GoToSlide(ctx, kernel.ID(session.ID), 5)
	if err != nil || view.Slide != 5 {
		t.Fatalf("slide change must be saved even if broadcasting fails: %+v err=%v", view, err)
	}
	live, _ := svc.LiveSession(ctx)
	if live.Slide != 5 {
		t.Fatalf("joiners must see slide 5, got %d", live.Slide)
	}
}

func TestOutOfRangeSlideIsRejected(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	session, _ := svc.StartSession(ctx, 4)
	if _, err := svc.GoToSlide(ctx, kernel.ID(session.ID), 4); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
}
