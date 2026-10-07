package aircraft

import (
	"context"
	"maps"
	"slices"
	"time"

	"wayseer.dev/sdk"
)

// historyPoints is how many reads each series keeps: half an hour at adsb.lol's default pace.
const historyPoints = 180

// The metrics, in the module's own names: the SDK has no unit for feet or knots.
const (
	metricAltitude = "aircraft.altitude"
	metricSpeed    = "aircraft.speed"
	metricCount    = "aircraft.count"
)

var catalogue = []sdk.Metric{
	{Name: metricAltitude, Description: "barometric altitude in feet; 0 on the ground", Kinds: []sdk.Kind{KindAircraft}, Native: "alt_baro"},
	{Name: metricSpeed, Description: "ground speed in knots", Kinds: []sdk.Kind{KindAircraft}, Native: "gs"},
	{Name: metricCount, Unit: sdk.UnitCount, Description: "aircraft in the area", Kinds: []sdk.Kind{KindArea}, Native: "aircraft in the working set"},
}

func inCatalogue(name string) (sdk.Metric, bool) {
	i := slices.IndexFunc(catalogue, func(m sdk.Metric) bool { return m.Name == name })
	if i < 0 {
		return sdk.Metric{}, false
	}
	return catalogue[i], true
}

// Metrics lists what QuerySeries can answer.
func (m *Module) Metrics() []sdk.Metric { return slices.Clone(catalogue) }

// record adds this read's values as points, and forgets series whose entity has gone.
// Callers hold m.mu.
func (m *Module) record(w *world, now time.Time) {
	for ref, v := range w.points {
		r := m.series[ref]
		if r == nil {
			r = sdk.NewRing(historyPoints)
			m.series[ref] = r
		}
		r.Add(sdk.Point{T: now.UnixNano(), V: v})
	}
	maps.DeleteFunc(m.series, func(ref sdk.SeriesRef, _ *sdk.Ring) bool {
		_, ok := w.ents[ref.Entity]
		return !ok
	})
}

// QuerySeries answers from the points recorded since Run started.
func (m *Module) QuerySeries(ctx context.Context, q sdk.SeriesQuery) ([]sdk.Series, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []sdk.Series
	for _, ref := range m.queried(q) {
		for _, name := range q.Metrics {
			key := sdk.SeriesRef{Entity: ref, Metric: name}
			mt, ok := inCatalogue(name)
			if r := m.series[key]; ok && r != nil {
				out = append(out, sdk.Series{Ref: key, Unit: mt.Unit, Points: r.In(q.Window)})
			}
		}
	}
	return out, nil
}

// queried is the entities q names, or else those its filter matches, in ref order.
func (m *Module) queried(q sdk.SeriesQuery) []sdk.EntityRef {
	if len(q.Entities) > 0 {
		return q.Entities
	}
	var out []sdk.EntityRef
	for _, ref := range slices.Sorted(maps.Keys(m.world.ents)) {
		e := m.world.ents[ref]
		if q.Filter.Match(&e) {
			out = append(out, ref)
		}
	}
	return out
}
