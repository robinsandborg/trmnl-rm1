package trmnl

import (
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestDeployedRestoreMetadataSurvivesStateRoundTrip(t *testing.T) {
	p := isolatedPaths(t)
	original := []byte(`{"rendered_updates":1,"consecutive_failures":0,"last_cycle_changed":false,"masked_noise":{"chronyd.service":true,"memfaultd.service":false}}`)
	if err := os.WriteFile(p.StateFile, original, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := loadState(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveState(p, s); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p.StateFile)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["masked_noise"]) == "" {
		t.Fatal("deployed stock-service restore metadata lost")
	}
}

func TestCyclePreservesDeployedRestoreMetadata(t *testing.T) {
	h := newCycleHarness(t)
	// Simulate a deployed legacy file before its first metadata migration.
	for _, path := range []string{installationPath(h.paths), installationPath(h.paths) + ".bak"} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	h.before.MaskedNoise = map[string]bool{"chronyd.service": true, "memfaultd.service": false}
	h.seed()
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(h.state().MaskedNoise, h.before.MaskedNoise) {
		t.Fatal("cycle lost restore metadata")
	}
}

func TestInvalidConfigurationDoesNotEnumerateRadio(t *testing.T) {
	p := isolatedPaths(t)
	writeTestFile(t, p.ConfigFile, []byte(`{"base_url":":invalid","device_id":"explicit"}`))
	app := NewApp(io.Discard, io.Discard)
	called := false
	app.cycle.ensureInterface = func(Config) { called = true }
	err := app.Run([]string{"run-once"})
	if called || err == nil || !strings.Contains(err.Error(), "base_url must be a valid absolute URL") {
		t.Fatalf("ensure=%v error=%v", called, err)
	}
}
