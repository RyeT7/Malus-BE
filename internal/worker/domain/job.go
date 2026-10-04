package domain

import (
	"context"
	"strings"

	"malus-be/internal/kernel"
)

type Status string

const (
	StatusPending      Status = "pending"
	StatusRunning      Status = "running"
	StatusSucceeded    Status = "succeeded"
	StatusFailed       Status = "failed"
	StatusDeadLettered Status = "dead_lettered"
)

type Job struct {
	id          kernel.ID
	kind        string
	payload     []byte
	status      Status
	attempts    int
	maxAttempts int
	lastError   string
}

func NewJob(kind string, payload []byte, maxAttempts int) (*Job, error) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return nil, kernel.Invalid("job kind is required")
	}
	if maxAttempts < 1 {
		return nil, kernel.Invalid("max attempts must be at least 1")
	}
	return &Job{
		id:          kernel.NewID(),
		kind:        kind,
		payload:     append([]byte(nil), payload...),
		status:      StatusPending,
		maxAttempts: maxAttempts,
	}, nil
}

func (j *Job) ID() kernel.ID     { return j.id }
func (j *Job) Kind() string      { return j.kind }
func (j *Job) Payload() []byte   { return append([]byte(nil), j.payload...) }
func (j *Job) Status() Status    { return j.status }
func (j *Job) Attempts() int     { return j.attempts }
func (j *Job) LastError() string { return j.lastError }

func (j *Job) Start() error {
	if j.status != StatusPending && j.status != StatusFailed {
		return kernel.Conflict("job %s cannot start from status %s", j.id, j.status)
	}
	j.status = StatusRunning
	j.attempts++
	return nil
}

func (j *Job) Succeed() {
	j.status = StatusSucceeded
	j.lastError = ""
}

func (j *Job) Fail(err error) {
	j.lastError = err.Error()
	if j.attempts >= j.maxAttempts {
		j.status = StatusDeadLettered
		return
	}
	j.status = StatusFailed
}

func (j *Job) DeadLetter(reason string) {
	j.lastError = reason
	j.status = StatusDeadLettered
}

func (j *Job) CanRetry() bool { return j.status == StatusFailed }

type Queue interface {
	Receive(ctx context.Context) (*Job, error)
}
