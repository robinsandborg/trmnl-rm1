package cycle

import (
	"testing"
	"time"
)

func TestRetryBackoffRemainsBounded(t *testing.T) {
	for i, want := range []time.Duration{5, 10, 20, 40, 60, 60, 60} {
		if got := retryInterval(i + 1); got != want*time.Minute {
			t.Fatalf("attempt%d: %v", i+1, got)
		}
	}
	if retryInterval(1000000) != time.Hour {
		t.Fatal("large failure count overflowed retry")
	}
}
