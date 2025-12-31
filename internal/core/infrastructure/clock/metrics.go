package clock

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// fetchFn 返回 (ok, offset, lastSync, lastError)
type fetchFn func() (bool, time.Duration, time.Time, error)

type clockCollector struct {
	fetch fetchFn

	offsetSeconds   *prometheus.Desc
	lastSyncSeconds *prometheus.Desc
	healthy         *prometheus.Desc
}

func (c *clockCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.offsetSeconds
	ch <- c.lastSyncSeconds
	ch <- c.healthy
}

func (c *clockCollector) Collect(ch chan<- prometheus.Metric) {
	ok, offset, lastSync, _ := c.fetch()
	ch <- prometheus.MustNewConstMetric(c.offsetSeconds, prometheus.GaugeValue, offset.Seconds())
	ch <- prometheus.MustNewConstMetric(c.lastSyncSeconds, prometheus.GaugeValue, float64(lastSync.Unix()))
	var healthy float64
	if ok {
		healthy = 1
	} else {
		healthy = 0
	}
	ch <- prometheus.MustNewConstMetric(c.healthy, prometheus.GaugeValue, healthy)
}

// RegisterClockMetrics 在默认注册表中注册时钟指标采集器
func RegisterClockMetrics(fetch fetchFn) error {
	collector := &clockCollector{
		fetch: fetch,
		offsetSeconds: prometheus.NewDesc(
			"wes_clock_offset_seconds",
			"Positive means local time is behind NTP time",
			nil, nil,
		),
		lastSyncSeconds: prometheus.NewDesc(
			"wes_clock_last_sync_unix",
			"Last successful sync Unix timestamp",
			nil, nil,
		),
		healthy: prometheus.NewDesc(
			"wes_clock_healthy",
			"1 if clock is healthy, otherwise 0",
			nil, nil,
		),
	}
	return prometheus.Register(collector)
}
