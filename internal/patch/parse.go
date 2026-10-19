// Package patch reads the Patch calendar from Liquipedia's Portal:Patches and
// keeps raw.patches equal to it (ADR-0005).
package patch

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Patch is one game release.
type Patch struct {
	ReleaseDate time.Time
	Version     string
	Highlights  []string
}

// patchCell matches a first cell such as "Patch 2.1.67a (latest)".
var patchCell = regexp.MustCompile(`^Patch\s+([0-9][0-9A-Za-z.]*)`)

const releaseDateLayout = "January 2, 2006"

// ParseError reports a page with no Patch rows. A page with tables was
// redesigned and parses the same way again; one without tables is an error
// or throttle page, and a refetch may succeed.
type ParseError struct{ Tables int }

func (e *ParseError) Error() string {
	if e.Tables > 0 {
		return fmt.Sprintf("no Patch rows in %d tables: Portal:Patches changed", e.Tables)
	}
	return "no tables in the page"
}

// Retryable reports whether a refetch may succeed.
func (e *ParseError) Retryable() bool { return e.Tables == 0 }

// Parse reads every Patch row of the rendered page, in page order, newest
// first. A row is a Patch when its first cell names one and its second cell
// is a release date.
func Parse(r io.Reader) ([]Patch, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	var patches []Patch
	tables := 0
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			tables++
		}
		if n.Type == html.ElementNode && n.Data == "tr" {
			if p, ok := patchOf(n); ok {
				patches = append(patches, p)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if len(patches) == 0 {
		return nil, &ParseError{Tables: tables}
	}
	seen := make(map[time.Time]string, len(patches))
	for _, p := range patches {
		if v, ok := seen[p.ReleaseDate]; ok {
			return nil, fmt.Errorf("patches %s and %s share release date %s", v, p.Version, p.ReleaseDate.Format(time.DateOnly))
		}
		seen[p.ReleaseDate] = p.Version
	}
	return patches, nil
}

func patchOf(tr *html.Node) (Patch, bool) {
	var cells []*html.Node
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "td" {
			cells = append(cells, c)
		}
	}
	if len(cells) < 2 {
		return Patch{}, false
	}
	m := patchCell.FindStringSubmatch(collapse(textOf(cells[0])))
	if m == nil {
		return Patch{}, false
	}
	released, err := time.Parse(releaseDateLayout, collapse(textOf(cells[1])))
	if err != nil {
		return Patch{}, false
	}
	p := Patch{ReleaseDate: released, Version: m[1], Highlights: []string{}}
	if len(cells) > 2 {
		p.Highlights = highlightsOf(cells[2])
	}
	return p, true
}

// highlightsOf returns one entry per top-level list item, with nested items
// folded into their parent, or the cell's text as one entry when it has no
// list.
func highlightsOf(td *html.Node) []string {
	out := []string{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" {
			if s := collapse(textOf(n)); s != "" {
				out = append(out, s)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(td)
	if len(out) == 0 {
		if s := collapse(textOf(td)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// blockElements are separated from their neighbours' text by a space.
var blockElements = map[string]bool{"li": true, "ul": true, "ol": true, "p": true, "br": true, "div": true}

func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		block := n.Type == html.ElementNode && blockElements[n.Data]
		if block {
			b.WriteString(" ")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if block {
			b.WriteString(" ")
		}
	}
	walk(n)
	return b.String()
}
