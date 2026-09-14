package power

import (
	"testing"
)

func TestBatteryPolicyHysteresisAndUnknownSensors(t *testing.T) {
	p := BatteryPolicy{}
	for _, tc := range []struct {
		capacity, status                       string
		wasLow, low, external, valid, shutdown bool
	}{
		{"20", "Discharging", false, true, false, true, false},
		{"21", "Discharging", false, false, false, true, false},
		{"29", "Charging", true, true, true, true, false},
		{"30", "Charging", true, false, true, true, false},
		{"4", "Discharging", false, true, false, true, false},
		{"100", "Full", true, false, true, true, false},
		{"101", "Discharging", false, true, false, false, false},
		{"?", "Charging", true, true, false, false, false},
		{"80", "Unknown", false, true, false, false, false},
	} {
		d := p.Decide(&BatterySample{CapacityPct: tc.capacity, Status: tc.status}, tc.wasLow)
		if d.Low != tc.low || d.External != tc.external || d.Valid != tc.valid || d.Shutdown != tc.shutdown {
			t.Fatalf("%+v => %+v", tc, d)
		}
	}
	if d := p.Decide(nil, false); !d.Low || d.Shutdown {
		t.Fatal(d)
	}
	p.Shutdown = true
	if !p.Decide(&BatterySample{CapacityPct: "5", Status: "Discharging"}, false).Shutdown {
		t.Fatal("explicit critical policy ignored")
	}
	if p.Decide(&BatterySample{CapacityPct: "5", Status: "Charging"}, true).Shutdown {
		t.Fatal("charger caused shutdown")
	}
}
