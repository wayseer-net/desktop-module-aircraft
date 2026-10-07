package aircraft

import (
	"testing"

	"wayseer.dev/sdk"
)

func TestEmergenciesSetTheStatus(t *testing.T) {
	for _, c := range []struct {
		squawk, emergency string
		want              sdk.StatusLevel
	}{
		{"7500", "", sdk.StatusCrit},
		{"7600", "", sdk.StatusWarn},
		{"7700", "none", sdk.StatusCrit},
		{"1200", "minfuel", sdk.StatusWarn},
		{"1200", "unlawful", sdk.StatusCrit},
		{"1200", "none", sdk.StatusOK},
		{"", "", sdk.StatusOK},
	} {
		st := status(&report{Squawk: c.squawk, Emergency: c.emergency})
		if st.Level != c.want || (st.Level != sdk.StatusOK) != (st.Reason != "") {
			t.Errorf("squawk %q, emergency %q: %+v", c.squawk, c.emergency, st)
		}
	}
}

func TestOperatorsComeFromAirlineCallsigns(t *testing.T) {
	for callsign, want := range map[string]string{"EIN12A": "EIN", "BAW7XY": "BAW", "N123AB": "", "GXAAB": "", "": ""} {
		if got, ok := operatorOf(callsign); got != want || ok != (want != "") {
			t.Errorf("%q: %q", callsign, got)
		}
	}
}

func TestAnAircraftIsNamedByCallsignThenRegistrationThenAddress(t *testing.T) {
	for _, c := range []struct {
		r    report
		want string
	}{
		{report{Hex: "4ca9f1", Flight: "EIN12A  ", Reg: "EI-XAA"}, "EIN12A"},
		{report{Hex: "4ca9f1", Reg: "EI-XAA"}, "EI-XAA"},
		{report{Hex: "~2a0f00"}, "2A0F00"},
	} {
		if got := displayName(&c.r); got != c.want {
			t.Errorf("%+v: %q", c.r, got)
		}
	}
}
