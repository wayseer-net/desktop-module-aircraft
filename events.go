package aircraft

import (
	"maps"
	"slices"
	"strconv"
	"time"

	"wayseer.dev/sdk"
)

// events are what happened between two worlds: each emergency that began or ended, and each
// aircraft that appeared or went. A nil old world is the first read, which says only emergencies.
func events(old, cur map[sdk.EntityRef]sdk.Entity, now time.Time) []sdk.Event {
	var out []sdk.Event
	for _, ref := range slices.Sorted(maps.Keys(cur)) {
		e := cur[ref]
		if e.Kind != KindAircraft {
			continue
		}
		was, seen := old[ref]
		if !seen && old != nil {
			out = append(out, event(&e, now, "appeared", sdk.SevDebug, e.Name+" appeared"))
		}
		if ev, ok := emergency(&was, &e, now); ok {
			out = append(out, ev)
		}
	}
	for _, ref := range slices.Sorted(maps.Keys(old)) {
		if e := old[ref]; e.Kind == KindAircraft && !has(cur, ref) {
			out = append(out, event(&e, now, "gone", sdk.SevDebug, e.Name+" is gone: unheard or out of the area"))
		}
	}
	return out
}

// emergency is the event for an emergency that began, changed or ended, if one did.
func emergency(was, e *sdk.Entity, now time.Time) (sdk.Event, bool) {
	switch {
	case e.Status.Reason == was.Status.Reason:
		return sdk.Event{}, false
	case e.Status.Level == sdk.StatusOK:
		return event(e, now, "emergency", sdk.SevInfo, e.Name+": emergency over"), true
	case e.Status.Level == sdk.StatusCrit:
		return event(e, now, "emergency", sdk.SevCritical, e.Name+": "+e.Status.Reason), true
	}
	return event(e, now, "emergency", sdk.SevWarn, e.Name+": "+e.Status.Reason), true
}

func event(e *sdk.Entity, at time.Time, kind string, sev sdk.Severity, msg string) sdk.Event {
	return sdk.Event{
		ID:     string(e.Ref) + "@" + kind + "@" + strconv.FormatInt(at.UnixNano(), 10),
		Entity: e.Ref, At: at, Severity: sev, Kind: kind, Message: msg, Source: e.Source,
	}
}

func has(m map[sdk.EntityRef]sdk.Entity, ref sdk.EntityRef) bool {
	_, ok := m[ref]
	return ok
}
