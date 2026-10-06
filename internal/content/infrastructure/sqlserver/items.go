package sqlserver

import (
	"encoding/json"
	"fmt"

	"malus-be/internal/content/domain"
)

type itemRow struct {
	Heading  string `json:"heading"`
	Detail   string `json:"detail,omitempty"`
	Semester int    `json:"semester,omitempty"`
}

func encodeItems(items []domain.Item) (string, error) {
	rows := make([]itemRow, len(items))
	for i, item := range items {
		rows[i] = itemRow{Heading: item.Heading, Detail: item.Detail, Semester: item.Semester}
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
		items[i] = domain.Item{Heading: row.Heading, Detail: row.Detail, Semester: row.Semester}
	}
	return items, nil
}
