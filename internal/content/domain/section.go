package domain

import (
	"context"
	"slices"
	"time"

	"malus-be/internal/kernel"
)

type Kind string

const (
	KindBiodata         Kind = "biodata"
	KindStrengths       Kind = "strengths"
	KindWeaknesses      Kind = "weaknesses"
	KindWorkplan        Kind = "workplan"
	KindInnovations     Kind = "innovations"
	KindProposedChanges Kind = "proposed_changes"
	KindWhyMe           Kind = "why_me"
)

var kindOrder = []Kind{
	KindBiodata,
	KindStrengths,
	KindWeaknesses,
	KindWorkplan,
	KindInnovations,
	KindProposedChanges,
	KindWhyMe,
}

func Kinds() []Kind {
	return slices.Clone(kindOrder)
}

func ParseKind(s string) (Kind, error) {
	if k := Kind(s); slices.Contains(kindOrder, k) {
		return k, nil
	}
	return "", kernel.Invalid("unknown section kind %q", s)
}

type Version struct {
	Number      int
	Content     Content
	PublishedAt time.Time
}

type Section struct {
	kernel.AggregateRoot
	id        kernel.ID
	kind      Kind
	draft     Content
	versions  []Version
	createdAt time.Time
	updatedAt time.Time
	revision  int64
}

func NewSection(kind Kind, draft Content, now time.Time) (*Section, error) {
	if err := kind.checkDraft(draft); err != nil {
		return nil, err
	}
	s := &Section{
		id:        kernel.NewID(),
		kind:      kind,
		draft:     draft.clone(),
		createdAt: now,
		updatedAt: now,
	}
	s.Record(SectionCreated{EventBase: kernel.NewEventBase(s.id, now), Kind: kind})
	return s, nil
}

func (s *Section) ID() kernel.ID        { return s.id }
func (s *Section) Kind() Kind           { return s.kind }
func (s *Section) Draft() Content       { return s.draft.clone() }
func (s *Section) CreatedAt() time.Time { return s.createdAt }
func (s *Section) UpdatedAt() time.Time { return s.updatedAt }
func (s *Section) Revision() int64      { return s.revision }

func (s *Section) Versions() []Version {
	versions := make([]Version, len(s.versions))
	for i, v := range s.versions {
		v.Content = v.Content.clone()
		versions[i] = v
	}
	return versions
}

func (s *Section) Published() (Version, bool) {
	if len(s.versions) == 0 {
		return Version{}, false
	}
	v := s.versions[len(s.versions)-1]
	v.Content = v.Content.clone()
	return v, true
}

func (s *Section) HasUnpublishedChanges() bool {
	p, ok := s.Published()
	return !ok || !p.Content.Equal(s.draft)
}

func (s *Section) EditDraft(c Content, now time.Time) error {
	if err := s.kind.checkDraft(c); err != nil {
		return err
	}
	if c.Equal(s.draft) {
		return nil
	}
	s.draft = c.clone()
	s.updatedAt = now
	s.Record(DraftEdited{EventBase: kernel.NewEventBase(s.id, now)})
	return nil
}

func (s *Section) Publish(now time.Time) (Version, error) {
	if !s.HasUnpublishedChanges() {
		return Version{}, kernel.Conflict("section %s has no unpublished changes", s.id)
	}
	if err := s.kind.checkPublishable(s.draft); err != nil {
		return Version{}, err
	}
	v := Version{Number: len(s.versions) + 1, Content: s.draft.clone(), PublishedAt: now}
	s.versions = append(s.versions, v)
	s.updatedAt = now
	s.Record(SectionPublished{EventBase: kernel.NewEventBase(s.id, now), Version: v.Number})
	return v, nil
}

func (s *Section) Rollback(number int, now time.Time) (Version, error) {
	if number < 1 || number > len(s.versions) {
		return Version{}, kernel.NotFound("section %s has no version %d", s.id, number)
	}
	previous := s.draft
	s.draft = s.versions[number-1].Content.clone()
	v, err := s.Publish(now)
	if err != nil {
		s.draft = previous
		return Version{}, err
	}
	s.Record(SectionRolledBack{EventBase: kernel.NewEventBase(s.id, now), FromVersion: number, Version: v.Number})
	return v, nil
}

type Snapshot struct {
	ID        kernel.ID
	Kind      Kind
	Draft     Content
	Versions  []Version
	CreatedAt time.Time
	UpdatedAt time.Time
	Revision  int64
}

func (s *Section) Snapshot() Snapshot {
	return Snapshot{
		ID:        s.id,
		Kind:      s.kind,
		Draft:     s.draft.clone(),
		Versions:  s.Versions(),
		CreatedAt: s.createdAt,
		UpdatedAt: s.updatedAt,
		Revision:  s.revision,
	}
}

func Restore(snap Snapshot) *Section {
	return &Section{
		id:        snap.ID,
		kind:      snap.Kind,
		draft:     snap.Draft.clone(),
		versions:  restoreVersions(snap.Versions),
		createdAt: snap.CreatedAt,
		updatedAt: snap.UpdatedAt,
		revision:  snap.Revision,
	}
}

func restoreVersions(versions []Version) []Version {
	restored := make([]Version, len(versions))
	for i, v := range versions {
		v.Content = v.Content.clone()
		restored[i] = v
	}
	return restored
}

type Repository interface {
	Get(ctx context.Context, id kernel.ID) (*Section, error)
	FindByKind(ctx context.Context, kind Kind) (*Section, error)
	List(ctx context.Context) ([]*Section, error)
	Save(ctx context.Context, s *Section) error
}
