package messaging

import (
	"encoding/json"
	"time"

	"malus-be/internal/kernel"
)

type Envelope struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Subject         string          `json:"subject,omitempty"`
	Time            time.Time       `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
}

func NewEnvelope(source string, e kernel.Event) (Envelope, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		SpecVersion:     "1.0",
		ID:              kernel.NewID().String(),
		Source:          source,
		Type:            "malus." + e.EventType() + ".v1",
		Subject:         e.AggregateID().String(),
		Time:            e.OccurredAt(),
		DataContentType: "application/json",
		Data:            data,
	}, nil
}
