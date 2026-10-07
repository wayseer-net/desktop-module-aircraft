package aircraft

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

// earthRadiusNM is the Earth's mean radius in nautical miles.
const earthRadiusNM = 3440.065

// maxRadiusNM is the widest circle adsb.lol answers for.
const maxRadiusNM = 250

// circle is a point and a radius around it.
type circle struct {
	Lat      float64 `yaml:"lat"`
	Lon      float64 `yaml:"lon"`
	RadiusNM float64 `yaml:"radius_nm"`
}

// box is a latitude and longitude range; it may not cross the antimeridian.
type box struct {
	South float64 `yaml:"south"`
	West  float64 `yaml:"west"`
	North float64 `yaml:"north"`
	East  float64 `yaml:"east"`
}

// area is where aircraft are shown: a circle, a box, or, unset, everywhere.
type area struct {
	circle *circle
	box    *box
}

func (a area) set() bool { return a.circle != nil || a.box != nil }

// query is the circle that covers the area, for a source that asks by point and radius.
func (a area) query() (lat, lon, radiusNM float64) {
	switch {
	case a.circle != nil:
		return a.circle.Lat, a.circle.Lon, a.circle.RadiusNM
	case a.box != nil:
		return a.box.around()
	}
	return 0, 0, 0
}

// contains reports whether a position is in the area.
func (a area) contains(lat, lon float64) bool {
	switch {
	case a.circle != nil:
		return distanceNM(a.circle.Lat, a.circle.Lon, lat, lon) <= a.circle.RadiusNM
	case a.box != nil:
		b := a.box
		return lat >= b.South && lat <= b.North && lon >= b.West && lon <= b.East
	}
	return true
}

func (a area) String() string {
	switch {
	case a.circle != nil:
		c := a.circle
		return num(c.RadiusNM) + " nm around " + num(c.Lat) + ", " + num(c.Lon)
	case a.box != nil:
		b := a.box
		return num(b.South) + ", " + num(b.West) + " to " + num(b.North) + ", " + num(b.East)
	}
	return "everywhere"
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// around is the box's centre and the distance to its farthest corner.
func (b *box) around() (lat, lon, radiusNM float64) {
	lat, lon = (b.South+b.North)/2, (b.West+b.East)/2
	for _, y := range []float64{b.South, b.North} {
		for _, x := range []float64{b.West, b.East} {
			radiusNM = max(radiusNM, distanceNM(lat, lon, y, x))
		}
	}
	return lat, lon, radiusNM
}

func (c *circle) validate() error {
	if err := checkPoint(c.Lat, c.Lon); err != nil {
		return fmt.Errorf("area: %w", err)
	}
	if c.RadiusNM <= 0 || c.RadiusNM > maxRadiusNM {
		return fmt.Errorf("area: radius_nm %v must be above 0 and at most %d", c.RadiusNM, maxRadiusNM)
	}
	return nil
}

func (b *box) validate() error {
	if err := errors.Join(checkPoint(b.South, b.West), checkPoint(b.North, b.East)); err != nil {
		return fmt.Errorf("box: %w", err)
	}
	if b.South >= b.North || b.West >= b.East {
		return errors.New("box: south must be below north and west below east")
	}
	if _, _, r := b.around(); r > maxRadiusNM {
		return fmt.Errorf("box: it spans %.0f nm from its centre; at most %d", r, maxRadiusNM)
	}
	return nil
}

func checkPoint(lat, lon float64) error {
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 || math.IsNaN(lat+lon) {
		return fmt.Errorf("%v, %v is not a latitude and longitude", lat, lon)
	}
	return nil
}

// distanceNM is the great-circle distance between two points, by the haversine formula.
func distanceNM(lat1, lon1, lat2, lon2 float64) float64 {
	rad := math.Pi / 180
	dLat, dLon := (lat2-lat1)*rad, (lon2-lon1)*rad
	h := math.Pow(math.Sin(dLat/2), 2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Pow(math.Sin(dLon/2), 2)
	return 2 * earthRadiusNM * math.Asin(math.Sqrt(min(h, 1)))
}
