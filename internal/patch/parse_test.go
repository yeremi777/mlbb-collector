package patch_test

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yeremi777/mlbb-collector/internal/patch"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func parseFixture(t *testing.T) []patch.Patch {
	t.Helper()
	f, err := os.Open("testdata/patches.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := patch.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestParseFixtureReadsTheWholeCalendar(t *testing.T) {
	got := parseFixture(t)
	if len(got) != 113 {
		t.Fatalf("got %d patches, want 113", len(got))
	}
	if first := got[0]; first.Version != "2.1.95" || !first.ReleaseDate.Equal(date(2026, time.August, 4)) {
		t.Errorf("newest = %s %s, want 2.1.95 2026-08-04", first.Version, first.ReleaseDate)
	}
	if last := got[112]; last.Version != "1.3.90" || !last.ReleaseDate.Equal(date(2019, time.June, 27)) {
		t.Errorf("oldest = %s %s, want 1.3.90 2019-06-27", last.Version, last.ReleaseDate)
	}
	versions := map[string][]time.Time{}
	for i, p := range got {
		if i > 0 && !p.ReleaseDate.Before(got[i-1].ReleaseDate) {
			t.Errorf("%s (%s) is not older than %s (%s)", p.Version, p.ReleaseDate, got[i-1].Version, got[i-1].ReleaseDate)
		}
		versions[p.Version] = append(versions[p.Version], p.ReleaseDate)
		for _, h := range p.Highlights {
			if h == "" || h != strings.Join(strings.Fields(h), " ") {
				t.Errorf("%s: highlight %q is not whitespace-collapsed", p.Version, h)
			}
		}
	}
	if len(versions) != 112 {
		t.Errorf("%d distinct versions, want 112", len(versions))
	}
	if want := []time.Time{date(2022, time.December, 28), date(2022, time.December, 23)}; !reflect.DeepEqual(versions["1.7.44B"], want) {
		t.Errorf("1.7.44B released %v, want %v", versions["1.7.44B"], want)
	}
	for _, v := range []string{"2.1.67a", "1.9.42", "1.9.42B", "1.9.42E", "1.9.64C"} {
		if len(versions[v]) != 1 {
			t.Errorf("version %s appears %d times, want once", v, len(versions[v]))
		}
	}
	if h := got[0].Highlights; len(h) != 5 || !strings.HasPrefix(h[0], "Revamped Hero Kaja, Nazar King") || h[1] != "Hero Adjustments" || h[4] != "Mode Adjustments" {
		t.Errorf("2.1.95 highlights = %q", h)
	}
}

const header = `<tr><th>Patch</th><th>Release Date</th><th>Release Highlights</th></tr>`

func table(rows ...string) string {
	return "<table>" + header + strings.Join(rows, "") + "</table>"
}

func TestParseRows(t *testing.T) {
	for name, tc := range map[string]struct {
		row  string
		want []string
	}{
		"list items": {`<tr><td><a>Patch 2.1.90</a></td><td>July 1, 2026</td><td><ul><li>Hero  Adjustments</li><li>New <b>Hero</b>:
 Zetian</li></ul></td></tr>`, []string{"Hero Adjustments", "New Hero: Zetian"}},
		"nested items fold": {`<tr><td>Patch 2.1.90</td><td>July 1, 2026</td><td><ul><li>Revamps<ul><li>Kaja</li></ul></li><li>Bug Fixes</li></ul></td></tr>`, []string{"Revamps Kaja", "Bug Fixes"}},
		"plain text cell":   {`<tr><td>Patch 2.1.90</td><td>July 1, 2026</td><td> Hero   adjustments </td></tr>`, []string{"Hero adjustments"}},
		"empty cell":        {`<tr><td>Patch 2.1.90</td><td>July 1, 2026</td><td></td></tr>`, []string{}},
		"missing cell":      {`<tr><td>Patch 2.1.90</td><td>July 1, 2026</td></tr>`, []string{}},
		"display marker":    {`<tr><td>Patch 2.1.90 (latest)</td><td>July 1, 2026</td></tr>`, []string{}},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := patch.Parse(strings.NewReader(table(tc.row)))
			if err != nil {
				t.Fatal(err)
			}
			want := []patch.Patch{{ReleaseDate: date(2026, time.July, 1), Version: "2.1.90", Highlights: tc.want}}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got  %#v\nwant %#v", got, want)
			}
		})
	}
}

func TestParseSkipsRowsThatAreNotPatches(t *testing.T) {
	got, err := patch.Parse(strings.NewReader(table(
		`<tr><td>Patch 2.1.90</td><td>July 1, 2026</td></tr>`,
		`<tr><td>Patch notes</td><td>July 2, 2026</td></tr>`,
		`<tr><td>Patch 2.1.91</td><td>TBA</td></tr>`,
		`<tr><td>Season 38</td><td>July 3, 2026</td></tr>`,
	)))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Version != "2.1.90" {
		t.Errorf("got %+v, want only 2.1.90", got)
	}
}

func TestParseRejectsAPageWithoutPatches(t *testing.T) {
	for name, tc := range map[string]struct {
		body      string
		retryable bool
	}{
		"tables without patch rows": {table(`<tr><td>Season 38</td><td>July 3, 2026</td></tr>`), false},
		"no tables":                 {`<html><body>Rate limited</body></html>`, true},
		"empty body":                {``, true},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := patch.Parse(strings.NewReader(tc.body))
			var pe *patch.ParseError
			if !errors.As(err, &pe) || pe.Retryable() != tc.retryable {
				t.Errorf("err = %v, want a ParseError with Retryable() %v", err, tc.retryable)
			}
		})
	}
}

func TestParseRejectsARepeatedReleaseDate(t *testing.T) {
	_, err := patch.Parse(strings.NewReader(table(
		`<tr><td>Patch 2.1.90</td><td>July 1, 2026</td></tr>`,
		`<tr><td>Patch 2.1.90a</td><td>July 1, 2026</td></tr>`,
	)))
	var pe *patch.ParseError
	if err == nil || errors.As(err, &pe) && pe.Retryable() {
		t.Errorf("err = %v, want a non-retryable error", err)
	}
}
