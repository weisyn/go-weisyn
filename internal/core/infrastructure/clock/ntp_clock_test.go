package clock

import (
	"testing"
	"time"
)

func TestNTPClock_Now(t *testing.T) {
	c, _ := NewNTPClock("time.google.com", 1*time.Minute)
	now := c.Now()
	if now.IsZero() {
		t.Fatal("Now should not be zero")
	}
}
