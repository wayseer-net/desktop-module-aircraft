package aircraft

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"wayseer.dev/sdk"
	"wayseer.dev/sdk/sdktest"
)

// feed serves a JSON body the test can change.
type feed struct {
	mu   sync.Mutex
	body string
}

func (f *feed) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(f.body))
}

func (f *feed) set(body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.body = body
}

// serve starts a server for a fixture feed.
func serve(t *testing.T, fixture string) (*feed, *httptest.Server) {
	t.Helper()
	b, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	f := &feed{body: string(b)}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
}

// receiver is the options for a receiver at srv, then any more.
func receiver(srv *httptest.Server, more ...string) string {
	return strings.Join(append([]string{"source: receiver", "url: " + srv.URL}, more...), "\n")
}

// configured is a module configured with options.
func configured(t *testing.T, options string) *Module {
	t.Helper()
	cfg, err := sdktest.Config("air", options)
	if err != nil {
		t.Fatal(err)
	}
	m := New()
	if err := m.Configure(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	return m
}

func closedURL() string {
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	return closed.URL
}

func TestConformanceWithAReceiver(t *testing.T) {
	_, srv := serve(t, "testdata/aircraft.json")
	sdktest.Conform(t, sdktest.Case{
		New:      func() sdk.Module { return New() },
		Name:     "air",
		Options:  receiver(srv),
		Failing:  "source: receiver\nurl: " + closedURL(),
		Manifest: "manifest.yaml",
	})
}

func TestConformanceWithADSBLol(t *testing.T) {
	_, srv := serve(t, "testdata/adsblol.json")
	sdktest.Conform(t, sdktest.Case{
		New:      func() sdk.Module { return New() },
		Name:     "air",
		Options:  "url: " + srv.URL + "\n" + london,
		Failing:  "url: " + closedURL() + "\n" + london,
		Manifest: "manifest.yaml",
	})
}

func TestSnapshotMatchesTheFixture(t *testing.T) {
	_, srv := serve(t, "testdata/aircraft.json")
	cs, err := configured(t, receiver(srv)).Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sdktest.Golden(t, "testdata/aircraft.txt", render(cs))
}

func TestADSBLolAircraftAreInTheArea(t *testing.T) {
	_, srv := serve(t, "testdata/adsblol.json")
	cs, err := configured(t, "url: "+srv.URL+"\n"+london).Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sdktest.Golden(t, "testdata/adsblol.txt", render(cs))
}

// render lists a change set's entities and edges, one per line.
func render(cs *sdk.ChangeSet) string {
	var b strings.Builder
	for _, e := range cs.Upserts {
		keys := make([]string, 0, len(e.Attrs))
		for k, v := range e.Attrs {
			keys = append(keys, k+"="+v.String())
		}
		slices.Sort(keys)
		place := "-"
		if e.Place.Known {
			place = fmt.Sprintf("%.3f,%.3f", e.Place.Lat, e.Place.Lon)
		}
		fmt.Fprintf(&b, "%s %q %s %s %q %s\n", e.Ref, e.Name, place, e.Status.Level, e.Status.Reason, strings.Join(keys, " "))
	}
	for _, e := range cs.Edges {
		fmt.Fprintf(&b, "%s %s %s\n", e.From, e.Rel, e.To)
	}
	return b.String()
}

func TestOperatorsCanBeLeftOut(t *testing.T) {
	_, srv := serve(t, "testdata/aircraft.json")
	cs, err := configured(t, receiver(srv, "operators: false")).Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cs.Edges) > 0 || slices.ContainsFunc(cs.Upserts, func(e sdk.Entity) bool { return e.Kind == KindOperator }) {
		t.Errorf("operators sent: %s", render(cs))
	}
}

func TestAnAircraftMovesOnTheMap(t *testing.T) {
	f, srv := serve(t, "testdata/aircraft.json")
	m := configured(t, receiver(srv, "interval: 500ms"))
	sink := sdktest.Run(t, func(ctx context.Context, s *sdktest.Sink) error { return m.Run(ctx, s) })
	sink.WaitFor(t, 1)
	f.set(`{"aircraft": [{"hex": "4ca9f1", "flight": "EIN12A", "lat": 51.6, "lon": -0.2, "seen": 0}]}`)
	ref, want := aircraftRef(t, "4ca9f1"), sdk.At(51.6, -0.2)
	sdktest.Eventually(t, func() bool {
		return slices.ContainsFunc(sink.Sets(), func(cs sdk.ChangeSet) bool {
			return slices.ContainsFunc(cs.Upserts, func(e sdk.Entity) bool { return e.Ref == ref && e.Place == want })
		})
	})
}

func TestEachReadIsAPointInTheAltitudeSeries(t *testing.T) {
	f, srv := serve(t, "testdata/aircraft.json")
	m := configured(t, receiver(srv, "interval: 500ms"))
	sink := sdktest.Run(t, func(ctx context.Context, s *sdktest.Sink) error { return m.Run(ctx, s) })
	sink.WaitFor(t, 1)
	f.set(`{"aircraft": [{"hex": "4ca9f1", "alt_baro": 13000, "seen": 0}]}`)
	ref := sdk.SeriesRef{Entity: aircraftRef(t, "4ca9f1"), Metric: metricAltitude}
	query := func() []sdk.Series {
		now := time.Now()
		got, err := m.QuerySeries(context.Background(), sdk.SeriesQuery{
			Entities: []sdk.EntityRef{ref.Entity}, Metrics: []string{ref.Metric},
			Window: sdk.TimeWindow{From: now.Add(-time.Minute), To: now.Add(time.Second)},
		})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	sdktest.Eventually(t, func() bool {
		got := query()
		return len(got) == 1 && len(got[0].Points) >= 2 && got[0].Points[len(got[0].Points)-1].V == 13000
	})
	if got := query()[0]; got.Points[0].V != 12000 {
		t.Errorf("series %+v", got)
	}
}

func aircraftRef(t *testing.T, hex string) sdk.EntityRef {
	t.Helper()
	ref, err := sdk.NewEntityRef("air", KindAircraft, hex)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func TestARateLimitIsWaitedOut(t *testing.T) {
	every := 10 * time.Second
	for _, c := range []struct {
		err  error
		want time.Duration
	}{
		{fmt.Errorf("refused"), every},
		{&limitedError{wait: 0}, 2 * every},
		{&limitedError{wait: time.Minute}, time.Minute},
		{&limitedError{wait: time.Hour}, limitMax},
	} {
		if got := retryAfter(c.err, every); got != c.want {
			t.Errorf("%v: waits %v, want %v", c.err, got, c.want)
		}
	}
}
