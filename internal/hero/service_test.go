package hero

import (
	"net/url"
	"slices"
	"testing"
)

var catalog = []Hero{
	{UID: "miya", MLID: 1, Name: "Miya", Roles: []string{"marksman"}, Lanes: []string{"gold"}},
	{UID: "balmond", MLID: 2, Name: "Balmond", Roles: []string{"fighter"}, Lanes: []string{"jungle", "exp"}},
	{UID: "alice", MLID: 4, Name: "Alice", Roles: []string{"mage", "tank"}, Lanes: []string{"exp"}},
	{UID: "tigreal", MLID: 6, Name: "Tigreal", Roles: []string{"tank"}, Lanes: []string{"roam"}},
	{UID: "alucard", MLID: 9, Name: "Alucard", Roles: []string{"fighter", "assassin"}, Lanes: []string{"jungle", "exp"}},
}

func uids(heroes []Hero) []string {
	out := []string{}
	for _, h := range heroes {
		out = append(out, h.UID)
	}
	return out
}

func TestParseListQuery(t *testing.T) {
	cases := []struct {
		query            string
		wantPage, wantSz int
	}{
		{"", 1, 10},
		{"page=3&size=25", 3, 25},
		{"page=0&size=0", 1, 10},
		{"page=-2&size=-5", 1, 10},
		{"page=abc&size=abc", 1, 10},
		{"size=100", 1, 100},
		{"size=500", 1, 100},
	}
	for _, c := range cases {
		v, _ := url.ParseQuery(c.query)
		q := ParseListQuery(v)
		if q.Page != c.wantPage || q.Size != c.wantSz {
			t.Errorf("%q: page %d size %d, want %d %d", c.query, q.Page, q.Size, c.wantPage, c.wantSz)
		}
	}
}

func TestList(t *testing.T) {
	cases := []struct {
		query                    string
		want                     []string
		page, size, total, pages int
	}{
		{"", []string{"miya", "balmond", "alice", "tigreal", "alucard"}, 1, 10, 5, 1},
		{"search=ali", []string{"alice"}, 1, 10, 1, 1},
		{"search=%20ALU%20", []string{"alucard"}, 1, 10, 1, 1},
		{"role=%20TANK", []string{"alice", "tigreal"}, 1, 10, 2, 1},
		{"lane=Exp", []string{"balmond", "alice", "alucard"}, 1, 10, 3, 1},
		{"role=fighter&lane=jungle&search=car", []string{"alucard"}, 1, 10, 1, 1},
		{"lane=exp&page=2&size=2", []string{"alucard"}, 2, 2, 3, 2},
		{"page=9", []string{}, 9, 10, 5, 1},
		{"search=zzz", []string{}, 1, 10, 0, 0},
		{"role=", []string{"miya", "balmond", "alice", "tigreal", "alucard"}, 1, 10, 5, 1},
	}
	for _, c := range cases {
		v, _ := url.ParseQuery(c.query)
		got := List(catalog, ParseListQuery(v))
		if ids := uids(got.Items); !slices.Equal(ids, c.want) ||
			got.Items == nil || got.Page != c.page || got.Size != c.size || got.Total != c.total || got.Pages != c.pages {
			t.Errorf("%q: items %v page %d size %d total %d pages %d (nil items: %t), want %v %d %d %d %d",
				c.query, ids, got.Page, got.Size, got.Total, got.Pages, got.Items == nil, c.want, c.page, c.size, c.total, c.pages)
		}
	}
}
