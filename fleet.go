package aircraft

import (
	"cmp"
	"maps"
	"slices"
	"time"
)

// fleet is the working set: each aircraft as last heard, kept until unheard for expire, and
// never more than limit of them.
type fleet struct {
	area   area
	expire time.Duration
	limit  int
	byHex  map[string]sighting
}

// sighting is a report and when the aircraft was last heard.
type sighting struct {
	report
	heard time.Time
}

func newFleet(a area, expire time.Duration, limit int) *fleet {
	return &fleet{area: a, expire: expire, limit: limit, byHex: map[string]sighting{}}
}

// update takes one read's reports at now, then drops what has expired or does not fit.
func (f *fleet) update(rs []report, now time.Time) {
	for _, r := range rs {
		f.admit(r, now.Add(-time.Duration(r.Seen*float64(time.Second))))
	}
	maps.DeleteFunc(f.byHex, func(_ string, s sighting) bool { return now.Sub(s.heard) > f.expire })
	f.bound()
}

// admit keeps r if it is newer than what is held; an aircraft outside the area goes at once.
func (f *fleet) admit(r report, heard time.Time) {
	if f.area.set() && (!r.placed() || !f.area.contains(*r.Lat, *r.Lon)) {
		delete(f.byHex, r.Hex)
		return
	}
	if old, ok := f.byHex[r.Hex]; !ok || heard.After(old.heard) {
		f.byHex[r.Hex] = sighting{report: r, heard: heard}
	}
}

// bound drops the least recently heard beyond the limit.
func (f *fleet) bound() {
	if len(f.byHex) <= f.limit {
		return
	}
	all := slices.SortedFunc(maps.Values(f.byHex), func(a, b sighting) int {
		return cmp.Or(b.heard.Compare(a.heard), cmp.Compare(a.Hex, b.Hex))
	})
	for _, s := range all[f.limit:] {
		delete(f.byHex, s.Hex)
	}
}

// aircraft is the working set's reports, by address.
func (f *fleet) aircraft() []report {
	out := make([]report, 0, len(f.byHex))
	for _, hex := range slices.Sorted(maps.Keys(f.byHex)) {
		out = append(out, f.byHex[hex].report)
	}
	return out
}
