package clock

import (
	"time"

	infraClock "github.com/weisyn/v1/pkg/interfaces/infrastructure/clock"
)

// MockClock 测试用时钟，时间可控
type MockClock struct{ currentTime time.Time }

func NewMockClock(initial time.Time) *MockClock { return &MockClock{currentTime: initial} }

func (c *MockClock) Now() time.Time                  { return c.currentTime }
func (c *MockClock) Since(t time.Time) time.Duration { return c.currentTime.Sub(t) }
func (c *MockClock) Unix() int64                     { return c.currentTime.Unix() }
func (c *MockClock) UnixNano() int64                 { return c.currentTime.UnixNano() }

// Advance 推进时间
func (c *MockClock) Advance(d time.Duration) { c.currentTime = c.currentTime.Add(d) }

// Ensure接口实现满足 infraClock.Clock
var _ infraClock.Clock = (*MockClock)(nil)
