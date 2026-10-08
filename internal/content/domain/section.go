package domain

import (
	"context"
	"time"

	"malus-be/internal/kernel"
)

type Version struct {
	Number      int
	Content     Content
	PublishedAt time.Time
}

type Section struct {
	kernel.AggregateRoot
	id        kernel.ID
	draft     Content
	versions  []Version
	createdAt time.Time
	updatedAt time.Time
	revision  int64
}

func NewSection(draft Content, now time.Time) *Section {
	s := &Section{
		id:        kernel.NewID(),
		draft:     draft.clone(),
		createdAt: now,
		updatedAt: now,
	}
	s.Record(SectionCreated{EventBase: kernel.NewEventBase(s.id, now), Title: draft.Title})
	return s
}

func (s *Section) ID() kernel.ID        { return s.id }
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

func (s *Section) EditDraft(c Content, now time.Time) {
	if c.Equal(s.draft) {
		return
	}
	s.draft = c.clone()
	s.updatedAt = now
	s.Record(DraftEdited{EventBase: kernel.NewEventBase(s.id, now)})
}

func (s *Section) Publish(now time.Time) (Version, error) {
	if !s.HasUnpublishedChanges() {
		return Version{}, kernel.Conflict("section %s has no unpublished changes", s.id)
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

func (s *Section) Delete(now time.Time) {
	s.Record(SectionDeleted{EventBase: kernel.NewEventBase(s.id, now)})
}

type Snapshot struct {
	ID        kernel.ID
	Draft     Content
	Versions  []Version
	CreatedAt time.Time
	UpdatedAt time.Time
	Revision  int64
}

func (s *Section) Snapshot() Snapshot {
	return Snapshot{
		ID:        s.id,
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
	List(ctx context.Context) ([]*Section, error)
	Save(ctx context.Context, s *Section) error
	Delete(ctx context.Context, s *Section) error
	Reorder(ctx context.Context, ids []kernel.ID) error
}
