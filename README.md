# Wayseer module: aircraft

Live aircraft from ADS-B in Wayseer Desktop. Each aircraft appears on the Geo lens where it is
and moves on every read. It carries its callsign, registration, type, altitude, speed, track and
squawk. An emergency squawk turns it red and shows in Stream.

It is an external module, published in the Wayseer marketplace, and is not built into the app.
It reads one of two sources:

| Source | What it is | Terms |
|---|---|---|
| `adsb.lol` (default) | The [adsb.lol](https://adsb.lol) API: a community network's live data | Open data under [ODbL 1.0](https://opendatacommons.org/licenses/odbl/1-0/), "available to everyone", no key today |
| `receiver` | Your own readsb, tar1090 or dump1090-fa receiver's `aircraft.json` | Yours |

OpenSky Network, airplanes.live and ADSB.fi are not offered. Their terms allow only
non-commercial or personal use, and Wayseer's users include businesses.

Wayseer is a lens, not a store. The module keeps only aircraft heard within `expire`, and at most
`max_aircraft` of them. It never asks adsb.lol for the whole world: it needs an area.

## Configuration

Install the package with `>modules.install`, then name it in a config:

```yaml
modules:
  - kind: external
    name: sky
    options:
      module: wayseer-labs/aircraft
      options:
        area: {lat: 51.47, lon: -0.45, radius_nm: 40}   # around Heathrow
```

A local receiver, which needs no area:

```yaml
      options:
        source: receiver
        url: http://adsb-pi.local/tar1090/data/aircraft.json
```

| Option | Default | Meaning |
|---|---|---|
| `source` | `adsb.lol` | `adsb.lol` or `receiver`. |
| `url` | `https://api.adsb.lol` | The receiver's `aircraft.json`; for adsb.lol, the API's base (a mirror). |
| `area` | none | `lat`, `lon` and `radius_nm` (up to 250 nm). adsb.lol needs this or `box`. |
| `box` | none | `south`, `west`, `north` and `east` in degrees, not across the antimeridian, within 250 nm of its centre. adsb.lol is asked for the circle around it, and only aircraft inside the box are kept. |
| `interval` | `30s` for adsb.lol, `2s` for a receiver | How often the source is read: at least `5s` for adsb.lol, `500ms` for a receiver. |
| `timeout` | `10s` | The longest wait for one read. |
| `expire` | `1m` | How long an aircraft stays after it was last heard (10s to 1h). |
| `max_aircraft` | `1000` | The most aircraft kept; the least recently heard go first. |
| `operators` | `true` | Shows the airline an airline callsign names (`BAW123` belongs to `BAW`). |
| `secret_file`, `secret_env`, `secret_keyring` | none | A receiver's bearer token, for one behind an authenticating proxy. adsb.lol takes none. |

A secret in `secret_env` must also be listed in the entry's `env`. A `secret_keyring` on Linux
also needs `DBUS_SESSION_BUS_ADDRESS` there.

adsb.lol's limits rise and fall with its load. When it answers `429 Too Many Requests`, the module
waits for its `Retry-After`, or twice the interval, doubling with each limit in a row, and at most
five minutes. The error shows in the module's health until a read works. Requests name the module
in their `User-Agent`, and Health declares the interval as the module's pace, so Wayseer doesn't
call its data stale between reads.

## What it shows

| Kind | One per | Attributes |
|---|---|---|
| `aircraft/aircraft` | aircraft, by its 24-bit address (`hex`) | `callsign`, `registration`, `type`, `category`, `squawk`, `emergency`, `altitude_ft`, `on_ground`, `speed_kt`, `track_deg`, `vertical_rate_fpm` |
| `aircraft/operator` | airline designator in a callsign | `icao` |
| `aircraft/area` | instance: where aircraft are sought, placed at its centre | `source`, `aircraft` (the count), `data` (adsb.lol's attribution) |

An aircraft is named by its callsign, else its registration, else its address. Each is a
`member_of` its operator. Squawk 7500 or 7700, or an ADS-B emergency of `general`, `unlawful` or
`downed`, makes it critical; squawk 7600 or another emergency makes it a warning. The reason says
which. An aircraft with no position has no place, so the Geo lens counts it as "without a place".

**Series** (`>grid.show metric=`): `aircraft.altitude` (feet, 0 on the ground) and
`aircraft.speed` (knots) on each aircraft, and `aircraft.count` on the area. The SDK has no unit
for feet or knots, so they carry none, and the names and attributes say the unit.

**Events**: `emergency` when one begins (critical or warning) and when it ends (info), and
`appeared` and `gone` (debug) as aircraft come and go, so a busy area does not flood Stream.

## Data and attribution

Data from adsb.lol is © adsb.lol contributors and is made available under the Open Database
License 1.0. The area entity's `data` attribute carries this attribution in the app. The test
fixtures in `testdata` are made up and copy no real flight.

## Working on it

```
make check   # what CI runs: tests with the conformance suite, vet and lint for every platform, a key scan
make help    # every target
```

Tests never reach the network. They serve `testdata` from `httptest`, in both feeds' formats. It
imports only the SDK (`wayseer.dev/sdk`) and the standard library; `TestImportsOnlyTheSDK` keeps
it that way. `TestMakeSignPackagesTheModule` signs a package with throwaway keys when Wayseer's
source is in `../../core`, as in the Wayseer workspace, and skips otherwise.

For the marketplace's conformance run, serve `testdata` and use:

```yaml
options: |
  source: receiver
  url: http://127.0.0.1:8080/aircraft.json
failing: |
  source: receiver
  url: http://127.0.0.1:1/aircraft.json
fixture: testdata
```

## Licence

MIT; see `LICENSE`.
