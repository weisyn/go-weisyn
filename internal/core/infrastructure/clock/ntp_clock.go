package clock

import (
	"time"

	"github.com/beevik/ntp"
	infraClock "github.com/weisyn/v1/pkg/interfaces/infrastructure/clock"
)

// NTPClock 通过NTP周期性校正偏移的时钟实现
type NTPClock struct {
	server             string
	offset             time.Duration
	lastSync           time.Time
	syncInterval       time.Duration
	backoff            time.Duration
	backoffInitial     time.Duration
	backoffMax         time.Duration
	unhealthyThreshold time.Duration
	lastError          error
}

// NewNTPClock 创建NTP时钟
// server 例如 "time.google.com"，syncInterval 建议 5~10 分钟
func NewNTPClock(server string, syncInterval time.Duration) (infraClock.Clock, error) {
	c := &NTPClock{server: server, syncInterval: syncInterval, backoffInitial: 5 * time.Second, backoffMax: 5 * time.Minute}
	if err := c.sync(); err != nil {
		// 初始化失败不致命，置零偏移，后续重试
		c.offset = 0
		c.lastError = err
	}
	return c, nil
}

func (c *NTPClock) Now() time.Time {
	c.maybeSync()
	return time.Now().Add(c.offset)
}

func (c *NTPClock) Since(t time.Time) time.Duration { return c.Now().Sub(t) }
func (c *NTPClock) Unix() int64                     { return c.Now().Unix() }
func (c *NTPClock) UnixNano() int64                 { return c.Now().UnixNano() }

// Health 返回当前健康状态与关键指标
// healthy: 最近一次同步无严重错误，且偏移量在阈值内
func (c *NTPClock) Health() (healthy bool, offset time.Duration, lastSync time.Time, lastError error) {
	offset, lastSync, lastError = c.offset, c.lastSync, c.lastError
	// 偏移阈值未配置时不启用该检查
	if c.unhealthyThreshold > 0 && offset < 0-c.unhealthyThreshold || offset > c.unhealthyThreshold {
		return false, offset, lastSync, lastError
	}
	if lastError != nil {
		return false, offset, lastSync, lastError
	}
	return true, offset, lastSync, nil
}

func (c *NTPClock) maybeSync() {
	// 动态计算有效同步间隔（含退避）
	effective := c.syncInterval
	if c.backoff > 0 {
		if c.backoff > c.backoffMax {
			c.backoff = c.backoffMax
		}
		effective = c.backoff
	}
	if time.Since(c.lastSync) < effective {
		return
	}
	if err := c.sync(); err != nil {
		c.lastError = err
		if c.backoff == 0 {
			c.backoff = c.backoffInitial
		} else {
			c.backoff *= 2
		}
		return
	}
	// 成功，清零退避
	c.backoff = 0
}

func (c *NTPClock) sync() error {
	resp, err := ntp.Query(c.server)
	if err != nil {
		return err
	}
	c.offset = resp.ClockOffset
	c.lastSync = time.Now()
	return nil
}
