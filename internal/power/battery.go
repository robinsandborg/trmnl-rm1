package power

import (
	"strconv"
	"strings"
	"time"
)

// BatteryPolicy defaults are conservative provisional values, not calibrated
// fuel-gauge estimates. A critical shutdown must be explicitly enabled.
type BatteryPolicy struct {
	Low, Recovery, Critical int
	CheckInterval           time.Duration
	Shutdown                bool
}

func (p BatteryPolicy) Defaults() BatteryPolicy {
	if p.Low == 0 {
		p.Low = 20
	}
	if p.Recovery == 0 {
		p.Recovery = 30
	}
	if p.Critical == 0 {
		p.Critical = 5
	}
	if p.CheckInterval == 0 {
		p.CheckInterval = 30 * time.Minute
	}
	return p
}

type BatteryDecision struct {
	Low, Shutdown, External, Valid bool
	Reason                         string
}

func (p BatteryPolicy) Decide(s *BatterySample, wasLow bool) BatteryDecision {
	p = p.Defaults()
	d := BatteryDecision{Low: true, Reason: "battery-unavailable"}
	if s == nil {
		return d
	}
	n, err := strconv.Atoi(s.CapacityPct)
	if err != nil || n < 0 || n > 100 {
		return d
	}
	status := strings.ToLower(strings.TrimSpace(s.Status))
	d.External = status == "charging" || status == "full"
	if !d.External && status != "discharging" {
		return d
	}
	d.Valid = true
	if wasLow {
		d.Low = n < p.Recovery
	} else {
		d.Low = n <= p.Low
	}
	d.Shutdown = d.Low && !d.External && n <= p.Critical && p.Shutdown
	switch {
	case d.Shutdown:
		d.Reason = "critical-battery"
	case d.Low && d.External:
		d.Reason = "charging-recovery"
	case d.Low:
		d.Reason = "low-battery"
	default:
		d.Reason = "battery-adequate"
	}
	return d
}
