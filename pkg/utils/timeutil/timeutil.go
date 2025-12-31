// Package timeutil provides time utility functions.
package timeutil

import (
	"time"

	infraClock "github.com/weisyn/v1/pkg/interfaces/infrastructure/clock"
)

var nowProvider func() time.Time = time.Now

// SetClock 设置时间提供者（由基础设施注入），未注入时回退系统时钟
func SetClock(c infraClock.Clock) {
	if c != nil {
		nowProvider = c.Now
	}
}

// Now 返回当前时间（来自注入的时钟）
func Now() time.Time { return nowProvider() }

// NowUnix 返回当前Unix秒时间戳（uint64）
func NowUnix() uint64 { return uint64(nowProvider().Unix()) }
