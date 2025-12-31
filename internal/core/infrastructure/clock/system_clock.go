package clock

import (
	"time"

	infraClock "github.com/weisyn/v1/pkg/interfaces/infrastructure/clock"
)

// SystemClock 使用系统真实时间
type SystemClock struct{}

func NewSystemClock() infraClock.Clock { return &SystemClock{} }

func (c *SystemClock) Now() time.Time                  { return time.Now() }
func (c *SystemClock) Since(t time.Time) time.Duration { return time.Since(t) }
func (c *SystemClock) Unix() int64                     { return time.Now().Unix() }
func (c *SystemClock) UnixNano() int64                 { return time.Now().UnixNano() }
