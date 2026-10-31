// Package stats serves statistics measured by Moonton.
package stats

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/yeremi777/mlbb-collector/internal/hero"
)

// Query names the Rank tier and Window of a Board.
type Query struct {
	RankTier   string
	WindowDays int
}

const (
	defaultRankTier   = "all"
	defaultWindowDays = 7
)

// ParseQuery reads rank and window. The Rank tier is trimmed and lowercased;
// a blank one is all, and a Window that is not an integer is 7.
func ParseQuery(v url.Values) Query {
	q := Query{RankTier: strings.ToLower(strings.TrimSpace(v.Get("rank"))), WindowDays: defaultWindowDays}
	if q.RankTier == "" {
		q.RankTier = defaultRankTier
	}
	if n, err := strconv.Atoi(strings.TrimSpace(v.Get("window"))); err == nil {
		q.WindowDays = n
	}
	return q
}

// Board is every hero's standing at the newest Snapshot date of one Rank tier
// and Window. SnapshotDate is nil when that pair has no Snapshot, and Patch
// when no Patch covers that date.
type Board struct {
	RankTier     string     `json:"rankTier"`
	WindowDays   int        `json:"windowDays"`
	SnapshotDate *string    `json:"snapshotDate"`
	Patch        *Patch     `json:"patch"`
	Heroes       []Standing `json:"heroes"`
}

// Patch is the Patch a Snapshot date falls in.
type Patch struct {
	Version            string `json:"version"`
	DaysSince          int    `json:"daysSince"`
	WindowCrossesPatch bool   `json:"windowCrossesPatch"`
}

// Standing is one hero's row on a Board, with the hero in full.
type Standing struct {
	Hero            hero.Hero `json:"hero"`
	WinRate         float64   `json:"winRate"`
	AppearanceShare float64   `json:"appearanceShare"`
	BanRate         float64   `json:"banRate"`
	Change          *Change   `json:"change"`
	Position        Position  `json:"position"`
}

// Change is a Standing's movement since the hero's previous Snapshot date.
type Change struct {
	Since           string  `json:"since"`
	WinRate         float64 `json:"winRate"`
	AppearanceShare float64 `json:"appearanceShare"`
	BanRate         float64 `json:"banRate"`
}

// Position is a Standing's place by win rate and by Appearance share.
type Position struct {
	WinRate    int `json:"winRate"`
	Appearance int `json:"appearance"`
}
