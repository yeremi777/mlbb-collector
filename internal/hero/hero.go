// Package hero is a playable character: its identity, Roles, and Lanes.
package hero

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Hero is one playable character. UID is the Hero ID and MLID the Moonton ID;
// MLID serializes as a JSON string, as the dataset and the API carry it.
type Hero struct {
	UID    string          `json:"uid"`
	MLID   int             `json:"mlid,string"`
	Name   string          `json:"name"`
	Roles  []string        `json:"roles"`
	Lanes  []string        `json:"lanes"`
	Images json.RawMessage `json:"images"`
}

var (
	roles = []string{"tank", "fighter", "assassin", "mage", "marksman", "support"}
	lanes = []string{"gold", "exp", "mid", "roam", "jungle"}
)

// Validate reports the first rule the hero breaks, prefixed with the key that
// breaks it.
func (h Hero) Validate() error {
	if strings.TrimSpace(h.UID) == "" {
		return fmt.Errorf("uid: blank")
	}
	if strings.TrimSpace(h.Name) == "" {
		return fmt.Errorf("name: blank")
	}
	if len(h.Roles) == 0 {
		return fmt.Errorf("roles: empty")
	}
	if err := checkMembers("roles", "role", h.Roles, roles); err != nil {
		return err
	}
	return checkMembers("lanes", "lane", h.Lanes, lanes)
}

// checkMembers requires every value to be in allowed and to appear once.
func checkMembers(key, noun string, values, allowed []string) error {
	for i, v := range values {
		if !slices.Contains(allowed, v) {
			return fmt.Errorf("%s[%d]: %q is not a %s", key, i, v, noun)
		}
		if slices.Contains(values[:i], v) {
			return fmt.Errorf("%s[%d]: %q repeats", key, i, v)
		}
	}
	return nil
}
