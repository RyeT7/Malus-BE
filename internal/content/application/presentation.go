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
	ID          string
	Title       string
	Body        string
	Layout      string
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
	var published []domain.Content
	for _, section := range sections {
		if v, ok := section.Published(); ok {
			published = append(published, v.Content)
		}
	}
	files, err := s.attachmentIndex(ctx, published...)
	if err != nil {
		return PresentationView{}, err
	}

	view := PresentationView{Sections: make([]PresentationSectionView, 0, len(sections))}
	fingerprint := sha256.New()
	for _, section := range sections {
		published, ok := section.Published()
		if !ok {
			continue
		}
		view.Sections = append(view.Sections, PresentationSectionView{
			ID:          section.ID().String(),
			Title:       published.Content.Title,
			Body:        published.Content.Body,
			Layout:      string(published.Content.Layout),
			Items:       toItemViews(published.Content.Items, files),
			Version:     published.Number,
			PublishedAt: published.PublishedAt,
		})
		fmt.Fprintf(fingerprint, "%s:%d;", section.ID(), published.Number)
	}
	view.Revision = hex.EncodeToString(fingerprint.Sum(nil))[:32]
	return view, nil
}
