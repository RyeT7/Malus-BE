package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"malus-be/internal/content/domain"
)

type PresentationSectionView struct {
	Kind        string
	Title       string
	Body        string
	Items       []ItemView
	Version     int
	PublishedAt time.Time
}

type PresentationView struct {
	Sections []PresentationSectionView
	Revision string
}

func (s *Service) GetPresentation(ctx context.Context) (PresentationView, error) {
	sections, err := s.repo.List(ctx)
	if err != nil {
		return PresentationView{}, err
	}
	byKind := make(map[domain.Kind]*domain.Section, len(sections))
	for _, section := range sections {
		byKind[section.Kind()] = section
	}

	view := PresentationView{Sections: make([]PresentationSectionView, 0, len(sections))}
	fingerprint := sha256.New()
	for _, kind := range domain.Kinds() {
		section, ok := byKind[kind]
		if !ok {
			continue
		}
		published, ok := section.Published()
		if !ok {
			continue
		}
		view.Sections = append(view.Sections, PresentationSectionView{
			Kind:        string(kind),
			Title:       published.Content.Title,
			Body:        published.Content.Body,
			Items:       toItemViews(published.Content.Items),
			Version:     published.Number,
			PublishedAt: published.PublishedAt,
		})
		fmt.Fprintf(fingerprint, "%s:%s:%d;", kind, section.ID(), published.Number)
	}
	view.Revision = hex.EncodeToString(fingerprint.Sum(nil))[:32]
	return view, nil
}
