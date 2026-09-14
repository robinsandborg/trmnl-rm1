// Package diagnostics interprets existing cycle logs without device effects.
package diagnostics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Filter selects records by cycle start, with an inclusive Since and exclusive Until.
type Filter struct{ Since, Until time.Time }

// Only diagnostic fields are decoded. URLs, identity and error messages are never
// included in reports, including when an input record is malformed.
type record struct {
	Start    time.Time `json:"started_at"`
	End      time.Time `json:"ended_at"`
	Mode     string    `json:"mode"`
	Reason   string    `json:"maintenance_reason"`
	Failure  string    `json:"failure_category"`
	Interval int       `json:"refresh_interval_seconds"`
	Changed  bool      `json:"changed_screen"`
	Skipped  bool      `json:"skipped_render"`
	Full     bool      `json:"full_refresh"`
	Battery  *struct {
		Status   string `json:"status"`
		Capacity string `json:"capacity_pct"`
	} `json:"battery"`
}

type Distribution struct {
	Min    float64 `json:"min"`
	Median float64 `json:"median"`
	P95    float64 `json:"p95"`
	Max    float64 `json:"max"`
}

// DischargeWindow links adjacent, monotonically ordered discharging samples.
// Status between samples is unknown; this is not a continuous power measurement.
type DischargeWindow struct {
	Start         time.Time `json:"start"`
	End           time.Time `json:"end"`
	Samples       int       `json:"samples"`
	StartCapacity int       `json:"start_capacity_pct"`
	EndCapacity   int       `json:"end_capacity_pct"`
	Drop          int       `json:"drop_percentage_points"`
	Hours         float64   `json:"elapsed_hours"`
}

type Report struct {
	Version             int               `json:"schema_version"`
	Records             int               `json:"selected_records"`
	Invalid             int               `json:"invalid_records_in_input"`
	Duplicates          int               `json:"duplicate_cycle_records"`
	Cycles              int               `json:"unique_cycles"`
	Start               *time.Time        `json:"first_cycle_start,omitempty"`
	End                 *time.Time        `json:"last_record_end,omitempty"`
	RecordedWorkSeconds float64           `json:"recorded_work_seconds"`
	Duration            *Distribution     `json:"recorded_cycle_seconds,omitempty"`
	Interval            *Distribution     `json:"requested_interval_seconds,omitempty"`
	Modes               map[string]int    `json:"modes"`
	Reasons             map[string]int    `json:"maintenance_reasons"`
	Failures            map[string]int    `json:"failure_categories"`
	BatteryStatuses     map[string]int    `json:"battery_status_samples"`
	Changed             int               `json:"changed_screen_cycles"`
	Skipped             int               `json:"skipped_render_cycles"`
	Full                int               `json:"full_refresh_cycles"`
	Discharge           []DischargeWindow `json:"sampled_discharge_windows"`
	Limitations         []string          `json:"limitations"`
}

// Read retains the last record for each start time: a suspend failure can append
// a second record after a successful cycle log. This prevents double counting.
func Read(input io.Reader, filter Filter) (Report, error) {
	r := Report{Version: 1, Modes: map[string]int{}, Reasons: map[string]int{}, Failures: map[string]int{}, BatteryStatuses: map[string]int{}, Discharge: []DischargeWindow{}, Limitations: []string{
		"Logs record intent, not actual suspend residency. Check the suspend journal separately.",
		"Cycle timing omits startup, cleanup and time awake between cycles; it is not total awake time.",
		"Discharging status is sampled only at cycle start. Charging, reboots or missing cycles between samples may be invisible.",
		"Battery percentage is coarse and gauge-dependent. No battery-life projection or energy saving is inferred.",
	}}
	if !filter.Since.IsZero() && !filter.Until.IsZero() && !filter.Since.Before(filter.Until) {
		return r, fmt.Errorf("since must precede until")
	}
	var rows []record
	indices := map[time.Time]int{}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		if len(strings.TrimSpace(scanner.Text())) == 0 {
			continue
		}
		var row record
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil || row.Start.IsZero() || row.End.IsZero() || row.End.Before(row.Start) {
			r.Invalid++
			rows = append(rows, record{}) // Do not bridge an unknown battery observation.
			continue
		}
		row.Start, row.End = row.Start.UTC(), row.End.UTC()
		if !filter.Since.IsZero() && row.Start.Before(filter.Since) || !filter.Until.IsZero() && !row.Start.Before(filter.Until) {
			rows = append(rows, record{}) // Filtering must not hide a continuity break.
			continue
		}
		r.Records++
		if i, ok := indices[row.Start]; ok {
			rows[i] = row
			r.Duplicates++
		} else {
			indices[row.Start] = len(rows)
			rows = append(rows, row)
		}
	}
	if scanner.Err() != nil {
		return r, fmt.Errorf("read cycle log failed (maximum record size 1 MiB)")
	}
	r.Cycles = len(indices)
	var durations, intervals []float64
	var window *DischargeWindow
	flush := func() {
		if window != nil && window.Samples >= 2 {
			window.Drop = window.StartCapacity - window.EndCapacity
			window.Hours = window.End.Sub(window.Start).Hours()
			r.Discharge = append(r.Discharge, *window)
		}
		window = nil
	}
	for _, row := range rows {
		if row.Start.IsZero() {
			flush()
			continue
		}
		if r.Start == nil || row.Start.Before(*r.Start) {
			start := row.Start
			r.Start = &start
		}
		if r.End == nil || row.End.After(*r.End) {
			end := row.End
			r.End = &end
		}
		d := row.End.Sub(row.Start).Seconds()
		durations = append(durations, d)
		r.RecordedWorkSeconds += d
		if row.Interval > 0 {
			intervals = append(intervals, float64(row.Interval))
		}
		r.Modes[label(row.Mode, "appliance", "maintenance", "boot-grace", "recovery", "awake-fallback", "low-battery", "critical-battery")]++
		if row.Reason != "" {
			r.Reasons[label(row.Reason, "sentinel-file", "usb-network", "boot-grace", "failure-threshold", "rtc-fallback", "low-battery", "charging-recovery", "critical-battery", "battery-unavailable", "battery-adequate", "bounded-retry", "battery-retry")]++
		}
		if row.Failure != "" {
			r.Failures[label(row.Failure, "wifi", "http", "render", "schedule", "suspend", "battery", "state", "config", "startup")]++
		}
		if row.Changed {
			r.Changed++
		}
		if row.Skipped {
			r.Skipped++
		}
		if row.Full {
			r.Full++
		}
		capacity := -1
		status := "missing"
		if row.Battery != nil {
			status = label(strings.ToLower(row.Battery.Status), "charging", "discharging", "full", "not charging", "unknown")
		}
		r.BatteryStatuses[status]++
		if row.Battery != nil && strings.EqualFold(row.Battery.Status, "Discharging") {
			if n, err := strconv.Atoi(row.Battery.Capacity); err == nil && n >= 0 && n <= 100 {
				capacity = n
			}
		}
		if capacity < 0 {
			flush()
			continue
		}
		if window != nil && (!row.Start.After(window.End) || capacity > window.EndCapacity) {
			flush()
		}
		if window == nil {
			window = &DischargeWindow{Start: row.Start, StartCapacity: capacity}
		}
		window.End, window.EndCapacity = row.Start, capacity
		window.Samples++
	}
	flush()
	r.Duration, r.Interval = distribution(durations), distribution(intervals)
	return r, nil
}

func label(value string, known ...string) string {
	for _, name := range known {
		if value == name {
			return name
		}
	}
	return "other"
}

func distribution(values []float64) *Distribution {
	if len(values) == 0 {
		return nil
	}
	sort.Float64s(values)
	mid := len(values) / 2
	median := values[mid]
	if len(values)%2 == 0 {
		median = (values[mid-1] + values[mid]) / 2
	}
	return &Distribution{Min: values[0], Median: median, P95: values[int(math.Ceil(float64(len(values))*0.95))-1], Max: values[len(values)-1]}
}
