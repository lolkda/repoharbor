package scheduler

import (
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	cases := map[int]time.Duration{1: time.Minute, 2: 5 * time.Minute, 3: 15 * time.Minute}
	for attempt, want := range cases {
		if got := retryDelay(attempt); got != want {
			t.Fatalf("attempt %d: got %v want %v", attempt, got, want)
		}
	}
}
