package aircraft

import (
	"slices"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func at(hex string, lat, lon, seen float64) report {
	return report{Hex: hex, Lat: &lat, Lon: &lon, Seen: seen}
}

func hexes(f *fleet) []string {
	var out []string
	for _, r := range f.aircraft() {
		out = append(out, r.Hex)
	}
	return out
}

func TestAnAircraftStaysUntilUnheardForExpire(t *testing.T) {
	f := newFleet(area{}, time.Minute, 10)
	f.update([]report{at("aaaaa1", 50, 0, 0), at("aaaaa2", 50, 0, 70)}, t0)
	if got := hexes(f); !slices.Equal(got, []string{"aaaaa1"}) {
		t.Fatalf("heard 70s ago is kept: %v", got)
	}
	f.update(nil, t0.Add(59*time.Second))
	if got := hexes(f); len(got) != 1 {
		t.Errorf("missing from one read, it is dropped at once: %v", got)
	}
	f.update(nil, t0.Add(61*time.Second))
	if got := hexes(f); len(got) != 0 {
		t.Errorf("unheard for over a minute, it stays: %v", got)
	}
}

func TestTheNewestReportOfAnAircraftWins(t *testing.T) {
	f := newFleet(area{}, time.Minute, 10)
	f.update([]report{at("aaaaa1", 50, 0, 0)}, t0)
	f.update([]report{at("aaaaa1", 51, 0, 0)}, t0.Add(time.Second))
	f.update([]report{at("aaaaa1", 52, 0, 30)}, t0.Add(2*time.Second)) // older than what is held
	if r := f.aircraft()[0]; *r.Lat != 51 {
		t.Errorf("held the report at %v", *r.Lat)
	}
}

func TestTheWorkingSetKeepsTheMostRecentlyHeard(t *testing.T) {
	f := newFleet(area{}, time.Minute, 2)
	f.update([]report{at("aaaaa1", 50, 0, 5), at("aaaaa2", 50, 0, 1), at("aaaaa3", 50, 0, 3)}, t0)
	if got := hexes(f); !slices.Equal(got, []string{"aaaaa2", "aaaaa3"}) {
		t.Errorf("kept %v", got)
	}
}

func TestOnlyAircraftInTheAreaAreKept(t *testing.T) {
	f := newFleet(area{circle: &circle{Lat: 50, Lon: 0, RadiusNM: 30}}, time.Minute, 10)
	f.update([]report{at("aaaaa1", 50.1, 0, 0), at("aaaaa2", 52, 0, 0), {Hex: "aaaaa3"}}, t0)
	if got := hexes(f); !slices.Equal(got, []string{"aaaaa1"}) {
		t.Fatalf("kept %v", got)
	}
	f.update([]report{at("aaaaa1", 52, 0, 0)}, t0.Add(time.Second))
	if got := hexes(f); len(got) != 0 {
		t.Errorf("an aircraft that left the area stays: %v", got)
	}
}

func TestWithNoAreaAnAircraftWithNoPositionIsKept(t *testing.T) {
	f := newFleet(area{}, time.Minute, 10)
	f.update([]report{{Hex: "aaaaa3"}}, t0)
	if got := hexes(f); len(got) != 1 {
		t.Errorf("kept %v", got)
	}
}
