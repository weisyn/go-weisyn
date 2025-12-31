package clock

import (
	"os"
	"strconv"
	"time"
)

// ClockOptions 时钟配置
type ClockOptions struct {
	Type            string        `json:"type"` // system | ntp | deterministic | mock | roughtime
	NTPServer       string        `json:"ntp_server"`
	SyncInterval    time.Duration `json:"sync_interval"`
	OffsetThreshold time.Duration `json:"offset_threshold"` // 判定不健康的偏移阈值

	// 回退与重试
	BackoffInitial time.Duration `json:"backoff_initial"`
	BackoffMax     time.Duration `json:"backoff_max"`

	// Deterministic 配置
	DeterministicBaseUnix int64 `json:"deterministic_base_unix"`
}

// Config 提供访问选项
type Config struct {
	options *ClockOptions
}

// New 创建配置，支持环境变量覆盖（简化实现）
// 环境变量：
//
//	CLOCK_TYPE (system|ntp|deterministic)
//	CLOCK_NTP_SERVER (如 time.google.com)
//	CLOCK_SYNC_INTERVAL_MS
//	CLOCK_OFFSET_THRESHOLD_MS
//	CLOCK_BACKOFF_INITIAL_MS
//	CLOCK_BACKOFF_MAX_MS
//	CLOCK_DETERMINISTIC_BASE_UNIX
func New() *Config {
	opts := &ClockOptions{
		Type:                  defaultType,
		NTPServer:             defaultNTPServer,
		SyncInterval:          defaultSyncInterval,
		OffsetThreshold:       defaultOffsetThreshold,
		BackoffInitial:        defaultBackoffInitial,
		BackoffMax:            defaultBackoffMax,
		DeterministicBaseUnix: 0,
	}

	if v := os.Getenv("CLOCK_TYPE"); v != "" {
		opts.Type = v
	}
	if v := os.Getenv("CLOCK_NTP_SERVER"); v != "" {
		opts.NTPServer = v
	}
	if v := os.Getenv("CLOCK_SYNC_INTERVAL_MS"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			opts.SyncInterval = time.Duration(n) * time.Millisecond
		}
	}
	if v := os.Getenv("CLOCK_OFFSET_THRESHOLD_MS"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			opts.OffsetThreshold = time.Duration(n) * time.Millisecond
		}
	}
	if v := os.Getenv("CLOCK_BACKOFF_INITIAL_MS"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			opts.BackoffInitial = time.Duration(n) * time.Millisecond
		}
	}
	if v := os.Getenv("CLOCK_BACKOFF_MAX_MS"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			opts.BackoffMax = time.Duration(n) * time.Millisecond
		}
	}
	if v := os.Getenv("CLOCK_DETERMINISTIC_BASE_UNIX"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			opts.DeterministicBaseUnix = n
		}
	}

	return &Config{options: opts}
}

func (c *Config) GetOptions() *ClockOptions { return c.options }
