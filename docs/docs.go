// Package docs embeds the OpenAPI contract the API serves.
package docs

import (
	"bytes"
	_ "embed"
	"fmt"
)

//go:embed openapi.yaml
var spec []byte

// specServer is the servers block as openapi.yaml is written.
const specServer = "servers:\n  - url: http://127.0.0.1:8080\n"

// SpecFor returns the contract with serverURL as its only server.
func SpecFor(serverURL string) ([]byte, error) {
	if n := bytes.Count(spec, []byte(specServer)); n != 1 {
		return nil, fmt.Errorf("docs/openapi.yaml: want exactly one %q block, found %d", specServer, n)
	}
	return bytes.Replace(spec, []byte(specServer), []byte("servers:\n  - url: "+serverURL+"\n"), 1), nil
}
