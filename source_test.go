package aircraft

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"wayseer.dev/sdk"
)

func decodeFile(t *testing.T, name string) []report {
	t.Helper()
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	rs, err := decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func TestReadsbAndADSBLolFeedsDecodeAlike(t *testing.T) {
	recv, lol := decodeFile(t, "testdata/aircraft.json"), decodeFile(t, "testdata/adsblol.json")
	if len(recv) != 5 || len(lol) != 2 {
		t.Fatalf("decoded %d and %d aircraft", len(recv), len(lol))
	}
	a, b := recv[0], lol[0]
	if a.Hex != b.Hex || a.callsign() != "EIN12A" || *a.Lat != *b.Lat || a.Alt.Feet != 12000 || *a.Speed != 310.5 {
		t.Errorf("receiver %+v, adsb.lol %+v", a, b)
	}
}

func TestAltitudeIsFeetOrGround(t *testing.T) {
	rs := decodeFile(t, "testdata/aircraft.json")
	if g := rs[1].Alt; !g.Known || !g.Ground || g.Feet != 0 {
		t.Errorf("on the ground: %+v", g)
	}
	if a := rs[0].Alt; !a.Known || a.Ground || a.Feet != 12000 {
		t.Errorf("aloft: %+v", a)
	}
}

func TestBadFeedsAreErrors(t *testing.T) {
	for _, body := range []string{
		`not json`,
		`{"aircraft": [{"hex": "nothex"}]}`,
		`{"aircraft": [{"hex": ""}]}`,
		`{"aircraft": [{"hex": "4ca9f1", "alt_baro": "high"}]}`,
		`{"aircraft": [{"hex": "4ca9f1", "lat": "north"}]}`,
		`{"aircraft": [{"hex": "4ca9f1", "seen": -5}]}`,
	} {
		if _, err := decode(strings.NewReader(body)); err == nil {
			t.Errorf("%s accepted", body)
		}
	}
}

func TestAPositionOffTheEarthIsNoPosition(t *testing.T) {
	rs, err := decode(strings.NewReader(`{"aircraft": [{"hex": "4ca9f1", "lat": 95, "lon": 10}]}`))
	if err != nil || rs[0].placed() {
		t.Errorf("%+v, %v", rs, err)
	}
}

func TestADSBLolIsAskedForTheCircleAroundTheArea(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"ac": []}`))
	}))
	defer srv.Close()
	o, _ := parse(t, "url: "+srv.URL+"\n"+london)
	if _, err := newSource(&o, sdk.Secret{}).read(context.Background()); err != nil {
		t.Fatal(err)
	}
	if path != "/v2/point/51.47/-0.45/40" {
		t.Errorf("asked for %s", path)
	}
}

func TestARateLimitSaysHowLongToWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	o, _ := parse(t, "url: "+srv.URL+"\n"+london)
	_, err := newSource(&o, sdk.Secret{}).read(context.Background())
	var lim *limitedError
	if !errors.As(err, &lim) || lim.wait != 30*time.Second {
		t.Fatalf("error %v", err)
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error %q doesn't say the status", err)
	}
}

func TestTheReceiverTokenIsSentButNeverShown(t *testing.T) {
	var auth string
	body := `{"aircraft": []}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	t.Setenv("AIR_TOKEN", "s3cret-token")
	o, err := parse(t, "source: receiver\nurl: "+srv.URL+"\nsecret_env: AIR_TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	tok, _ := o.Read()
	src := newSource(&o, tok)
	if _, err := src.read(context.Background()); err != nil || auth != "Bearer s3cret-token" {
		t.Fatalf("Authorization %q, %v", auth, err)
	}
	body = "not json"
	if _, err := src.read(context.Background()); err == nil || strings.Contains(err.Error(), "s3cret") {
		t.Errorf("error %v", err)
	}
}
