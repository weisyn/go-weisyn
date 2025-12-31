package clock

import (
	"time"

	infraClock "github.com/weisyn/v1/pkg/interfaces/infrastructure/clock"
)

// RoughtimeClock 默认系统时钟实现
//
// 说明：
// - 当前实现使用系统时间（time.Now()），适用于大多数场景
// - 可替换为外部可信时间源（如 Cloudflare Roughtime、Google Roughtime 等）
// - 通过实现 infraClock.Clock 接口，可以无缝替换为其他时钟实现
type RoughtimeClock struct{}

// NewRoughtimeClock 创建默认系统时钟实例
func NewRoughtimeClock() infraClock.Clock { return &RoughtimeClock{} }

func (c *RoughtimeClock) Now() time.Time                  { return time.Now() }
func (c *RoughtimeClock) Since(t time.Time) time.Duration { return time.Since(t) }
func (c *RoughtimeClock) Unix() int64                     { return time.Now().Unix() }
func (c *RoughtimeClock) UnixNano() int64                 { return time.Now().UnixNano() }
