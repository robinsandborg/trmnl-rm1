package byos

import "time"

// RefreshPolicy holds effective durations supplied by the application.
// Interval falls back for nonpositive rates and applies min then max, matching
// the existing appliance policy even when bounds are inconsistent.
type RefreshPolicy struct {
	Fallback time.Duration
	Min      time.Duration
	Max      time.Duration
}

func (p RefreshPolicy) Interval(refreshRate int) time.Duration {
	interval := p.Fallback
	if refreshRate > 0 {
		interval = time.Duration(refreshRate) * time.Second
	}
	if interval < p.Min {
		interval = p.Min
	}
	if interval > p.Max {
		interval = p.Max
	}
	return interval
}
