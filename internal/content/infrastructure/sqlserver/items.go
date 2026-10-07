package sqlserver

import (
	"encoding/json"
	"fmt"

	"malus-be/internal/content/domain"
	"malus-be/internal/kernel"
)

type sourceRow struct {
	Label string `json:"label,omitempty"`
	URL   string `json:"url"`
}

type itemRow struct {
	Heading     string      `json:"heading"`
	Detail      string      `json:"detail,omitempty"`
	Semester    int         `json:"semester,omitempty"`
	Sources     []sourceRow `json:"sources,omitempty"`
	Attachments []string    `json:"attachments,omitempty"`
}

func encodeItems(items []domain.Item) (string, error) {
	rows := make([]itemRow, len(items))
	for i, item := range items {
		row := itemRow{Heading: item.Heading, Detail: item.Detail, Semester: item.Semester}
		for _, src := range item.Sources {
			row.Sources = append(row.Sources, sourceRow{Label: src.Label, URL: src.URL})
		}
		for _, id := range item.Attachments {
			row.Attachments = append(row.Attachments, id.String())
		}
		rows[i] = row
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return "", fmt.Errorf("encode items: %w", err)
	}
	return string(b), nil
}

func decodeItems(raw string) ([]domain.Item, error) {
	var rows []itemRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, fmt.Errorf("decode items: %w", err)
	}
	items := make([]domain.Item, len(rows))
	for i, row := range rows {
		item := domain.Item{Heading: row.Heading, Detail: row.Detail, Semester: row.Semester}
		for _, src := range row.Sources {
			item.Sources = append(item.Sources, domain.Source{Label: src.Label, URL: src.URL})
		}
		for _, id := range row.Attachments {
			item.Attachments = append(item.Attachments, kernel.ID(id))
		}
		items[i] = item
	}
	return items, nil
}
