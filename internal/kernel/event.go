package kernel

import (
	"context"
	"time"
)

type Event interface {
	EventType() string
	AggregateID() ID
	OccurredAt() time.Time
}

type EventBase struct {
	Aggregate ID        `json:"aggregateId"`
	At        time.Time `json:"occurredAt"`
}

func NewEventBase(id ID, at time.Time) EventBase {
	return EventBase{Aggregate: id, At: at}
}

func (b EventBase) AggregateID() ID       { return b.Aggregate }
func (b EventBase) OccurredAt() time.Time { return b.At }

type AggregateRoot struct {
	events []Event
}

func (a *AggregateRoot) Record(e Event) {
	a.events = append(a.events, e)
}

func (a *AggregateRoot) PullEvents() []Event {
	events := a.events
	a.events = nil
	return events
}

type EventPublisher interface {
	Publish(ctx context.Context, events ...Event) error
}

type Clock func() time.Time

func SystemClock() time.Time { return time.Now().UTC() }
