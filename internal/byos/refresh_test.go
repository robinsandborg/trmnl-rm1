package byos_test

import (
	"testing"
	"time"

	"github.com/robinsandborg/rm1-trmnl/internal/byos"
)

func TestRefreshPolicy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		rate   int
		policy byos.RefreshPolicy
		want   time.Duration
	}{
		{"omitted", 0, request().Refresh, 30 * time.Minute},
		{"negative", -1, request().Refresh, 30 * time.Minute},
		{"below min", 1, request().Refresh, 5 * time.Minute},
		{"at min", 300, request().Refresh, 5 * time.Minute},
		{"within bounds", 900, request().Refresh, 15 * time.Minute},
		{"at max", 3600, request().Refresh, time.Hour},
		{"above max", 7200, request().Refresh, time.Hour},
		{"fallback below min", 0, byos.RefreshPolicy{Fallback: time.Second, Min: time.Minute, Max: time.Hour}, time.Minute},
		{"fallback above max", -1, byos.RefreshPolicy{Fallback: 2 * time.Hour, Min: time.Minute, Max: time.Hour}, time.Hour},
		{"inconsistent bounds keep min then max order", 900, byos.RefreshPolicy{Fallback: 30 * time.Minute, Min: time.Hour, Max: time.Minute}, time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.policy.Interval(tc.rate); got != tc.want {
				t.Fatalf("interval=%s want %s", got, tc.want)
			}
		})
	}
}
