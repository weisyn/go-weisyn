package router

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEngine_SelectRoute_SelectsBestCandidate(t *testing.T) {
	e := NewEngine()

	nextHop, score, err := e.SelectRoute(nil, RouteSelectionCriteria{
		Candidates: []RouteCandidate{
			{NextHop: "A", Latency: 100 * time.Millisecond, BandwidthBps: 10_000, Reliability: 0.90},
			{NextHop: "B", Latency: 50 * time.Millisecond, BandwidthBps: 5_000, Reliability: 0.95},
			{NextHop: "C", Latency: 200 * time.Millisecond, BandwidthBps: 50_000, Reliability: 0.10},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, nextHop)
	require.GreaterOrEqual(t, score, 0.0)
	require.LessOrEqual(t, score, 1.0)
	// 默认权重偏向可靠性，其次延迟；B 应胜出
	require.Equal(t, "B", nextHop)
}

func TestEngine_SelectRoute_ThresholdsFilterCandidates(t *testing.T) {
	e := NewEngine()

	nextHop, _, err := e.SelectRoute(nil, &RouteSelectionCriteria{
		Candidates: []RouteCandidate{
			{NextHop: "A", Latency: 70 * time.Millisecond, BandwidthBps: 10_000, Reliability: 0.90},
			{NextHop: "B", Latency: 50 * time.Millisecond, BandwidthBps: 5_000, Reliability: 0.40},
		},
		MinReliability: 0.80,
		MaxLatency:     80 * time.Millisecond,
	})
	require.NoError(t, err)
	require.Equal(t, "A", nextHop)
}

func TestEngine_SelectRoute_NoCandidates(t *testing.T) {
	e := NewEngine()

	_, _, err := e.SelectRoute(nil, RouteSelectionCriteria{})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNoRouteAvailable)
}


