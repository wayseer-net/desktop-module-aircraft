package aircraft_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/wayseer-net/desktop-module-aircraft"
	"wayseer.dev/sdk/manifest"
)

// TestManifestDeclaresTheModule keeps manifest.yaml in step with the code: the kinds it sends,
// and no actions, for it changes nothing.
func TestManifestDeclaresTheModule(t *testing.T) {
	m := readManifest(t)
	if m.ID != "wayseer-labs/aircraft" || m.Namespace != "aircraft" || len(m.Actions) != 0 {
		t.Errorf("manifest.yaml is %s in %s with %d actions", m.ID, m.Namespace, len(m.Actions))
	}
	for _, k := range []string{string(aircraft.KindAircraft), string(aircraft.KindOperator), string(aircraft.KindArea)} {
		if !slices.ContainsFunc(m.Kinds, func(d manifest.Kind) bool { return d.Kind == k }) {
			t.Errorf("manifest.yaml doesn't declare %s", k)
		}
	}
}

// readManifest reads manifest.yaml as dev sign does, filling the fields it writes.
func readManifest(t *testing.T) manifest.Manifest {
	t.Helper()
	data, err := os.ReadFile("manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, m, err := manifest.Fill(data, manifest.Platform{OS: "linux", Arch: "amd64", SHA256: strings.Repeat("0", 64)})
	if err != nil {
		t.Fatal(err)
	}
	return m
}
