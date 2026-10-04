package domain

import (
	"errors"
	"testing"
	"time"

	"malus-be/internal/kernel"
)

func TestGoToRecordsOnlyRealChanges(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	s, err := Start(5, now)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	s.PullEvents()

	if err := s.GoTo(0, now); err != nil || len(s.PullEvents()) != 0 {
		t.Fatalf("staying on the same slide must not emit events, err=%v", err)
	}
	if err := s.GoTo(3, now); err != nil {
		t.Fatalf("GoTo: %v", err)
	}
	if events := s.PullEvents(); len(events) != 1 || events[0].(SlideChanged).Slide != 3 {
		t.Fatalf("want one SlideChanged to 3, got %#v", events)
	}
	if err := s.GoTo(5, now); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid out of range, got %v", err)
	}
}
