package application

import (
	"time"

	"malus-be/internal/content/domain"
)

type ContentView struct {
	Title string
	Body  string
}

type VersionView struct {
	Number      int
	Content     ContentView
	PublishedAt time.Time
}

type SectionView struct {
	ID                    string
	Kind                  string
	Draft                 ContentView
	Published             *VersionView
	HasUnpublishedChanges bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func toContentView(c domain.Content) ContentView {
	return ContentView{Title: c.Title, Body: c.Body}
}

func toVersionView(v domain.Version) VersionView {
	return VersionView{Number: v.Number, Content: toContentView(v.Content), PublishedAt: v.PublishedAt}
}

func toSectionView(s *domain.Section) SectionView {
	view := SectionView{
		ID:                    s.ID().String(),
		Kind:                  string(s.Kind()),
		Draft:                 toContentView(s.Draft()),
		HasUnpublishedChanges: s.HasUnpublishedChanges(),
		CreatedAt:             s.CreatedAt(),
		UpdatedAt:             s.UpdatedAt(),
	}
	if v, ok := s.Published(); ok {
		pv := toVersionView(v)
		view.Published = &pv
	}
	return view
}
