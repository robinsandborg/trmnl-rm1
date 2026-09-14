package diagnostics

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func line(hour, capacity int, status string) string {
	return fmt.Sprintf(`{"started_at":"2026-09-14T%02d:00:00Z","ended_at":"2026-09-14T%02d:00:20Z","mode":"appliance","refresh_interval_seconds":3600,"skipped_render":true,"battery":{"status":%q,"capacity_pct":%q}}`+"\n", hour, hour, status, fmt.Sprint(capacity))
}

func TestReportSummarizesWithoutLeakingPayloads(t *testing.T) {
	input := line(0, 90, "Discharging") + line(1, 88, "Discharging")
	input += `{"started_at":"2026-09-14T02:00:00Z","ended_at":"2026-09-14T02:01:00Z","mode":"maintenance","maintenance_reason":"usb-network","failure_category":"http","image_url":"https://secret.example/token","failure_message":"private API key","battery":{"status":"Charging","capacity_pct":"95"}}` + "\n"
	input += `{"started_at":"2026-09-14T03:00:00Z","ended_at":"2026-09-14T03:00:10Z","mode":"secret mode","maintenance_reason":"private reason","failure_category":"private failure"}` + "\n"
	r, err := Read(strings.NewReader(input), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Cycles != 4 || r.RecordedWorkSeconds != 110 || r.Duration.Median != 20 || r.Duration.P95 != 60 || r.Interval.Min != 3600 {
		t.Fatalf("unexpected summary: %+v", r)
	}
	if r.Modes["maintenance"] != 1 || r.Reasons["usb-network"] != 1 || r.Failures["http"] != 1 || r.Skipped != 2 {
		t.Fatalf("unexpected counts: %+v", r)
	}
	if len(r.Discharge) != 1 || r.Discharge[0].Drop != 2 || r.Discharge[0].Hours != 1 || r.Discharge[0].Samples != 2 {
		t.Fatalf("unexpected discharge: %+v", r.Discharge)
	}
	encoded, _ := json.Marshal(r)
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "private") {
		t.Fatalf("report leaked input: %s", encoded)
	}
}

func TestDischargeWindowsBreakOnUncertainObservations(t *testing.T) {
	for name, middle := range map[string]string{
		"charging":             line(2, 98, "Charging"),
		"unknown":              line(2, 87, "Unknown"),
		"missing":              `{"started_at":"2026-09-14T02:00:00Z","ended_at":"2026-09-14T02:00:20Z"}` + "\n",
		"invalid capacity":     line(2, -1, "Discharging"),
		"malformed":            "{broken\n",
		"clock went backwards": line(0, 86, "Discharging"),
	} {
		t.Run(name, func(t *testing.T) {
			// The backward timestamp is distinct from the first two samples.
			input := line(1, 90, "Discharging") + middle + line(3, 85, "Discharging")
			r, err := Read(strings.NewReader(input), Filter{})
			if err != nil {
				t.Fatal(err)
			}
			if name == "clock went backwards" {
				if len(r.Discharge) != 1 || r.Discharge[0].Start.Hour() != 0 {
					t.Fatalf("clock window crossed reset: %+v", r.Discharge)
				}
			} else if len(r.Discharge) != 0 {
				t.Fatalf("bridged unknown sample: %+v", r.Discharge)
			}
		})
	}
}

func TestIncreaseStartsNewWindowAndPlateauIsRetained(t *testing.T) {
	r, err := Read(strings.NewReader(line(0, 90, "Discharging")+line(1, 90, "Discharging")+line(2, 95, "Discharging")+line(3, 93, "Discharging")), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Discharge) != 2 || r.Discharge[0].Drop != 0 || r.Discharge[1].Drop != 2 {
		t.Fatalf("unexpected windows: %+v", r.Discharge)
	}
}

func TestDuplicateUsesLastOutcomeWithoutDoubleCounting(t *testing.T) {
	input := line(0, 90, "Discharging")
	input += `{"started_at":"2026-09-14T00:00:00Z","ended_at":"2026-09-14T00:00:25Z","mode":"appliance","failure_category":"suspend"}` + "\n"
	r, err := Read(strings.NewReader(input), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Records != 2 || r.Cycles != 1 || r.Duplicates != 1 || r.RecordedWorkSeconds != 25 || r.Failures["suspend"] != 1 || r.Skipped != 0 {
		t.Fatalf("unexpected summary: %+v", r)
	}
}

func TestTimeFilterAndInvalidInput(t *testing.T) {
	since, _ := time.Parse(time.RFC3339, "2026-09-14T01:00:00Z")
	until := since.Add(time.Hour)
	r, err := Read(strings.NewReader(line(0, 90, "Discharging")+line(1, 88, "Discharging")+line(2, 87, "Discharging")+"{}\nnull\n{broken\n"), Filter{Since: since, Until: until})
	if err != nil {
		t.Fatal(err)
	}
	if r.Cycles != 1 || r.Invalid != 3 || !r.Start.Equal(since) {
		t.Fatalf("unexpected selection: %+v", r)
	}
	if _, err := Read(strings.NewReader(""), Filter{Since: until, Until: since}); err == nil {
		t.Fatal("reversed filter accepted")
	}
	if _, err := Read(strings.NewReader(strings.Repeat("x", 1024*1024+1)), Filter{}); err == nil {
		t.Fatal("oversized record accepted")
	}
}

func TestEmptyReportAndInvalidTime(t *testing.T) {
	r, err := Read(strings.NewReader("\n"), Filter{})
	if err != nil || r.Cycles != 0 || r.Duration != nil || r.Start != nil {
		t.Fatalf("unexpected empty report: %+v, %v", r, err)
	}
	r, err = Read(strings.NewReader(`{"started_at":"2026-09-14T01:00:00Z","ended_at":"2026-09-14T00:00:00Z"}`), Filter{})
	if err != nil || r.Invalid != 1 {
		t.Fatalf("negative duration accepted: %+v, %v", r, err)
	}
}

func TestRecoveryLabelsRemainDiagnostic(t *testing.T) {
	for _, reason := range []string{"charging-recovery", "critical-battery", "battery-unavailable", "battery-adequate", "bounded-retry", "battery-retry", "low-battery"} {
		input := fmt.Sprintf(`{"started_at":"2026-09-14T00:00:00Z","ended_at":"2026-09-14T00:00:20Z","mode":"recovery","maintenance_reason":%q,"failure_category":"startup"}`, reason)
		r, err := Read(strings.NewReader(input), Filter{})
		if err != nil || r.Reasons[reason] != 1 || r.Failures["startup"] != 1 {
			t.Fatalf("lost recovery labels: %+v, %v", r, err)
		}
	}
}

func TestExcludedRowsBreakDischargeContinuity(t *testing.T) {
	since, _ := time.Parse(time.RFC3339, "2026-09-14T01:00:00Z")
	until, _ := time.Parse(time.RFC3339, "2026-09-14T03:00:00Z")
	for _, excludedHour := range []int{0, 3} {
		input := line(1, 90, "Discharging") + line(excludedHour, 95, "Charging") + line(2, 89, "Discharging")
		r, err := Read(strings.NewReader(input), Filter{Since: since, Until: until})
		if err != nil {
			t.Fatal(err)
		}
		if r.Records != 2 || r.Cycles != 2 || len(r.Discharge) != 0 {
			t.Fatalf("bridged excluded charging record at hour %d: %+v", excludedHour, r)
		}
	}
}
