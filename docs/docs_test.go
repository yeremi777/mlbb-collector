package docs

import (
	"strings"
	"testing"
)

func TestSpecForSetsTheOnlyServer(t *testing.T) {
	got, err := SpecFor("https://api.example")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "servers:\n  - url: https://api.example\n") || strings.Contains(string(got), "127.0.0.1:8080") {
		t.Errorf("servers block not replaced:\n%s", string(got)[:400])
	}
	if len(got) != len(spec)-len("http://127.0.0.1:8080")+len("https://api.example") {
		t.Errorf("SpecFor changed more than the server URL")
	}
}

func TestSpecForRefusesASpecWithoutTheServerBlock(t *testing.T) {
	saved := spec
	t.Cleanup(func() { spec = saved })
	spec = []byte("openapi: 3.1.0\npaths: {}\n")
	if _, err := SpecFor("https://api.example"); err == nil ||
		err.Error() != `docs/openapi.yaml: want exactly one "servers:\n  - url: http://127.0.0.1:8080\n" block, found 0` {
		t.Errorf("err = %v", err)
	}
}
