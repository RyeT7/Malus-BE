package broadcast

import (
	"encoding/json"

	"malus-be/internal/realtime/application"
)

type stateMessage struct {
	Type       string `json:"type"`
	SessionID  string `json:"sessionId"`
	Slide      int    `json:"slide"`
	SlideCount int    `json:"slideCount"`
	Version    int64  `json:"version"`
	Active     bool   `json:"active"`
}

func encode(state application.LiveState) ([]byte, error) {
	return json.Marshal(stateMessage{
		Type:       "state",
		SessionID:  state.SessionID,
		Slide:      state.Slide,
		SlideCount: state.SlideCount,
		Version:    state.Version,
		Active:     state.Active,
	})
}
