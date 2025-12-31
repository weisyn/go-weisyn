// Package clock provides default configuration values for clock service.
package clock

import "time"

// 时钟服务配置默认值
const (
	// defaultType 默认时钟类型设为"system"
	// 原因：系统时钟是最常用的时钟类型
	defaultType = "system"

	// defaultNTPServer 默认NTP服务器设为"time.google.com"
	// 原因：Google的NTP服务器稳定可靠
	defaultNTPServer = "time.google.com"
)

var (
	// defaultSyncInterval 默认同步间隔设为5分钟
	// 原因：5分钟间隔平衡同步频率和系统开销
	defaultSyncInterval = 5 * time.Minute

	// defaultOffsetThreshold 默认偏移阈值设为500毫秒
	// 原因：500毫秒是合理的时钟偏移容忍度
	defaultOffsetThreshold = 500 * time.Millisecond

	// defaultBackoffInitial 默认初始退避时间设为5秒
	// 原因：5秒初始退避避免立即重试
	defaultBackoffInitial = 5 * time.Second

	// defaultBackoffMax 默认最大退避时间设为5分钟
	// 原因：5分钟最大退避避免过长等待
	defaultBackoffMax = 5 * time.Minute
)
