package hero

import (
	"net/url"
	"strconv"
	"strings"
)

// ListQuery is a hero list request: filters and a page.
type ListQuery struct {
	Search, Role, Lane string
	Page, Size         int
}

const (
	defaultSize = 10
	maxSize     = 100
)

// ParseListQuery reads the list parameters. Filters are trimmed and
// lowercased; a page or size that is not a positive integer takes its
// default, and a size above the maximum is clamped to it.
func ParseListQuery(v url.Values) ListQuery {
	return ListQuery{
		Search: strings.ToLower(strings.TrimSpace(v.Get("search"))),
		Role:   strings.ToLower(strings.TrimSpace(v.Get("role"))),
		Lane:   strings.ToLower(strings.TrimSpace(v.Get("lane"))),
		Page:   positiveOr(v.Get("page"), 1),
		Size:   min(positiveOr(v.Get("size"), defaultSize), maxSize),
	}
}

func positiveOr(raw string, fallback int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

// Page is one page of a filtered hero list.
type Page struct {
	Items []Hero `json:"items"`
	Page  int    `json:"page"`
	Size  int    `json:"size"`
	Total int    `json:"total"`
	Pages int    `json:"pages"`
}

// List filters heroes, which arrive in Moonton ID order, and returns the
// requested page of the result.
func List(heroes []Hero, q ListQuery) Page {
	matched := []Hero{}
	for _, h := range heroes {
		if q.Search != "" && !strings.Contains(strings.ToLower(h.Name), q.Search) {
			continue
		}
		if q.Role != "" && !containsFold(h.Roles, q.Role) {
			continue
		}
		if q.Lane != "" && !containsFold(h.Lanes, q.Lane) {
			continue
		}
		matched = append(matched, h)
	}
	total := len(matched)
	start := min((q.Page-1)*q.Size, total)
	end := min(start+q.Size, total)
	return Page{
		Items: matched[start:end],
		Page:  q.Page,
		Size:  q.Size,
		Total: total,
		Pages: (total + q.Size - 1) / q.Size,
	}
}

func containsFold(values []string, want string) bool {
	for _, v := range values {
		if strings.ToLower(v) == want {
			return true
		}
	}
	return false
}
