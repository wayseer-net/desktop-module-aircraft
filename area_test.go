package aircraft

import (
	"math"
	"testing"
)

func TestDistanceIsInNauticalMiles(t *testing.T) {
	// One minute of latitude is one nautical mile.
	if d := distanceNM(50, 0, 51, 0); math.Abs(d-60) > 0.1 {
		t.Errorf("one degree of latitude is %.2f nm", d)
	}
	if d := distanceNM(10, 20, 10, 20); d != 0 {
		t.Errorf("no distance is %.2f nm", d)
	}
}

func TestACircleContainsWhatIsWithinItsRadius(t *testing.T) {
	c := area{circle: &circle{Lat: 50, Lon: 0, RadiusNM: 30}}
	if !c.contains(50.4, 0) || c.contains(50.6, 0) {
		t.Error("a 30 nm circle holds 24 nm north but not 36 nm")
	}
	if lat, lon, r := c.query(); lat != 50 || lon != 0 || r != 30 {
		t.Errorf("queried %v %v %v", lat, lon, r)
	}
}

func TestABoxIsQueriedByTheCircleAroundIt(t *testing.T) {
	b := area{box: &box{South: 50, West: -1, North: 51, East: 1}}
	lat, lon, r := b.query()
	if lat != 50.5 || lon != 0 {
		t.Errorf("centre %v, %v", lat, lon)
	}
	for _, corner := range [][2]float64{{50, -1}, {51, 1}, {50, 1}, {51, -1}} {
		if d := distanceNM(lat, lon, corner[0], corner[1]); d > r {
			t.Errorf("corner %v is %.1f nm out, beyond the radius %.1f", corner, d, r)
		}
	}
	if !b.contains(50.5, 0.9) || b.contains(50.5, 1.1) || b.contains(51.1, 0) {
		t.Error("the box holds only what is inside it")
	}
}

func TestNoAreaHoldsEverything(t *testing.T) {
	var none area
	if !none.contains(-80, 170) || none.set() {
		t.Error("no area is unset and holds everything")
	}
}

func TestAnAreaIsNamedByWhereItIs(t *testing.T) {
	for _, c := range []struct {
		a    area
		want string
	}{
		{area{circle: &circle{Lat: 51.47, Lon: -0.45, RadiusNM: 40}}, "40 nm around 51.47, -0.45"},
		{area{box: &box{South: 50, West: -1.5, North: 51, East: 1}}, "50, -1.5 to 51, 1"},
		{area{}, "everywhere"},
	} {
		if got := c.a.String(); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}
