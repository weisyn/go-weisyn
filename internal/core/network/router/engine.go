// Package router provides routing engine functionality for network communication.
package router

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// engine.go
// 路由选择引擎（方法框架）：
// - 负责综合延迟/带宽/可靠性等指标进行路径选择
// - 提供评分与选择的入口方法签名

// Engine 路由选择引擎（方法框架）
type Engine struct{}

// NewEngine 创建路由引擎
func NewEngine() *Engine { return &Engine{} }

// RouteCandidate 候选路由（下一跳）样本
//
// 说明：
// - NextHop 的具体类型由上层网络栈决定（peer.ID / multiaddr / 自定义节点ID 等）
// - 指标用于评分选择；缺失指标时会被当作较差候选
type RouteCandidate struct {
	NextHop      interface{}
	Latency      time.Duration // 越小越好
	BandwidthBps uint64        // 越大越好
	Reliability  float64       // 0..1 越大越好
}

// RouteSelectionCriteria 路由选择输入（可选）
// - Candidates：候选列表
// - Weights：各指标权重（若全为0则使用默认权重）
// - MinReliability：最低可靠性阈值（<=0 表示不限制）
// - MaxLatency：最大可接受延迟（<=0 表示不限制）
type RouteSelectionCriteria struct {
	Candidates     []RouteCandidate
	WeightLatency  float64
	WeightBandwidth float64
	WeightReliability float64
	MinReliability float64
	MaxLatency     time.Duration
}

// SelectRoute 依据路由表与质量评估选择最优路径
// 返回：
//   - nextHop: 下一跳节点
//   - score: 评分（0..1，越大越好）
//   - error: 选择失败的错误
func (e *Engine) SelectRoute(_target interface{}, criteria interface{}) (interface{}, float64, error) {
	_ = _target

	// 兼容多种入参形态
	var c RouteSelectionCriteria
	switch v := criteria.(type) {
	case nil:
		return nil, 0, ErrNoRouteAvailable
	case RouteSelectionCriteria:
		c = v
	case *RouteSelectionCriteria:
		if v != nil {
			c = *v
		}
	case []RouteCandidate:
		c.Candidates = v
	case []*RouteCandidate:
		c.Candidates = make([]RouteCandidate, 0, len(v))
		for _, it := range v {
			if it != nil {
				c.Candidates = append(c.Candidates, *it)
			}
		}
	default:
		return nil, 0, fmt.Errorf("unsupported route criteria type: %T", criteria)
	}

	if len(c.Candidates) == 0 {
		return nil, 0, ErrNoRouteAvailable
	}

	// 默认权重（偏向可靠性，其次延迟）
	wL, wB, wR := c.WeightLatency, c.WeightBandwidth, c.WeightReliability
	if wL == 0 && wB == 0 && wR == 0 {
		wL, wB, wR = 0.35, 0.15, 0.50
	}
	// 归一化权重
	sumW := math.Abs(wL) + math.Abs(wB) + math.Abs(wR)
	if sumW == 0 {
		wL, wB, wR, sumW = 0.35, 0.15, 0.50, 1.0
	}
	wL, wB, wR = wL/sumW, wB/sumW, wR/sumW

	// 过滤候选（阈值）
	filtered := make([]RouteCandidate, 0, len(c.Candidates))
	for _, cand := range c.Candidates {
		if cand.NextHop == nil {
			continue
		}
		if c.MinReliability > 0 && cand.Reliability < c.MinReliability {
			continue
		}
		if c.MaxLatency > 0 && cand.Latency > c.MaxLatency {
			continue
		}
		filtered = append(filtered, cand)
	}
	if len(filtered) == 0 {
		return nil, 0, ErrNoRouteAvailable
	}

	// 统计范围（用于归一化）
	minLat, maxLat := time.Duration(math.MaxInt64), time.Duration(0)
	var minBw uint64 = math.MaxUint64
	var maxBw uint64 = 0
	minRel, maxRel := 1.0, 0.0
	for _, cand := range filtered {
		if cand.Latency > 0 && cand.Latency < minLat {
			minLat = cand.Latency
		}
		if cand.Latency > maxLat {
			maxLat = cand.Latency
		}
		if cand.BandwidthBps < minBw {
			minBw = cand.BandwidthBps
		}
		if cand.BandwidthBps > maxBw {
			maxBw = cand.BandwidthBps
		}
		rel := clamp01(cand.Reliability)
		if rel < minRel {
			minRel = rel
		}
		if rel > maxRel {
			maxRel = rel
		}
	}
	if minLat == time.Duration(math.MaxInt64) {
		minLat = 0
	}
	if minBw == math.MaxUint64 {
		minBw = 0
	}

	// 评分并选最优（确定性：分数降序，若相同按 nextHop 的字符串化排序）
	type scored struct {
		cand  RouteCandidate
		score float64
		key   string
	}
	scoredList := make([]scored, 0, len(filtered))
	for _, cand := range filtered {
		latScore := normalizeLowerBetter(cand.Latency, minLat, maxLat)
		bwScore := normalizeHigherBetterUint(cand.BandwidthBps, minBw, maxBw)
		relScore := normalizeHigherBetterFloat(clamp01(cand.Reliability), minRel, maxRel)
		score := wL*latScore + wB*bwScore + wR*relScore
		scoredList = append(scoredList, scored{
			cand:  cand,
			score: score,
			key:   fmt.Sprintf("%T:%v", cand.NextHop, cand.NextHop),
		})
	}

	sort.Slice(scoredList, func(i, j int) bool {
		if scoredList[i].score != scoredList[j].score {
			return scoredList[i].score > scoredList[j].score
		}
		return scoredList[i].key < scoredList[j].key
	})

	return scoredList[0].cand.NextHop, scoredList[0].score, nil
}

// ErrRouteEngineNotImplemented 路由引擎未实现（显式失败，避免静默降级）。
var ErrRouteEngineNotImplemented = fmt.Errorf("router engine not implemented")

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func normalizeLowerBetter(v, minV, maxV time.Duration) float64 {
	if maxV <= minV {
		return 1.0
	}
	if v <= 0 {
		// 无数据视为最差
		return 0.0
	}
	x := float64(maxV-v) / float64(maxV-minV)
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func normalizeHigherBetterUint(v, minV, maxV uint64) float64 {
	if maxV <= minV {
		return 1.0
	}
	x := float64(v-minV) / float64(maxV-minV)
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func normalizeHigherBetterFloat(v, minV, maxV float64) float64 {
	if maxV <= minV {
		return 1.0
	}
	x := (v - minV) / (maxV - minV)
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
