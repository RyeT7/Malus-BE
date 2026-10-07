package application

import (
	"context"
	"errors"

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

type CreateSection struct {
	Kind  string
	Title string
	Body  string
	Items []ItemInput
}

func (s *Service) CreateSection(ctx context.Context, cmd CreateSection) (SectionView, error) {
	kind, err := domain.ParseKind(cmd.Kind)
	if err != nil {
		return SectionView{}, err
	}
	content, err := domain.NewContent(cmd.Title, cmd.Body, toItems(cmd.Items))
	if err != nil {
		return SectionView{}, err
	}
	if err := s.checkAttachments(ctx, content); err != nil {
		return SectionView{}, err
	}
	if _, err := s.repo.FindByKind(ctx, kind); err == nil {
		return SectionView{}, kernel.Conflict("section of kind %q already exists", kind)
	} else if !errors.Is(err, kernel.ErrNotFound) {
		return SectionView{}, err
	}

	section, err := domain.NewSection(kind, content, s.now())
	if err != nil {
		return SectionView{}, err
	}
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
	Items            []ItemInput
}

func (s *Service) EditDraft(ctx context.Context, cmd EditDraft) (SectionView, error) {
	content, err := domain.NewContent(cmd.Title, cmd.Body, toItems(cmd.Items))
	if err != nil {
		return SectionView{}, err
	}
	if err := s.checkAttachments(ctx, content); err != nil {
		return SectionView{}, err
	}
	return s.change(ctx, cmd.ID, cmd.ExpectedRevision, func(section *domain.Section) error {
		return section.EditDraft(content, s.now())
	})
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
