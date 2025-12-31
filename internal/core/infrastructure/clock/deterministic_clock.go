package clock

import (
	"time"

	infraClock "github.com/weisyn/v1/pkg/interfaces/infrastructure/clock"
)

// DeterministicClock 基于固定基准时间和递增序列，提供确定性时间源
type DeterministicClock struct {
	baseTime time.Time
	sequence int64
}

func NewDeterministicClock(base time.Time) infraClock.Clock {
	return &DeterministicClock{baseTime: base}
}

func (c *DeterministicClock) Now() time.Time {
	c.sequence++
	return c.baseTime.Add(time.Duration(c.sequence) * time.Millisecond)
}

func (c *DeterministicClock) Since(t time.Time) time.Duration { return c.Now().Sub(t) }
func (c *DeterministicClock) Unix() int64                     { return c.Now().Unix() }
func (c *DeterministicClock) UnixNano() int64                 { return c.Now().UnixNano() }
