package domain

import (
	"errors"
	"testing"
	"time"

	"malus-be/internal/kernel"
)

func TestUpvoteIsOncePerVoter(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	q, err := Ask("What comes first in semester one?", "", now)
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !q.Anonymous() {
		t.Fatal("want anonymous question when author is empty")
	}
	if err := q.Upvote("viewer-1", now); err != nil {
		t.Fatalf("Upvote: %v", err)
	}
	if err := q.Upvote("viewer-1", now); !errors.Is(err, kernel.ErrConflict) {
		t.Fatalf("want ErrConflict on duplicate vote, got %v", err)
	}
	if q.Votes() != 1 {
		t.Fatalf("want 1 vote, got %d", q.Votes())
	}
}

func TestAnsweredQuestionRejectsVotes(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	q, _ := Ask("Why this title?", "Ana", now)
	if err := q.MarkAnswered(now); err != nil {
		t.Fatalf("MarkAnswered: %v", err)
	}
	if err := q.Upvote("viewer-2", now); !errors.Is(err, kernel.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	if err := q.MarkAnswered(now); !errors.Is(err, kernel.ErrConflict) {
		t.Fatalf("want ErrConflict on second answer, got %v", err)
	}
}

func TestAskRejectsBlankText(t *testing.T) {
	if _, err := Ask("  ", "", time.Now()); !errors.Is(err, kernel.ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
}
