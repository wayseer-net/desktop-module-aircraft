package aircraft

import (
	"testing"
	"time"

	"wayseer.dev/sdk/sdktest"
)

const london = "area: {lat: 51.47, lon: -0.45, radius_nm: 40}"

func parse(t *testing.T, yaml string) (options, error) {
	t.Helper()
	cfg, err := sdktest.Config("air", yaml)
	if err != nil {
		t.Fatal(err)
	}
	return readOptions(cfg)
}

func TestDefaultsFollowTheSource(t *testing.T) {
	o, err := parse(t, london)
	if err != nil {
		t.Fatal(err)
	}
	if o.Source != sourceADSBLol || o.URL != adsbLolURL || o.Interval != 10*time.Second || !o.Operators {
		t.Errorf("adsb.lol defaults: %+v", o)
	}
	o, err = parse(t, "source: receiver\nurl: http://receiver.test/data/aircraft.json")
	if err != nil {
		t.Fatal(err)
	}
	if o.Interval != 2*time.Second || o.Expire != time.Minute || o.MaxAircraft != 1000 || o.area().set() {
		t.Errorf("receiver defaults: %+v", o)
	}
}

func TestABoxIsAnArea(t *testing.T) {
	o, err := parse(t, "box: {south: 51, west: -1, north: 52, east: 0}")
	if err != nil {
		t.Fatal(err)
	}
	if a := o.area(); a.box == nil || a.circle != nil {
		t.Errorf("area %+v", a)
	}
}

func TestBadOptionsAreRejected(t *testing.T) {
	recv := "source: receiver\nurl: http://receiver.test/aircraft.json\n"
	for _, opts := range []string{
		"",                           // adsb.lol needs an area: never the whole world
		"source: opensky\n" + london, // not a source this module reads
		"source: receiver",           // a receiver needs its URL
		recv + "url: ftp://receiver.test/aircraft.json",
		"source: receiver\nurl: http://user:pass@receiver.test/aircraft.json",
		london + "\nbox: {south: 51, west: -1, north: 52, east: 0}",
		"area: {lat: 91, lon: 0, radius_nm: 10}",
		"area: {lat: 50, lon: 0, radius_nm: 0}",
		"area: {lat: 50, lon: 0, radius_nm: 251}",
		"box: {south: 52, west: -1, north: 51, east: 0}",
		"box: {south: 40, west: -10, north: 60, east: 10}", // wider than adsb.lol answers
		london + "\ninterval: 2s",                          // faster than adsb.lol's fair use
		recv + "interval: 200ms",
		recv + "timeout: 0s",
		recv + "expire: 1s",
		recv + "max_aircraft: 0",
		london + "\nsecret_env: AIR_TOKEN", // adsb.lol takes no key
	} {
		if _, err := parse(t, opts); err == nil {
			t.Errorf("%q accepted", opts)
		}
	}
}
