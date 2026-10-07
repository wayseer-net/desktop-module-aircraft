package aircraft

import (
	"context"
	"slices"
	"testing"

	"wayseer.dev/sdk"
	"wayseer.dev/sdk/sdktest"
)

func plane(t *testing.T, hex, squawk string) sdk.Entity {
	t.Helper()
	e, err := aircraftEntity("air", &report{Hex: hex, Flight: "T" + hex, Squawk: squawk})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func ents(es ...sdk.Entity) map[sdk.EntityRef]sdk.Entity {
	m := map[sdk.EntityRef]sdk.Entity{}
	for _, e := range es {
		m[e.Ref] = e
	}
	return m
}

func summary(evs []sdk.Event) []string {
	var out []string
	for _, e := range evs {
		out = append(out, e.Kind+" "+e.Severity.String()+" "+e.Message)
	}
	return out
}

func TestTheFirstReadSaysOnlyEmergencies(t *testing.T) {
	got := summary(events(nil, ents(plane(t, "aaaaa1", "1200"), plane(t, "aaaaa2", "7700")), t0))
	want := []string{"emergency critical Taaaaa2: squawking 7700: emergency"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q", got)
	}
}

func TestAircraftAppearAndGo(t *testing.T) {
	old := ents(plane(t, "aaaaa1", "1200"), plane(t, "aaaaa2", "1200"))
	got := summary(events(old, ents(plane(t, "aaaaa2", "1200"), plane(t, "aaaaa3", "1200")), t0))
	want := []string{"appeared debug Taaaaa3 appeared", "gone debug Taaaaa1 is gone: unheard or out of the area"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q", got)
	}
}

func TestAnEmergencyIsSaidOnceAndWhenItEnds(t *testing.T) {
	ok, sq := plane(t, "aaaaa1", "1200"), plane(t, "aaaaa1", "7600")
	if got := summary(events(ents(ok), ents(sq), t0)); !slices.Equal(got, []string{"emergency warn Taaaaa1: squawking 7600: radio failure"}) {
		t.Errorf("began: %q", got)
	}
	if got := events(ents(sq), ents(sq), t0); len(got) != 0 {
		t.Errorf("still on: %q", summary(got))
	}
	if got := summary(events(ents(sq), ents(ok), t0)); !slices.Equal(got, []string{"emergency info Taaaaa1: emergency over"}) {
		t.Errorf("ended: %q", got)
	}
}

func TestEventIDsAreUnique(t *testing.T) {
	evs := events(ents(plane(t, "aaaaa1", "1200")), ents(plane(t, "aaaaa1", "7700"), plane(t, "aaaaa2", "7700")), t0)
	ids := map[string]bool{}
	for _, e := range evs {
		if ids[e.ID] || e.Source != "air" || !e.At.Equal(t0) {
			t.Errorf("event %+v", e)
		}
		ids[e.ID] = true
	}
}

func TestRunSendsAnEmergencyOnce(t *testing.T) {
	f, srv := serve(t, "testdata/aircraft.json")
	m := configured(t, receiver(srv, "interval: 500ms"))
	sink := sdktest.Run(t, func(ctx context.Context, s *sdktest.Sink) error { return m.Run(ctx, s) })
	sink.WaitFor(t, 1)
	f.set(`{"aircraft": [{"hex": "4ca9f1", "flight": "EIN12A", "squawk": "7500", "seen": 0}]}`)
	sink.WaitFor(t, 4)
	var hijack []string
	for _, e := range sink.Events() {
		if e.Entity == aircraftRef(t, "4ca9f1") && e.Kind == "emergency" {
			hijack = append(hijack, e.Message)
		}
	}
	if !slices.Equal(hijack, []string{"EIN12A: squawking 7500: unlawful interference"}) {
		t.Errorf("events %q", hijack)
	}
}
