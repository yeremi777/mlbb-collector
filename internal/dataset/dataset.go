// Package dataset reads the authored dataset from disk: heroes.json and the
// per-hero Counter and Synergy files. It owns the file layout and how a row
// decodes, and accepts the dataset only as a whole, once every file is valid
// and every reference resolves.
package dataset

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/yeremi777/mlbb-collector/internal/counter"
	"github.com/yeremi777/mlbb-collector/internal/hero"
	"github.com/yeremi777/mlbb-collector/internal/synergy"
)

// Dataset is the authored dataset read as one unit.
type Dataset struct {
	Heroes    []hero.Hero
	Counters  []counter.Counter
	Synergies []synergy.Synergy
}

// Load reads the dataset rooted at dir.
func Load(dir string) (Dataset, error) {
	heroes, err := loadHeroes(dir)
	if err != nil {
		return Dataset{}, err
	}
	known := make(map[string]bool, len(heroes))
	for _, h := range heroes {
		known[h.UID] = true
	}
	counters, err := loadMatchups(dir, counterFiles, known)
	if err != nil {
		return Dataset{}, err
	}
	synergies, err := loadMatchups(dir, synergyFiles, known)
	if err != nil {
		return Dataset{}, err
	}
	return Dataset{Heroes: heroes, Counters: counters, Synergies: synergies}, nil
}

const heroesFile = "heroes.json"

func loadHeroes(dir string) ([]hero.Hero, error) {
	rows, err := readArray(dir, heroesFile)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s: no heroes", heroesFile)
	}

	heroes := make([]hero.Hero, len(rows))
	byUID := map[string]int{}
	byMLID := map[int]int{}
	for i, row := range rows {
		h, err := decodeHero(row)
		if err == nil {
			err = h.Validate()
		}
		if err != nil {
			return nil, rowError(heroesFile, i, err)
		}
		if j, dup := byUID[h.UID]; dup {
			return nil, fmt.Errorf("%s[%d].uid: %q repeats %s[%d]", heroesFile, i, h.UID, heroesFile, j)
		}
		if j, dup := byMLID[h.MLID]; dup {
			return nil, fmt.Errorf("%s[%d].mlid: %d repeats %s[%d]", heroesFile, i, h.MLID, heroesFile, j)
		}
		byUID[h.UID], byMLID[h.MLID] = i, i
		heroes[i] = h
	}
	return heroes, nil
}

func decodeHero(row json.RawMessage) (hero.Hero, error) {
	var (
		h      hero.Hero
		mlid   string
		images json.RawMessage
	)
	err := decodeObject(row, []field{
		{key: "uid", dst: &h.UID},
		{key: "mlid", dst: &mlid},
		{key: "name", dst: &h.Name},
		{key: "roles", dst: &h.Roles},
		{key: "lanes", dst: &h.Lanes},
		{key: "images", dst: &images, optional: true},
	})
	if err != nil {
		return h, err
	}
	if h.MLID, err = positiveDigits(mlid); err != nil {
		return h, fmt.Errorf("mlid: %w", err)
	}
	if images == nil {
		images = json.RawMessage(`{}`)
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(images, &obj) != nil {
		return h, errors.New("images: not a JSON object")
	}
	h.Images = images
	return h, nil
}

// positiveDigits parses s as a positive integer written in decimal digits
// only, refusing signs, spaces, and zero.
func positiveDigits(s string) (int, error) {
	n, err := strconv.Atoi(s)
	valid := err == nil && n > 0 && s == strconv.Itoa(n)
	if !valid {
		return 0, fmt.Errorf("want a positive integer as decimal digits, got %q", s)
	}
	return n, nil
}

// readArray reads a file holding a JSON array and returns its elements undecoded.
func readArray(dir, name string) ([]json.RawMessage, error) {
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		if syntaxErr, ok := errors.AsType[*json.SyntaxError](err); ok {
			return nil, fmt.Errorf("%s: invalid JSON at byte %d", name, syntaxErr.Offset)
		}
		return nil, fmt.Errorf("%s: not a JSON array", name)
	}
	return rows, nil
}

// rowError places an error from one row at its file and index. A row error
// starts with the key it concerns, so the two join with a dot.
func rowError(file string, i int, err error) error {
	sep := "."
	if err == errNotObject {
		sep = ": "
	}
	return fmt.Errorf("%s[%d]%s%w", file, i, sep, err)
}

var errNotObject = errors.New("not a JSON object")

// field is one key of a JSON object and where its value decodes to.
type field struct {
	key      string
	dst      any
	optional bool
}

// decodeObject decodes raw, which must be a JSON object, key by key into the
// fields' destinations. It refuses a missing required key, a null value, a
// value of the wrong type, and any key not among the fields.
func decodeObject(raw json.RawMessage, fields []field) error {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || obj == nil {
		return errNotObject
	}
	known := make(map[string]bool, len(fields))
	for _, f := range fields {
		known[f.key] = true
	}
	for _, key := range slices.Sorted(maps.Keys(obj)) {
		if !known[key] {
			return fmt.Errorf("%s: unknown key", key)
		}
	}
	for _, f := range fields {
		value, present := obj[f.key]
		switch {
		case !present && f.optional:
			continue
		case !present:
			return fmt.Errorf("%s: missing", f.key)
		case string(value) == "null":
			return fmt.Errorf("%s: null", f.key)
		}
		if err := json.Unmarshal(value, f.dst); err != nil {
			if typeErr, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
				return fmt.Errorf("%s: want %s, got %s", f.key, typeErr.Type, typeErr.Value)
			}
			return fmt.Errorf("%s: %w", f.key, err)
		}
	}
	return nil
}

// matchup is a Counter or Synergy as its file spells it, before it becomes
// the type of its kind.
type matchup struct {
	first, second string
	reasons       []string
	types         []string
	proof         []proof
}

// proof has the field layout of counter.Proof and synergy.Proof, so it
// converts to either.
type proof struct {
	ID            string
	Category      string
	Priority      string
	Impact        string
	Summary       string
	WorksBestWhen []string
	FailureCases  []string
}

// matchupFiles describes one directory of per-hero matchup files: the keys its
// rows use and how a decoded row becomes the kind's type.
type matchupFiles[T interface{ Validate() error }] struct {
	dir                           string
	firstKey, secondKey, typesKey string
	build                         func(matchup) T
}

var counterFiles = matchupFiles[counter.Counter]{
	dir: "counters", firstKey: "targetHeroId", secondKey: "counterHeroId", typesKey: "counterTypes",
	build: func(m matchup) counter.Counter {
		ps := make([]counter.Proof, len(m.proof))
		for i, p := range m.proof {
			ps[i] = counter.Proof(p)
		}
		return counter.Counter{TargetHeroID: m.first, CounterHeroID: m.second, Reasons: m.reasons, CounterTypes: m.types, Proof: ps}
	},
}

var synergyFiles = matchupFiles[synergy.Synergy]{
	dir: "synergies", firstKey: "anchorHeroId", secondKey: "synergyHeroId", typesKey: "synergyTypes",
	build: func(m matchup) synergy.Synergy {
		ps := make([]synergy.Proof, len(m.proof))
		for i, p := range m.proof {
			ps[i] = synergy.Proof(p)
		}
		return synergy.Synergy{AnchorHeroID: m.first, SynergyHeroID: m.second, Reasons: m.reasons, SynergyTypes: m.types, Proof: ps}
	},
}

// loadMatchups reads every file in the kind's directory. A file is named for
// the hero its rows belong to, and that name is authoritative.
func loadMatchups[T interface{ Validate() error }](dir string, k matchupFiles[T], heroes map[string]bool) ([]T, error) {
	paths, err := filepath.Glob(filepath.Join(dir, k.dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var out []T
	proofAt := map[string]string{}
	for _, path := range paths {
		name := k.dir + "/" + filepath.Base(path)
		stem := strings.TrimSuffix(filepath.Base(path), ".json")
		rows, err := readArray(dir, name)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, fmt.Errorf("%s: no %s", name, k.dir)
		}
		partnerAt := map[string]int{}
		for i, row := range rows {
			m, err := k.decode(row)
			if err == nil {
				err = k.build(m).Validate()
			}
			if err == nil {
				err = k.checkReferences(m, stem, heroes)
			}
			if err != nil {
				return nil, rowError(name, i, err)
			}
			if j, dup := partnerAt[m.second]; dup {
				return nil, fmt.Errorf("%s[%d].%s: %q repeats %s[%d]", name, i, k.secondKey, m.second, name, j)
			}
			partnerAt[m.second] = i
			for pi, p := range m.proof {
				at := fmt.Sprintf("%s[%d].proof[%d]", name, i, pi)
				if prev, dup := proofAt[p.ID]; dup {
					return nil, fmt.Errorf("%s.id: %q repeats %s", at, p.ID, prev)
				}
				proofAt[p.ID] = at
			}
			out = append(out, k.build(m))
		}
	}
	return out, nil
}

func (k matchupFiles[T]) checkReferences(m matchup, stem string, heroes map[string]bool) error {
	if m.first != stem {
		return fmt.Errorf("%s: %q does not match the file name", k.firstKey, m.first)
	}
	if !heroes[m.first] {
		return fmt.Errorf("%s: %q is not in %s", k.firstKey, m.first, heroesFile)
	}
	if !heroes[m.second] {
		return fmt.Errorf("%s: %q is not in %s", k.secondKey, m.second, heroesFile)
	}
	return nil
}

func (k matchupFiles[T]) decode(row json.RawMessage) (matchup, error) {
	var (
		m      matchup
		proofs []json.RawMessage
	)
	err := decodeObject(row, []field{
		{key: k.firstKey, dst: &m.first},
		{key: k.secondKey, dst: &m.second},
		{key: "reasons", dst: &m.reasons},
		{key: k.typesKey, dst: &m.types},
		{key: "proof", dst: &proofs},
	})
	if err != nil {
		return m, err
	}
	m.proof = make([]proof, len(proofs))
	for i, raw := range proofs {
		p := &m.proof[i]
		err := decodeObject(raw, []field{
			{key: "id", dst: &p.ID},
			{key: "category", dst: &p.Category},
			{key: "priority", dst: &p.Priority},
			{key: "impact", dst: &p.Impact},
			{key: "summary", dst: &p.Summary},
			{key: "worksBestWhen", dst: &p.WorksBestWhen},
			{key: "failureCases", dst: &p.FailureCases},
		})
		if errors.Is(err, errNotObject) {
			return m, fmt.Errorf("proof[%d]: %w", i, err)
		}
		if err != nil {
			return m, fmt.Errorf("proof[%d].%w", i, err)
		}
	}
	return m, nil
}
