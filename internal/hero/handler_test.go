package hero

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeStore struct {
	heroes []Hero
	err    error
}

func (f fakeStore) List(context.Context) ([]Hero, error) { return f.heroes, f.err }

func (f fakeStore) Get(_ context.Context, uid string) (Hero, error) {
	if f.err != nil {
		return Hero{}, f.err
	}
	for _, h := range f.heroes {
		if h.UID == uid {
			return h, nil
		}
	}
	return Hero{}, ErrNotFound
}

func get(t *testing.T, store fakeStore, target string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(store).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

var tigreal = Hero{UID: "tigreal", MLID: 6, Name: "Tigreal", Roles: []string{"tank"}, Lanes: []string{"roam"},
	Images: json.RawMessage(`{"head": "https://example.test/t.png"}`)}

const tigrealJSON = `{"uid":"tigreal","mlid":"6","name":"Tigreal","roles":["tank"],"lanes":["roam"],"images":{"head":"https://example.test/t.png"}}`

func TestHandlerListsHeroes(t *testing.T) {
	rec := get(t, fakeStore{heroes: []Hero{tigreal}}, "/api/heroes?role=tank")
	want := `{"items":[` + tigrealJSON + `],"page":1,"size":10,"total":1,"pages":1}` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Errorf("got %d %q, want 200 %q", rec.Code, rec.Body.String(), want)
	}

	empty := get(t, fakeStore{heroes: []Hero{tigreal}}, "/api/heroes?search=zzz")
	if want := `{"items":[],"page":1,"size":10,"total":0,"pages":0}` + "\n"; empty.Body.String() != want {
		t.Errorf("empty page: got %q, want %q", empty.Body.String(), want)
	}
}

func TestHandlerGetsAHero(t *testing.T) {
	rec := get(t, fakeStore{heroes: []Hero{tigreal}}, "/api/heroes/tigreal")
	if rec.Code != http.StatusOK || rec.Body.String() != tigrealJSON+"\n" {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}

	missing := get(t, fakeStore{heroes: []Hero{tigreal}}, "/api/heroes/nope")
	if want := `{"error":{"code":"hero_not_found","message":"Hero was not found in the dataset."}}` + "\n"; missing.Code != http.StatusNotFound || missing.Body.String() != want {
		t.Errorf("unknown hero: got %d %q, want 404 %q", missing.Code, missing.Body.String(), want)
	}
}

func TestHandlerHidesStoreFailures(t *testing.T) {
	store := fakeStore{err: errors.New("connection refused at 10.0.0.5")}
	want := `{"error":{"code":"internal_error","message":"Internal server error."}}` + "\n"
	for _, target := range []string{"/api/heroes", "/api/heroes/tigreal"} {
		if rec := get(t, store, target); rec.Code != http.StatusInternalServerError || rec.Body.String() != want {
			t.Errorf("%s: got %d %q, want 500 %q", target, rec.Code, rec.Body.String(), want)
		}
	}
}
