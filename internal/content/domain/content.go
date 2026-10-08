package domain

import (
	"net/url"
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
	maxSources      = 10
	maxSourceLabel  = 200
	maxSourceURL    = 2048
	maxAttachments  = 10
)

type Layout string

const (
	LayoutList     Layout = "list"
	LayoutFacts    Layout = "facts"
	LayoutTimeline Layout = "timeline"
)

var layouts = []Layout{LayoutList, LayoutFacts, LayoutTimeline}

func ParseLayout(s string) (Layout, error) {
	if l := Layout(strings.TrimSpace(s)); slices.Contains(layouts, l) {
		return l, nil
	}
	return "", kernel.Invalid("layout must be one of list, facts or timeline")
}

type Source struct {
	Label string
	URL   string
}

type Item struct {
	Heading     string
	Detail      string
	Semester    int
	Sources     []Source
	Attachments []kernel.ID
}

func (i Item) equal(other Item) bool {
	return i.Heading == other.Heading &&
		i.Detail == other.Detail &&
		i.Semester == other.Semester &&
		slices.Equal(i.Sources, other.Sources) &&
		slices.Equal(i.Attachments, other.Attachments)
}

func (i Item) clone() Item {
	i.Sources = slices.Clone(i.Sources)
	i.Attachments = slices.Clone(i.Attachments)
	return i
}

func normalizeSources(index int, sources []Source) ([]Source, error) {
	if len(sources) > maxSources {
		return nil, kernel.Invalid("item %d: at most %d sources", index, maxSources)
	}
	normalized := make([]Source, 0, len(sources))
	for j, src := range sources {
		src.Label = strings.TrimSpace(src.Label)
		src.URL = strings.TrimSpace(src.URL)
		if utf8.RuneCountInString(src.Label) > maxSourceLabel {
			return nil, kernel.Invalid("item %d, source %d: label exceeds %d characters", index, j+1, maxSourceLabel)
		}
		u, err := url.Parse(src.URL)
		if err != nil || len(src.URL) > maxSourceURL || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, kernel.Invalid("item %d, source %d: URL must be an http or https link", index, j+1)
		}
		normalized = append(normalized, src)
	}
	return normalized, nil
}

func normalizeAttachments(index int, ids []kernel.ID) ([]kernel.ID, error) {
	if len(ids) > maxAttachments {
		return nil, kernel.Invalid("item %d: at most %d attachments", index, maxAttachments)
	}
	normalized := make([]kernel.ID, 0, len(ids))
	for _, id := range ids {
		id = kernel.ID(strings.TrimSpace(id.String()))
		if id == "" {
			return nil, kernel.Invalid("item %d: attachment id is required", index)
		}
		if slices.Contains(normalized, id) {
			return nil, kernel.Invalid("item %d: attachment %s is listed twice", index, id)
		}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

type Content struct {
	Title  string
	Body   string
	Layout Layout
	Items  []Item
}

func NewContent(title, body string, layout Layout, items []Item) (Content, error) {
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
	if !slices.Contains(layouts, layout) {
		return Content{}, kernel.Invalid("layout must be one of list, facts or timeline")
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
		sources, err := normalizeSources(i+1, item.Sources)
		if err != nil {
			return Content{}, err
		}
		attachments, err := normalizeAttachments(i+1, item.Attachments)
		if err != nil {
			return Content{}, err
		}
		item.Sources, item.Attachments = sources, attachments
		normalized = append(normalized, item)
	}
	return Content{Title: title, Body: body, Layout: layout, Items: normalized}, nil
}

func (c Content) Equal(other Content) bool {
	return c.Title == other.Title && c.Body == other.Body && c.Layout == other.Layout && slices.EqualFunc(c.Items, other.Items, Item.equal)
}

func (c Content) clone() Content {
	items := make([]Item, len(c.Items))
	for i, item := range c.Items {
		items[i] = item.clone()
	}
	c.Items = items
	return c
}

func (c Content) AttachmentIDs() []kernel.ID {
	var ids []kernel.ID
	for _, item := range c.Items {
		for _, id := range item.Attachments {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	return ids
}
