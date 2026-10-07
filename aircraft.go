package aircraft

import (
	"fmt"
	"regexp"
	"strings"

	"wayseer.dev/sdk"
)

// The kinds the module sends, all in its namespace.
const (
	KindAircraft sdk.Kind = "aircraft/aircraft"
	KindOperator sdk.Kind = "aircraft/operator"
	KindArea     sdk.Kind = "aircraft/area"
)

// adsbLolCredit is the attribution adsb.lol's ODbL licence asks for.
const adsbLolCredit = "adsb.lol contributors, ODbL 1.0"

// airline is an ICAO airline callsign: a three-letter designator, then a flight number.
var airline = regexp.MustCompile(`^([A-Z]{3})[0-9]`)

// alarms are the emergency squawks, with their status and meaning.
var alarms = map[string]sdk.Status{
	"7500": {Level: sdk.StatusCrit, Reason: "squawking 7500: unlawful interference"},
	"7600": {Level: sdk.StatusWarn, Reason: "squawking 7600: radio failure"},
	"7700": {Level: sdk.StatusCrit, Reason: "squawking 7700: emergency"},
}

// critical are the ADS-B emergency states as bad as a 7500 or 7700; any other is a warning.
var critical = map[string]bool{"general": true, "unlawful": true, "downed": true}

// world is the working set as entities and edges, keyed as sdk.Tracker wants them, with this
// read's metric values.
type world struct {
	ents   map[sdk.EntityRef]sdk.Entity
	edges  map[sdk.EdgeKey]sdk.Edge
	points map[sdk.SeriesRef]float64
}

// scene is what a world is built from: the aircraft and where they were sought.
type scene struct {
	src       sdk.ModuleID
	source    string
	area      area
	operators bool
	aircraft  []report
}

// buildWorld makes an entity for each aircraft, its operator, and the area with its count.
func buildWorld(s *scene) (world, error) {
	w := world{ents: map[sdk.EntityRef]sdk.Entity{}, edges: map[sdk.EdgeKey]sdk.Edge{}, points: map[sdk.SeriesRef]float64{}}
	ar, err := areaEntity(s, len(s.aircraft))
	if err != nil {
		return world{}, err
	}
	w.ents[ar.Ref] = ar
	w.points[sdk.SeriesRef{Entity: ar.Ref, Metric: metricCount}] = float64(len(s.aircraft))
	for i := range s.aircraft {
		if err := w.addAircraft(s, &s.aircraft[i]); err != nil {
			return world{}, err
		}
	}
	return w, nil
}

func (w *world) addAircraft(s *scene, r *report) error {
	e, err := aircraftEntity(s.src, r)
	if err != nil {
		return err
	}
	w.ents[e.Ref] = e
	if r.Alt.Known {
		w.points[sdk.SeriesRef{Entity: e.Ref, Metric: metricAltitude}] = r.Alt.Feet
	}
	if r.Speed != nil {
		w.points[sdk.SeriesRef{Entity: e.Ref, Metric: metricSpeed}] = *r.Speed
	}
	if code, ok := operatorOf(r.callsign()); ok && s.operators {
		return w.addOperator(s.src, e.Ref, code)
	}
	return nil
}

// addOperator adds the airline, once, and makes the aircraft its member.
func (w *world) addOperator(src sdk.ModuleID, plane sdk.EntityRef, code string) error {
	ref, err := sdk.NewEntityRef(string(src), KindOperator, code)
	if err != nil {
		return err
	}
	attrs := map[string]sdk.Value{"icao": sdk.String(code)}
	w.ents[ref] = sdk.Entity{Ref: ref, Kind: KindOperator, Name: code, Source: src, Status: sdk.Status{Level: sdk.StatusOK}, Attrs: attrs}
	e := sdk.Edge{From: plane, To: ref, Rel: sdk.RelMemberOf, Weight: 1, Source: src}
	w.edges[e.Key()] = e
	return nil
}

// operatorOf is the airline designator an airline callsign starts with.
func operatorOf(callsign string) (string, bool) {
	m := airline.FindStringSubmatch(callsign)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func aircraftEntity(src sdk.ModuleID, r *report) (sdk.Entity, error) {
	ref, err := sdk.NewEntityRef(string(src), KindAircraft, r.Hex)
	if err != nil {
		return sdk.Entity{}, fmt.Errorf("aircraft %q: %w", r.Hex, err)
	}
	e := sdk.Entity{Ref: ref, Kind: KindAircraft, Name: displayName(r), Status: status(r), Attrs: attrs(r), Source: src}
	if r.placed() {
		e.Place = sdk.At(float32(*r.Lat), float32(*r.Lon))
	}
	return e, nil
}

// displayName is the callsign, else the registration, else the address.
func displayName(r *report) string {
	for _, n := range []string{r.callsign(), strings.TrimSpace(r.Reg)} {
		if n != "" {
			return n
		}
	}
	return strings.ToUpper(strings.TrimPrefix(r.Hex, "~"))
}

// status is critical or a warning for an emergency squawk or ADS-B emergency, else OK.
func status(r *report) sdk.Status {
	if st, ok := alarms[r.Squawk]; ok {
		return st
	}
	switch em := r.Emergency; {
	case em == "" || em == "none":
		return sdk.Status{Level: sdk.StatusOK}
	case critical[em]:
		return sdk.Status{Level: sdk.StatusCrit, Reason: "declares an emergency: " + em}
	default:
		return sdk.Status{Level: sdk.StatusWarn, Reason: "declares an emergency: " + em}
	}
}

// attrs are what the report says, each only when it says it.
func attrs(r *report) map[string]sdk.Value {
	a := map[string]sdk.Value{"hex": sdk.String(r.Hex)}
	for k, v := range map[string]string{
		"callsign": r.callsign(), "registration": strings.TrimSpace(r.Reg), "type": r.Type,
		"category": r.Category, "squawk": r.Squawk, "emergency": r.Emergency,
	} {
		if v != "" && v != "none" {
			a[k] = sdk.String(v)
		}
	}
	if r.Alt.Known {
		a["altitude_ft"], a["on_ground"] = sdk.Number(r.Alt.Feet), sdk.Bool(r.Alt.Ground)
	}
	for k, v := range map[string]*float64{"speed_kt": r.Speed, "track_deg": r.Track, "vertical_rate_fpm": r.Rate} {
		if v != nil {
			a[k] = sdk.Number(*v)
		}
	}
	return a
}

// areaEntity is where aircraft are sought, at its centre, with how many are there.
func areaEntity(s *scene, n int) (sdk.Entity, error) {
	ref, err := sdk.NewEntityRef(string(s.src), KindArea, "area")
	if err != nil {
		return sdk.Entity{}, err
	}
	a := map[string]sdk.Value{"source": sdk.String(s.source), "aircraft": sdk.Number(float64(n))}
	if s.source == sourceADSBLol {
		a["data"] = sdk.String(adsbLolCredit)
	}
	e := sdk.Entity{Ref: ref, Kind: KindArea, Name: areaName(s), Status: sdk.Status{Level: sdk.StatusOK}, Attrs: a, Source: s.src}
	if s.area.set() {
		lat, lon, _ := s.area.query()
		e.Place = sdk.At(float32(lat), float32(lon))
	}
	return e, nil
}

// areaName says where aircraft are sought; a receiver with no area shows all it hears.
func areaName(s *scene) string {
	if !s.area.set() {
		return "all the receiver hears"
	}
	return s.area.String()
}
