package snapshot_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yeremi777/mlbb-collector/internal/snapshot"
)

// record is one upstream hero record, as Moonton wraps it.
func record(id int) string {
	return fmt.Sprintf(`{"data": {"main_heroid": %d, "main_hero_win_rate": 0.512345, "main_hero_appearance_rate": %g, "main_hero_ban_rate": 0.0123, "sub_hero": [{"heroid": 7, "increase_win_rate": 0.031}]}, "id": %d}`,
		id, 1.0/101, 9000+id)
}

func records(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = record(i + 1)
	}
	return out
}

func body(code, total int, recs []string) string {
	list := "null"
	if recs != nil {
		list = "[" + strings.Join(recs, ",") + "]"
	}
	return fmt.Sprintf(`{"code": %d, "message": "OK", "data": {"total": %d, "records": %s}}`, code, total, list)
}

// stub answers each request with the next reply and records what it received.
type stub struct {
	mu       sync.Mutex
	replies  []func(http.ResponseWriter)
	requests []*http.Request
	bodies   []string
}

func (s *stub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := io.ReadAll(r.Body)
	s.requests = append(s.requests, r)
	s.bodies = append(s.bodies, string(b))
	reply := s.replies[0]
	if len(s.replies) > 1 {
		s.replies = s.replies[1:]
	}
	reply(w)
}

func ok(b string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) { _, _ = io.WriteString(w, b) }
}

func status(code int) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) { w.WriteHeader(code) }
}

func newClient(t *testing.T, s *stub) (*snapshot.Client, *[]time.Duration) {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	c := snapshot.NewClient(srv.URL+"/source", "probe/1.0 (me@example.com)")
	var waits []time.Duration
	c.Sleep = func(d time.Duration) { waits = append(waits, d) }
	return c, &waits
}

func TestFetchSendsTheRankStatisticsRequest(t *testing.T) {
	s := &stub{replies: []func(http.ResponseWriter){ok(body(0, 101, records(101)))}}
	c, _ := newClient(t, s)

	got, err := c.Fetch(context.Background(), snapshot.Combination{Window: 7, RankTier: "mythic"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 101 {
		t.Fatalf("got %d records, want 101", len(got))
	}
	r := s.requests[0]
	if r.Method != http.MethodPost || r.URL.Path != "/source/2756569" {
		t.Errorf("request = %s %s, want POST /source/2756569", r.Method, r.URL.Path)
	}
	for header, want := range map[string]string{
		"Origin":       "https://www.mobilelegends.com",
		"Referer":      "https://www.mobilelegends.com",
		"User-Agent":   "probe/1.0 (me@example.com)",
		"Content-Type": "application/json",
	} {
		if r.Header.Get(header) != want {
			t.Errorf("%s = %q, want %q", header, r.Header.Get(header), want)
		}
	}
	var sent struct {
		PageSize  int                 `json:"pageSize"`
		PageIndex int                 `json:"pageIndex"`
		Filters   []map[string]string `json:"filters"`
		Sorts     []json.RawMessage   `json:"sorts"`
		Fields    []string            `json:"fields"`
	}
	if err := json.Unmarshal([]byte(s.bodies[0]), &sent); err != nil {
		t.Fatal(err)
	}
	if sent.PageSize != 200 || sent.PageIndex != 1 || sent.Fields == nil || len(sent.Fields) != 0 {
		t.Errorf("body = %s", s.bodies[0])
	}
	filters := map[string]string{}
	for _, f := range sent.Filters {
		filters[f["field"]] = f["value"]
	}
	if filters["bigrank"] != "7" || filters["match_type"] != "0" || len(filters) != 2 {
		t.Errorf("filters = %v, want bigrank 7 and match_type 0", filters)
	}
	if len(sent.Sorts) != 1 || !strings.Contains(string(sent.Sorts[0]), `"main_hero_win_rate"`) || !strings.Contains(string(sent.Sorts[0]), `"desc"`) {
		t.Errorf("sorts = %s, want main_hero_win_rate descending", sent.Sorts)
	}
}

func TestFetchDecodesRecordsAndKeepsThePayload(t *testing.T) {
	s := &stub{replies: []func(http.ResponseWriter){ok(body(0, 101, records(101)))}}
	c, _ := newClient(t, s)

	got, err := c.Fetch(context.Background(), snapshot.Combination{Window: 1, RankTier: "all"})
	if err != nil {
		t.Fatal(err)
	}
	first := got[0]
	if first.MoontonID != 1 || first.WinRate != 0.512345 || first.AppearanceShare != 1.0/101 || first.BanRate != 0.0123 {
		t.Errorf("record = %+v", first)
	}
	if string(first.Payload) != record(1) {
		t.Errorf("payload = %s\nwant      %s", first.Payload, record(1))
	}
}

func TestFetchUsesTheEndpointAndBigrankOfEachCombination(t *testing.T) {
	endpoints := map[int]string{1: "2756567", 3: "2756568", 7: "2756569", 15: "2756565", 30: "2756570"}
	bigranks := map[string]string{"all": "101", "epic": "5", "legend": "6", "mythic": "7", "honor": "8", "glory": "9"}
	s := &stub{replies: []func(http.ResponseWriter){ok(body(0, 101, records(101)))}}
	c, _ := newClient(t, s)

	combos := snapshot.Combinations()
	for _, combo := range combos {
		if _, err := c.Fetch(context.Background(), combo); err != nil {
			t.Fatal(err)
		}
	}
	if len(combos) != 30 || len(s.requests) != 30 {
		t.Fatalf("%d combinations, %d requests, want 30", len(combos), len(s.requests))
	}
	for i, combo := range combos {
		if want := "/source/" + endpoints[combo.Window]; s.requests[i].URL.Path != want {
			t.Errorf("%+v: path %s, want %s", combo, s.requests[i].URL.Path, want)
		}
		if want := `"value":"` + bigranks[combo.RankTier] + `"`; !strings.Contains(s.bodies[i], want) {
			t.Errorf("%+v: body %s, want %s", combo, s.bodies[i], want)
		}
	}
}

func TestCombinationsAreWindowsAscendingThenRankTiers(t *testing.T) {
	combos := snapshot.Combinations()
	var got []string
	for _, c := range combos[:7] {
		got = append(got, fmt.Sprintf("%d/%s", c.Window, c.RankTier))
	}
	want := "1/all 1/epic 1/legend 1/mythic 1/honor 1/glory 3/all"
	if strings.Join(got, " ") != want {
		t.Errorf("got %v, want %s", got, want)
	}
	if last := combos[29]; last.Window != 30 || last.RankTier != "glory" {
		t.Errorf("last = %+v, want 30/glory", last)
	}
}

func TestFetchFailsAtOnceOnARejectedBody(t *testing.T) {
	for name, b := range map[string]string{
		"records null":      body(0, 0, nil),
		"records empty":     body(0, 0, []string{}),
		"code non-zero":     body(1001, 101, records(101)),
		"total 100":         body(0, 100, records(100)),
		"record without id": body(0, 101, append(records(100), `{"data": {"main_hero_win_rate": 0.5, "main_hero_appearance_rate": 0.01, "main_hero_ban_rate": 0.01}}`)),
	} {
		t.Run(name, func(t *testing.T) {
			s := &stub{replies: []func(http.ResponseWriter){ok(b)}}
			c, waits := newClient(t, s)
			if _, err := c.Fetch(context.Background(), snapshot.Combination{Window: 1, RankTier: "all"}); err == nil {
				t.Fatal("want an error")
			}
			if len(s.requests) != 1 || len(*waits) != 0 {
				t.Errorf("%d requests, waits %v; want 1 request and no retry", len(s.requests), *waits)
			}
		})
	}
}

func TestFetchRetriesOnceAfterOneSecond(t *testing.T) {
	for name, first := range map[string]func(http.ResponseWriter){
		"short read": ok(body(0, 101, records(100))),
		"status 500": status(http.StatusInternalServerError),
		"bad json":   ok(`{"code": 0, "data": {"total": 101, "records": [`),
	} {
		t.Run(name, func(t *testing.T) {
			s := &stub{replies: []func(http.ResponseWriter){first, ok(body(0, 101, records(101)))}}
			c, waits := newClient(t, s)
			got, err := c.Fetch(context.Background(), snapshot.Combination{Window: 1, RankTier: "all"})
			if err != nil || len(got) != 101 {
				t.Fatalf("got %d records, err %v; want the retry's 101", len(got), err)
			}
			if len(s.requests) != 2 || len(*waits) != 1 || (*waits)[0] != time.Second {
				t.Errorf("%d requests, waits %v; want 2 requests and one 1s wait", len(s.requests), *waits)
			}
		})
	}
}

func TestFetchGivesUpAfterTheRetry(t *testing.T) {
	s := &stub{replies: []func(http.ResponseWriter){status(http.StatusBadGateway)}}
	c, _ := newClient(t, s)
	if _, err := c.Fetch(context.Background(), snapshot.Combination{Window: 1, RankTier: "all"}); err == nil {
		t.Fatal("want an error")
	}
	if len(s.requests) != 2 {
		t.Errorf("%d requests, want 2", len(s.requests))
	}
}

func TestFetchRetriesATransportError(t *testing.T) {
	c := snapshot.NewClient("http://127.0.0.1:1", "probe/1.0")
	var waits []time.Duration
	c.Sleep = func(d time.Duration) { waits = append(waits, d) }
	if _, err := c.Fetch(context.Background(), snapshot.Combination{Window: 1, RankTier: "all"}); err == nil {
		t.Fatal("want an error")
	}
	if len(waits) != 1 {
		t.Errorf("waits %v, want one retry", waits)
	}
}

func TestFetchDoesNotRetryACanceledRequest(t *testing.T) {
	s := &stub{replies: []func(http.ResponseWriter){ok(body(0, 101, records(101)))}}
	c, waits := newClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Fetch(ctx, snapshot.Combination{Window: 1, RankTier: "all"}); err == nil {
		t.Fatal("want an error")
	}
	if len(*waits) != 0 {
		t.Errorf("waits %v, want no retry", *waits)
	}
}
