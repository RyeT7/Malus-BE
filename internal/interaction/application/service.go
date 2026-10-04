package application

import (
	"context"
	"errors"
	"math/rand/v2"
	"sort"
	"time"

	"malus-be/internal/interaction/domain"
	"malus-be/internal/kernel"
)

type Service struct {
	questions domain.QuestionRepository
	events    kernel.EventPublisher
	now       kernel.Clock
}

func NewService(questions domain.QuestionRepository, events kernel.EventPublisher, now kernel.Clock) *Service {
	return &Service{questions: questions, events: events, now: now}
}

type QuestionView struct {
	ID         string
	Text       string
	Author     string
	Anonymous  bool
	Votes      int
	Answered   bool
	AskedAt    time.Time
	AnsweredAt *time.Time
}

func toQuestionView(q *domain.Question) QuestionView {
	return QuestionView{
		ID:         q.ID().String(),
		Text:       q.Text(),
		Author:     q.Author(),
		Anonymous:  q.Anonymous(),
		Votes:      q.Votes(),
		Answered:   q.Answered(),
		AskedAt:    q.AskedAt(),
		AnsweredAt: q.AnsweredAt(),
	}
}

func (s *Service) AskQuestion(ctx context.Context, text, author string) (QuestionView, error) {
	q, err := domain.Ask(text, author, s.now())
	if err != nil {
		return QuestionView{}, err
	}
	if err := s.commit(ctx, q); err != nil {
		return QuestionView{}, err
	}
	return toQuestionView(q), nil
}

func (s *Service) Upvote(ctx context.Context, id kernel.ID, voter string) (QuestionView, error) {
	return s.mutate(ctx, id, func(q *domain.Question) error {
		return q.Upvote(voter, s.now())
	})
}

func (s *Service) MarkAnswered(ctx context.Context, id kernel.ID) (QuestionView, error) {
	return s.mutate(ctx, id, func(q *domain.Question) error {
		return q.MarkAnswered(s.now())
	})
}

func (s *Service) ListQuestions(ctx context.Context) ([]QuestionView, error) {
	questions, err := s.questions.List(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(questions, func(i, j int) bool {
		if questions[i].Answered() != questions[j].Answered() {
			return !questions[i].Answered()
		}
		if questions[i].Votes() != questions[j].Votes() {
			return questions[i].Votes() > questions[j].Votes()
		}
		return questions[i].AskedAt().Before(questions[j].AskedAt())
	})
	views := make([]QuestionView, 0, len(questions))
	for _, q := range questions {
		views = append(views, toQuestionView(q))
	}
	return views, nil
}

const maxConcurrentRetries = 5

func (s *Service) mutate(ctx context.Context, id kernel.ID, change func(*domain.Question) error) (QuestionView, error) {
	for attempt := 1; ; attempt++ {
		q, err := s.questions.Get(ctx, id)
		if err != nil {
			return QuestionView{}, err
		}
		if err := change(q); err != nil {
			return QuestionView{}, err
		}
		err = s.commit(ctx, q)
		if errors.Is(err, kernel.ErrConcurrentUpdate) && attempt < maxConcurrentRetries {
			if err := backoff(ctx, attempt); err != nil {
				return QuestionView{}, err
			}
			continue
		}
		if err != nil {
			return QuestionView{}, err
		}
		return toQuestionView(q), nil
	}
}

func backoff(ctx context.Context, attempt int) error {
	t := time.NewTimer(time.Duration(attempt) * time.Duration(5+rand.IntN(20)) * time.Millisecond)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (s *Service) commit(ctx context.Context, q *domain.Question) error {
	if err := s.questions.Save(ctx, q); err != nil {
		return err
	}
	return s.events.Publish(ctx, q.PullEvents()...)
}
