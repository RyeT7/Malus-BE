package domain

import (
	"slices"
	"strings"
	"unicode/utf8"

	"malus-be/internal/kernel"
)

const (
	maxTitleRunes   = 200
	maxBodyRunes    = 20000
	maxItems        = 50
	maxHeadingRunes = 200
	maxDetailRunes  = 5000
	maxSemester     = 12

	strengthsAndWeaknessesCount = 3
	minWorkplanSemesters        = 2
)

type Item struct {
	Heading  string
	Detail   string
	Semester int
}

type Content struct {
	Title string
	Body  string
	Items []Item
}

func NewContent(title, body string, items []Item) (Content, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Content{}, kernel.Invalid("title is required")
	}
	if utf8.RuneCountInString(title) > maxTitleRunes {
		return Content{}, kernel.Invalid("title exceeds %d characters", maxTitleRunes)
	}
	if utf8.RuneCountInString(body) > maxBodyRunes {
		return Content{}, kernel.Invalid("body exceeds %d characters", maxBodyRunes)
	}
	if len(items) > maxItems {
		return Content{}, kernel.Invalid("a section can have at most %d items", maxItems)
	}

	normalized := make([]Item, 0, len(items))
	for i, item := range items {
		item.Heading = strings.TrimSpace(item.Heading)
		item.Detail = strings.TrimSpace(item.Detail)
		switch {
		case item.Heading == "":
			return Content{}, kernel.Invalid("item %d: heading is required", i+1)
		case utf8.RuneCountInString(item.Heading) > maxHeadingRunes:
			return Content{}, kernel.Invalid("item %d: heading exceeds %d characters", i+1, maxHeadingRunes)
		case utf8.RuneCountInString(item.Detail) > maxDetailRunes:
			return Content{}, kernel.Invalid("item %d: detail exceeds %d characters", i+1, maxDetailRunes)
		case item.Semester < 0 || item.Semester > maxSemester:
			return Content{}, kernel.Invalid("item %d: semester must be between 1 and %d", i+1, maxSemester)
		}
		normalized = append(normalized, item)
	}
	return Content{Title: title, Body: body, Items: normalized}, nil
}

func (c Content) Equal(other Content) bool {
	return c.Title == other.Title && c.Body == other.Body && slices.Equal(c.Items, other.Items)
}

func (c Content) clone() Content {
	c.Items = slices.Clone(c.Items)
	return c
}

func (k Kind) checkDraft(c Content) error {
	if k == KindWorkplan {
		return nil
	}
	for i, item := range c.Items {
		if item.Semester != 0 {
			return kernel.Invalid("item %d: only workplan items have a semester", i+1)
		}
	}
	return nil
}

func (k Kind) checkPublishable(c Content) error {
	switch k {
	case KindStrengths, KindWeaknesses:
		if len(c.Items) != strengthsAndWeaknessesCount {
			return kernel.Conflict("%s needs exactly %d items to be published, has %d", k, strengthsAndWeaknessesCount, len(c.Items))
		}
	case KindWorkplan:
		semesters := make(map[int]bool)
		for i, item := range c.Items {
			if item.Semester == 0 {
				return kernel.Conflict("workplan item %d needs a semester to be published", i+1)
			}
			semesters[item.Semester] = true
		}
		if len(semesters) < minWorkplanSemesters {
			return kernel.Conflict("workplan needs items in at least %d semesters to be published, has %d", minWorkplanSemesters, len(semesters))
		}
	case KindBiodata, KindInnovations, KindProposedChanges:
		if len(c.Items) == 0 {
			return kernel.Conflict("%s needs at least one item to be published", k)
		}
	case KindWhyMe:
		if strings.TrimSpace(c.Body) == "" {
			return kernel.Conflict("why_me needs a body to be published")
		}
	}
	return nil
}
