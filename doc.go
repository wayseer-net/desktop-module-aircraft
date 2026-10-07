// Package aircraft shows live aircraft from ADS-B in Wayseer: each aircraft on the map where it
// is, with its callsign, altitude and speed, and the airline its callsign names.
//
// It reads the adsb.lol API (open data under ODbL 1.0) for a point and radius or a box, or a
// local readsb, tar1090 or dump1090-fa receiver's aircraft.json. It is a lens, not a store: it
// keeps only the aircraft heard within expire, at most max_aircraft of them. An empty feed is
// an empty sky, not an error.
package aircraft
