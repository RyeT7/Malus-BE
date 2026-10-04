package domain

import (
	"errors"
	"testing"
)

func TestJobDeadLettersAfterMaxAttempts(t *testing.T) {
	j, err := NewJob("pdf.export", nil, 2)
	if err != nil {
		t.Fatalf("NewJob: %v", err)
	}

	_ = j.Start()
	j.Fail(errors.New("boom"))
	if !j.CanRetry() {
		t.Fatal("want retry after first failure")
	}

	_ = j.Start()
	j.Fail(errors.New("boom again"))
	if j.Status() != StatusDeadLettered || j.CanRetry() {
		t.Fatalf("want dead-lettered after max attempts, got %s", j.Status())
	}
	if err := j.Start(); err == nil {
		t.Fatal("dead-lettered job must not start")
	}
}
