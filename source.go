package aircraft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"wayseer.dev/sdk"
)

// maxBody is the largest feed read.
const maxBody = 16 << 20

// hexPattern is a 24-bit address as six hex digits; '~' marks one that is not ICAO's.
var hexPattern = regexp.MustCompile(`^~?[0-9a-f]{6}$`)

// report is one aircraft as readsb writes it, in aircraft.json and in the adsb.lol API.
type report struct {
	Hex       string   `json:"hex"`
	Flight    string   `json:"flight"` // the callsign, padded with spaces
	Reg       string   `json:"r"`
	Type      string   `json:"t"`
	Category  string   `json:"category"`
	Squawk    string   `json:"squawk"`
	Emergency string   `json:"emergency"`
	Alt       altitude `json:"alt_baro"`
	Speed     *float64 `json:"gs"`        // ground speed, knots
	Track     *float64 `json:"track"`     // degrees true
	Rate      *float64 `json:"baro_rate"` // feet per minute
	Lat       *float64 `json:"lat"`
	Lon       *float64 `json:"lon"`
	Seen      float64  `json:"seen"` // seconds since last heard
}

// altitude is barometric altitude in feet, or on the ground.
type altitude struct {
	Feet   float64
	Ground bool
	Known  bool
}

func (a *altitude) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case "null":
		return nil
	case `"ground"`:
		*a = altitude{Ground: true, Known: true}
		return nil
	}
	if err := json.Unmarshal(b, &a.Feet); err != nil {
		return fmt.Errorf("alt_baro %s is not feet or \"ground\"", b)
	}
	a.Known = true
	return nil
}

func (r *report) callsign() string { return strings.TrimSpace(r.Flight) }

func (r *report) placed() bool { return r.Lat != nil && r.Lon != nil }

// check normalises r and rejects what cannot be read; a position off the Earth is dropped.
func (r *report) check() error {
	r.Hex = strings.ToLower(r.Hex)
	if !hexPattern.MatchString(r.Hex) {
		return fmt.Errorf("aircraft %q: hex is not six hex digits", r.Hex)
	}
	if r.Seen < 0 || math.IsNaN(r.Seen) {
		return fmt.Errorf("aircraft %s: seen %v is negative", r.Hex, r.Seen)
	}
	if r.placed() && checkPoint(*r.Lat, *r.Lon) != nil {
		r.Lat, r.Lon = nil, nil
	}
	return nil
}

// decode reads a feed: adsb.lol lists aircraft under "ac", readsb under "aircraft".
func decode(rd io.Reader) ([]report, error) {
	var feed struct {
		AC       []report `json:"ac"`
		Aircraft []report `json:"aircraft"`
	}
	if err := json.NewDecoder(io.LimitReader(rd, maxBody)).Decode(&feed); err != nil {
		return nil, err
	}
	rs := append(feed.AC, feed.Aircraft...)
	for i := range rs {
		if err := rs[i].check(); err != nil {
			return nil, err
		}
	}
	return rs, nil
}

// source reads one feed over HTTP.
type source struct {
	client *http.Client
	url    string
	token  sdk.Secret
}

func newSource(o *options, token sdk.Secret) *source {
	return &source{client: &http.Client{Timeout: o.Timeout}, url: requestURL(o), token: token}
}

// requestURL is the receiver's URL, or adsb.lol's query for the circle around the area.
func requestURL(o *options) string {
	if o.Source == sourceReceiver {
		return o.URL
	}
	lat, lon, r := o.area().query()
	return strings.TrimRight(o.URL, "/") + "/v2/point/" + num(lat) + "/" + num(lon) + "/" + num(math.Ceil(r))
}

// read fetches and decodes the feed; errors name the URL but never the token.
func (s *source) read(ctx context.Context) ([]report, error) {
	resp, err := s.get(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	rs, err := decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.url, err)
	}
	return rs, nil
}

// get requests the feed with the token, if any, and returns a 200 response.
func (s *source) get(ctx context.Context) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if t := s.token.Reveal(); t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, statusError(s.url, resp)
	}
	return resp, nil
}

// limitedError is a source asking to be read less often, and for how long to wait.
type limitedError struct {
	url    string
	status string
	wait   time.Duration
}

func (e *limitedError) Error() string {
	return fmt.Sprintf("%s: %s, rate limited", e.url, e.status)
}

func statusError(url string, resp *http.Response) error {
	if resp.StatusCode == http.StatusTooManyRequests {
		secs, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
		return &limitedError{url: url, status: resp.Status, wait: time.Duration(max(secs, 0)) * time.Second}
	}
	return fmt.Errorf("%s: %s", url, resp.Status)
}

// errLimited reports whether err is a rate limit, and how long it asks to wait.
func errLimited(err error) (time.Duration, bool) {
	var lim *limitedError
	if errors.As(err, &lim) {
		return lim.wait, true
	}
	return 0, false
}
