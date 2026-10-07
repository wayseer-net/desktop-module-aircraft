package aircraft

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"wayseer.dev/sdk"
)

// Kind is the module kind in config.
const Kind = "aircraft"

const version = "1"

// retryMax is the longest wait before retrying a failed read; limitMax is the longest wait a
// rate limit is given.
const (
	retryMax = 10 * time.Second
	limitMax = 5 * time.Minute
)

// init registers the kind for a build of the app that imports the package.
func init() { sdk.Register(Kind, func() sdk.Module { return New() }) }

// Module reads the source every interval and sends what changed.
type Module struct {
	health atomic.Pointer[sdk.Health]

	mu      sync.Mutex // guards what follows, shared by Run, Discover and QuerySeries
	name    sdk.ModuleID
	opts    options
	src     *source
	tracker sdk.Tracker
	fleet   *fleet
	world   world                       // as last sent
	series  map[sdk.SeriesRef]*sdk.Ring // recorded by Run
}

// New makes an unconfigured module.
func New() *Module { return &Module{} }

// Info describes the module.
func (m *Module) Info() sdk.Info {
	return sdk.Info{Kind: Kind, Version: version, Description: "Live aircraft from ADS-B, from adsb.lol or a local receiver"}
}

// Configure checks the options and reads the token; nothing is fetched until Run or Discover.
func (m *Module) Configure(_ context.Context, cfg sdk.Config) error {
	o, err := readOptions(cfg)
	if err != nil {
		return err
	}
	token, err := o.Read()
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.name, m.opts, m.src = cfg.Name, o, newSource(&o, token)
	m.world, m.series = world{}, map[sdk.SeriesRef]*sdk.Ring{}
	m.health.Store(&sdk.Health{})
	return nil
}

// Run reads the source every interval: a snapshot after the first good read, then deltas.
// A failed read shows in Health and is retried; Run returns only when ctx ends.
func (m *Module) Run(ctx context.Context, sink sdk.Sink) error {
	m.mu.Lock()
	m.tracker.Reset()
	m.fleet = m.newFleet()
	every := m.opts.Interval
	m.mu.Unlock()
	send := sink.Snapshot
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
		cs, err := m.refresh(ctx)
		if ctx.Err() != nil {
			return nil
		}
		m.health.Store(&sdk.Health{Err: err})
		if err != nil {
			t.Reset(retryAfter(err, every))
			continue
		}
		if err := send(ctx, cs); err != nil {
			return err
		}
		send = sink.Delta
		t.Reset(every)
	}
}

// retryAfter is how long to wait after a failed read: sooner, unless the source is limiting.
func retryAfter(err error, every time.Duration) time.Duration {
	if wait, ok := errLimited(err); ok {
		return min(max(wait, 2*every), limitMax)
	}
	return min(every, retryMax)
}

func (m *Module) newFleet() *fleet { return newFleet(m.opts.area(), m.opts.Expire, m.opts.MaxAircraft) }

// refresh reads the source into the working set, records its metrics, and returns what
// changed since the last send, with what happened.
func (m *Module) refresh(ctx context.Context) (*sdk.ChangeSet, error) {
	rs, err := m.src.read(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fleet.update(rs, now)
	w, err := buildWorld(m.scene(m.fleet))
	if err != nil {
		return nil, err
	}
	evs := events(m.world.ents, w.ents, now)
	m.world = w
	m.record(&w, now)
	cs := m.tracker.Changes(w.ents, w.edges, now)
	cs.Events = evs
	return cs, nil
}

// scene is what to build a world from the fleet. Callers hold m.mu.
func (m *Module) scene(f *fleet) *scene {
	return &scene{src: m.name, source: m.opts.Source, area: m.opts.area(), operators: m.opts.Operators, aircraft: f.aircraft()}
}

// Health reports whether the last read worked.
func (m *Module) Health() sdk.Health {
	if h := m.health.Load(); h != nil {
		return *h
	}
	return sdk.Health{}
}

// Discover returns what Run last read, so the source is not asked more often than interval;
// before Run has read, it reads the source now.
func (m *Module) Discover(ctx context.Context) (*sdk.ChangeSet, error) {
	m.mu.Lock()
	w, src, f := m.world, m.src, m.newFleet()
	m.mu.Unlock()
	if w.ents == nil {
		rs, err := src.read(ctx)
		if err != nil {
			return nil, err
		}
		f.update(rs, time.Now())
		m.mu.Lock()
		s := m.scene(f)
		m.mu.Unlock()
		if w, err = buildWorld(s); err != nil {
			return nil, err
		}
	}
	var fresh sdk.Tracker
	return fresh.Changes(w.ents, w.edges, time.Now()), nil
}
