package aircraft

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"wayseer.dev/sdk"
)

// The sources the module reads.
const (
	sourceADSBLol  = "adsb.lol" // the adsb.lol API, open data under ODbL 1.0
	sourceReceiver = "receiver" // a local readsb, tar1090 or dump1090-fa aircraft.json
)

// adsbLolURL is the adsb.lol API's base.
const adsbLolURL = "https://api.adsb.lol"

// The fastest each source is read: adsb.lol limits by load, so it is asked gently.
var minInterval = map[string]time.Duration{sourceADSBLol: 5 * time.Second, sourceReceiver: 500 * time.Millisecond}

var defaultInterval = map[string]time.Duration{sourceADSBLol: 30 * time.Second, sourceReceiver: 2 * time.Second}

type options struct {
	Source            string           `yaml:"source"`       // adsb.lol or receiver
	URL               string           `yaml:"url"`          // the receiver's aircraft.json, or the API's base
	Area              *circle          `yaml:"area"`         // a point and radius
	Box               *box             `yaml:"box"`          // or a latitude and longitude range
	Interval          time.Duration    `yaml:"interval"`     // how often it is read
	Timeout           time.Duration    `yaml:"timeout"`      // longest wait for one read
	Expire            time.Duration    `yaml:"expire"`       // how long an aircraft stays unheard
	MaxAircraft       int              `yaml:"max_aircraft"` // the most kept at once
	Operators         bool             `yaml:"operators"`    // show airlines from callsigns
	sdk.SecretOptions `yaml:",inline"` // a receiver's bearer token, if behind a proxy
}

func defaults() options {
	return options{Source: sourceADSBLol, Timeout: 10 * time.Second, Expire: time.Minute, MaxAircraft: 1000, Operators: true}
}

// readOptions decodes cfg over the defaults, fills what depends on the source, and checks it.
func readOptions(cfg sdk.Config) (options, error) {
	o := defaults()
	if err := cfg.Decode(&o); err != nil {
		return options{}, err
	}
	o.fill()
	if err := o.validate(); err != nil {
		return options{}, fmt.Errorf("line %d: %w", cfg.Line, err)
	}
	return o, nil
}

// fill sets the source's own defaults for what was left out.
func (o *options) fill() {
	if o.Interval == 0 {
		o.Interval = defaultInterval[o.Source]
	}
	if o.URL == "" && o.Source == sourceADSBLol {
		o.URL = adsbLolURL
	}
}

func (o *options) area() area { return area{circle: o.Area, box: o.Box} }

func (o *options) validate() error {
	floor, ok := minInterval[o.Source]
	switch {
	case !ok:
		return fmt.Errorf("source %q must be %s or %s", o.Source, sourceADSBLol, sourceReceiver)
	case o.Interval < floor:
		return fmt.Errorf("interval %v must be at least %v for %s", o.Interval, floor, o.Source)
	case o.Timeout <= 0 || o.Timeout > 5*time.Minute:
		return fmt.Errorf("timeout %v must be above 0 and at most 5m", o.Timeout)
	case o.Expire < 10*time.Second || o.Expire > time.Hour:
		return fmt.Errorf("expire %v must be from 10s to 1h", o.Expire)
	case o.MaxAircraft < 1 || o.MaxAircraft > 10000:
		return fmt.Errorf("max_aircraft %d must be from 1 to 10000", o.MaxAircraft)
	}
	return errors.Join(o.checkArea(), o.checkSecret(), checkURL(o.URL), o.Validate())
}

// checkArea allows one area at most, and requires one of a public API, never the whole world.
func (o *options) checkArea() error {
	switch {
	case o.Area != nil && o.Box != nil:
		return errors.New("give area or box, not both")
	case o.Area != nil:
		return o.Area.validate()
	case o.Box != nil:
		return o.Box.validate()
	case o.Source == sourceADSBLol:
		return errors.New("adsb.lol needs an area (lat, lon and radius_nm) or a box")
	}
	return nil
}

func (o *options) checkSecret() error {
	if o.Source == sourceADSBLol && o.Set() {
		return errors.New("adsb.lol takes no key; secret_file, secret_env and secret_keyring are for a receiver")
	}
	return nil
}

func checkURL(raw string) error {
	u, err := url.Parse(raw)
	switch {
	case raw == "":
		return errors.New("url is required for a receiver: its aircraft.json")
	case err != nil:
		return fmt.Errorf("url: %w", err)
	case u.Scheme != "http" && u.Scheme != "https" || u.Host == "":
		return fmt.Errorf("url %q must be http:// or https:// with a host", raw)
	case u.User != nil:
		return errors.New("url must not hold credentials; use secret_file, secret_env or secret_keyring")
	}
	return nil
}
