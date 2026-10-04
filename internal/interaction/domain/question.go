package domain

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"malus-be/internal/kernel"
)

const maxQuestionRunes = 500

type Question struct {
	kernel.AggregateRoot
	id         kernel.ID
	text       string
	author     string
	voters     map[string]struct{}
	askedAt    time.Time
	answeredAt *time.Time
	etag       string
}

func Ask(text, author string, now time.Time) (*Question, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, kernel.Invalid("question text is required")
	}
	if utf8.RuneCountInString(text) > maxQuestionRunes {
		return nil, kernel.Invalid("question exceeds %d characters", maxQuestionRunes)
	}
	q := &Question{
		id:      kernel.NewID(),
		text:    text,
		author:  strings.TrimSpace(author),
		voters:  make(map[string]struct{}),
		askedAt: now,
	}
	q.Record(QuestionAsked{EventBase: kernel.NewEventBase(q.id, now)})
	return q, nil
}

func (q *Question) ID() kernel.ID          { return q.id }
func (q *Question) Text() string           { return q.text }
func (q *Question) Author() string         { return q.author }
func (q *Question) Anonymous() bool        { return q.author == "" }
func (q *Question) Votes() int             { return len(q.voters) }
func (q *Question) AskedAt() time.Time     { return q.askedAt }
func (q *Question) AnsweredAt() *time.Time { return q.answeredAt }
func (q *Question) Answered() bool         { return q.answeredAt != nil }

func (q *Question) Upvote(voter string, now time.Time) error {
	if voter == "" {
		return kernel.Invalid("voter is required")
	}
	if q.Answered() {
		return kernel.Conflict("question %s is already answered", q.id)
	}
	if _, ok := q.voters[voter]; ok {
		return kernel.Conflict("already upvoted question %s", q.id)
	}
	q.voters[voter] = struct{}{}
	q.Record(QuestionUpvoted{EventBase: kernel.NewEventBase(q.id, now), Votes: q.Votes()})
	return nil
}

func (q *Question) MarkAnswered(now time.Time) error {
	if q.Answered() {
		return kernel.Conflict("question %s is already answered", q.id)
	}
	q.answeredAt = &now
	q.Record(QuestionAnswered{EventBase: kernel.NewEventBase(q.id, now)})
	return nil
}

type Snapshot struct {
	ID         kernel.ID
	Text       string
	Author     string
	Voters     []string
	AskedAt    time.Time
	AnsweredAt *time.Time
	ETag       string
}

func (q *Question) Snapshot() Snapshot {
	voters := make([]string, 0, len(q.voters))
	for v := range q.voters {
		voters = append(voters, v)
	}
	return Snapshot{
		ID:         q.id,
		Text:       q.text,
		Author:     q.author,
		Voters:     voters,
		AskedAt:    q.askedAt,
		AnsweredAt: q.answeredAt,
		ETag:       q.etag,
	}
}

func Restore(snap Snapshot) *Question {
	voters := make(map[string]struct{}, len(snap.Voters))
	for _, v := range snap.Voters {
		voters[v] = struct{}{}
	}
	return &Question{
		id:         snap.ID,
		text:       snap.Text,
		author:     snap.Author,
		voters:     voters,
		askedAt:    snap.AskedAt,
		answeredAt: snap.AnsweredAt,
		etag:       snap.ETag,
	}
}

type QuestionRepository interface {
	Get(ctx context.Context, id kernel.ID) (*Question, error)
	List(ctx context.Context) ([]*Question, error)
	Save(ctx context.Context, q *Question) error
}
