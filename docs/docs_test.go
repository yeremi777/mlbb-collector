package docs

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSpecIsTheContractFile(t *testing.T) {
	onDisk, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if string(Spec()) != string(onDisk) {
		t.Error("Spec() differs from docs/openapi.yaml")
	}
}

// With no servers, OpenAPI's default server is "/", the address the docs page
// was opened on, so the page calls localhost and 127.0.0.1 alike without
// showing a servers picker.
func TestSpecDeclaresNoServers(t *testing.T) {
	var spec map[string]any
	if err := yaml.Unmarshal(Spec(), &spec); err != nil {
		t.Fatal(err)
	}
	if servers, ok := spec["servers"]; ok {
		t.Errorf("servers = %v, want none", servers)
	}
}
