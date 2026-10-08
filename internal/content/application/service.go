package application

import (
	"context"
	"errors"
	"strings"

	"malus-be/internal/content/domain"
	"malus-be/internal/kernel"
)

type Service struct {
	repo        domain.Repository
	attachments domain.AttachmentRepository
	blobs       BlobStore
	events      kernel.EventPublisher
	now         kernel.Clock
}

func NewService(repo domain.Repository, attachments domain.AttachmentRepository, blobs BlobStore, events kernel.EventPublisher, now kernel.Clock) *Service {
	return &Service{repo: repo, attachments: attachments, blobs: blobs, events: events, now: now}
}

type SourceInput struct {
	Label string
	URL   string
}

type ItemInput struct {
	Heading     string
	Detail      string
	Semester    int
	Sources     []SourceInput
	Attachments []string
}

func toItems(inputs []ItemInput) []domain.Item {
	items := make([]domain.Item, len(inputs))
	for i, in := range inputs {
		sources := make([]domain.Source, len(in.Sources))
		for j, src := range in.Sources {
			sources[j] = domain.Source{Label: src.Label, URL: src.URL}
		}
		attachments := make([]kernel.ID, len(in.Attachments))
		for j, id := range in.Attachments {
			attachments[j] = kernel.ID(id)
		}
		items[i] = domain.Item{Heading: in.Heading, Detail: in.Detail, Semester: in.Semester, Sources: sources, Attachments: attachments}
	}
	return items
}

func (s *Service) newContent(ctx context.Context, title, body, layout string, items []ItemInput) (domain.Content, error) {
	l, err := domain.ParseLayout(layout)
	if err != nil {
		return domain.Content{}, err
	}
	content, err := domain.NewContent(title, body, l, toItems(items))
	if err != nil {
		return domain.Content{}, err
	}
	return content, s.checkAttachments(ctx, content)
}

type CreateSection struct {
	Title  string
	Body   string
	Layout string
	Items  []ItemInput
}

func (s *Service) CreateSection(ctx context.Context, cmd CreateSection) (SectionView, error) {
	content, err := s.newContent(ctx, cmd.Title, cmd.Body, cmd.Layout, cmd.Items)
	if err != nil {
		return SectionView{}, err
	}
	section := domain.NewSection(content, s.now())
	if err := s.commit(ctx, section); err != nil {
		return SectionView{}, err
	}
	return s.GetSection(ctx, section.ID())
}

type EditDraft struct {
	ID               kernel.ID
	ExpectedRevision int64
	Title            string
	Body             string
	Layout           string
	Items            []ItemInput
}

func (s *Service) EditDraft(ctx context.Context, cmd EditDraft) (SectionView, error) {
	content, err := s.newContent(ctx, cmd.Title, cmd.Body, cmd.Layout, cmd.Items)
	if err != nil {
		return SectionView{}, err
	}
	return s.change(ctx, cmd.ID, cmd.ExpectedRevision, func(section *domain.Section) error {
		section.EditDraft(content, s.now())
		return nil
	})
}

func (s *Service) DeleteSection(ctx context.Context, id kernel.ID, expectedRevision int64) error {
	section, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if section.Revision() != expectedRevision {
		return kernel.ErrPrecondition
	}
	section.Delete(s.now())
	if err := s.repo.Delete(ctx, section); err != nil {
		if errors.Is(err, kernel.ErrConcurrentUpdate) {
			return kernel.ErrPrecondition
		}
		return err
	}
	return s.events.Publish(ctx, section.PullEvents()...)
}

func (s *Service) ReorderSections(ctx context.Context, ids []string) ([]SectionView, error) {
	sections, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	existing := make(map[kernel.ID]bool, len(sections))
	for _, section := range sections {
		existing[section.ID()] = true
	}
	order := make([]kernel.ID, 0, len(ids))
	seen := make(map[kernel.ID]bool, len(ids))
	for _, raw := range ids {
		id := kernel.ID(strings.TrimSpace(raw))
		if !existing[id] {
			return nil, kernel.Invalid("section %s does not exist", id)
		}
		if seen[id] {
			return nil, kernel.Invalid("section %s is listed twice", id)
		}
		seen[id] = true
		order = append(order, id)
	}
	if len(order) != len(sections) {
		return nil, kernel.Invalid("the order must list all %d sections, got %d", len(sections), len(order))
	}
	if err := s.repo.Reorder(ctx, order); err != nil {
		return nil, err
	}
	return s.ListSections(ctx)
}

func (s *Service) Publish(ctx context.Context, id kernel.ID, expectedRevision int64) (SectionView, error) {
	return s.change(ctx, id, expectedRevision, func(section *domain.Section) error {
		_, err := section.Publish(s.now())
		return err
	})
}

func (s *Service) Rollback(ctx context.Context, id kernel.ID, version int, expectedRevision int64) (SectionView, error) {
	return s.change(ctx, id, expectedRevision, func(section *domain.Section) error {
		_, err := section.Rollback(version, s.now())
		return err
	})
}

func (s *Service) change(ctx context.Context, id kernel.ID, expectedRevision int64, apply func(*domain.Section) error) (SectionView, error) {
	section, err := s.repo.Get(ctx, id)
	if err != nil {
		return SectionView{}, err
	}
	if section.Revision() != expectedRevision {
		return SectionView{}, kernel.ErrPrecondition
	}
	if err := apply(section); err != nil {
		return SectionView{}, err
	}
	if err := s.commit(ctx, section); err != nil {
		if errors.Is(err, kernel.ErrConcurrentUpdate) {
			return SectionView{}, kernel.ErrPrecondition
		}
		return SectionView{}, err
	}
	return s.GetSection(ctx, id)
}

func (s *Service) GetSection(ctx context.Context, id kernel.ID) (SectionView, error) {
	section, err := s.repo.Get(ctx, id)
	if err != nil {
		return SectionView{}, err
	}
	files, err := s.attachmentIndex(ctx, sectionContents(section, false)...)
	if err != nil {
		return SectionView{}, err
	}
	return toSectionView(section, files), nil
}

func (s *Service) ListSections(ctx context.Context) ([]SectionView, error) {
	sections, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	var contents []domain.Content
	for _, section := range sections {
		contents = append(contents, sectionContents(section, false)...)
	}
	files, err := s.attachmentIndex(ctx, contents...)
	if err != nil {
		return nil, err
	}
	views := make([]SectionView, 0, len(sections))
	for _, section := range sections {
		views = append(views, toSectionView(section, files))
	}
	return views, nil
}

func (s *Service) ListVersions(ctx context.Context, id kernel.ID) ([]VersionView, error) {
	section, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	files, err := s.attachmentIndex(ctx, sectionContents(section, true)...)
	if err != nil {
		return nil, err
	}
	versions := section.Versions()
	views := make([]VersionView, 0, len(versions))
	for i := len(versions) - 1; i >= 0; i-- {
		views = append(views, toVersionView(versions[i], files))
	}
	return views, nil
}

func (s *Service) commit(ctx context.Context, section *domain.Section) error {
	if err := s.repo.Save(ctx, section); err != nil {
		return err
	}
	return s.events.Publish(ctx, section.PullEvents()...)
}
