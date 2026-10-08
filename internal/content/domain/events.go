package domain

import "malus-be/internal/kernel"

type SectionCreated struct {
	kernel.EventBase
	Title string `json:"title"`
}

func (SectionCreated) EventType() string { return "content.section.created" }

type DraftEdited struct {
	kernel.EventBase
}

func (DraftEdited) EventType() string { return "content.section.draft_edited" }

type SectionPublished struct {
	kernel.EventBase
	Version int `json:"version"`
}

func (SectionPublished) EventType() string { return "content.section.published" }

type SectionRolledBack struct {
	kernel.EventBase
	FromVersion int `json:"fromVersion"`
	Version     int `json:"version"`
}

func (SectionRolledBack) EventType() string { return "content.section.rolled_back" }

type SectionDeleted struct {
	kernel.EventBase
}

func (SectionDeleted) EventType() string { return "content.section.deleted" }
