// Package docs embeds the OpenAPI contract the API serves.
package docs

import (
	_ "embed"
	"slices"
)

//go:embed openapi.yaml
var spec []byte

// Spec returns the contract as written in docs/openapi.yaml.
func Spec() []byte { return slices.Clone(spec) }
