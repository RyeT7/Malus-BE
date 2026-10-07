package application

import (
	"time"

	"malus-be/internal/content/domain"
	"malus-be/internal/kernel"
)

type SourceView struct {
	Label string
	URL   string
}

type ItemView struct {
	Heading     string
	Detail      string
	Semester    int
	Sources     []SourceView
	Attachments []AttachmentView
}

type attachmentIndex = map[kernel.ID]AttachmentView

type ContentView struct {
	Title string
	Body  string
	Items []ItemView
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
	Revision              int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func toItemViews(items []domain.Item, files attachmentIndex) []ItemView {
	views := make([]ItemView, len(items))
	for i, item := range items {
		sources := make([]SourceView, len(item.Sources))
		for j, src := range item.Sources {
			sources[j] = SourceView{Label: src.Label, URL: src.URL}
		}
		attachments := make([]AttachmentView, 0, len(item.Attachments))
		for _, id := range item.Attachments {
			if a, ok := files[id]; ok {
				attachments = append(attachments, a)
			}
		}
		views[i] = ItemView{Heading: item.Heading, Detail: item.Detail, Semester: item.Semester, Sources: sources, Attachments: attachments}
	}
	return views
}

func toContentView(c domain.Content, files attachmentIndex) ContentView {
	return ContentView{Title: c.Title, Body: c.Body, Items: toItemViews(c.Items, files)}
}

func toVersionView(v domain.Version, files attachmentIndex) VersionView {
	return VersionView{Number: v.Number, Content: toContentView(v.Content, files), PublishedAt: v.PublishedAt}
}

func toSectionView(s *domain.Section, files attachmentIndex) SectionView {
	view := SectionView{
		ID:                    s.ID().String(),
		Kind:                  string(s.Kind()),
		Draft:                 toContentView(s.Draft(), files),
		HasUnpublishedChanges: s.HasUnpublishedChanges(),
		Revision:              s.Revision(),
		CreatedAt:             s.CreatedAt(),
		UpdatedAt:             s.UpdatedAt(),
	}
	if v, ok := s.Published(); ok {
		pv := toVersionView(v, files)
		view.Published = &pv
	}
	return view
}
