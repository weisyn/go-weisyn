// Package event_handler 网络质量变化事件处理器
//
// 🌐 **网络质量变化专门处理器**
//
// 本文件实现聚合器对网络质量变化事件的响应逻辑：
// - 监控网络连接质量的变化情况
// - 动态调整聚合策略以适应网络条件
// - 优化候选区块收集和分发机制
// - 确保聚合器在不同网络条件下的稳定运行
package event_handler

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// networkQualityHandler 网络质量变化事件处理器
//
// 🎯 **专门职责**：
// 处理网络质量变化事件，动态调整聚合器的网络相关策略
type networkQualityHandler struct {
	logger       log.Logger                        // 日志记录器
	stateManager interfaces.AggregatorStateManager // 状态管理器
}

// newNetworkQualityHandler 创建网络质量变化事件处理器
//
// 🏗️ **内部构造器**：
// 仅供manager.go使用的内部构造函数
func newNetworkQualityHandler(
	logger log.Logger,
	stateManager interfaces.AggregatorStateManager,
) *networkQualityHandler {
	return &networkQualityHandler{
		logger:       logger,
		stateManager: stateManager,
	}
}

// handleNetworkQualityChanged 处理网络质量变化事件的核心逻辑
//
// 🌐 **网络质量响应流程**：
//
// 1. **事件数据解析**：
//   - 解析网络质量变化事件数据
//   - 提取网络健康度、连接数、变化类型等信息
//
// 2. **质量评估**：
//   - 评估当前网络质量对聚合过程的影响程度
//   - 确定是否需要调整聚合策略
//
// 3. **策略调整**：
//   - 根据网络质量调整候选收集超时时间
//   - 优化网络评分权重配置
//   - 调整结果分发机制
//
// 4. **状态适配**：
//   - 如果网络质量严重下降，考虑延缓聚合流程
//   - 如果网络质量恢复，恢复正常聚合节奏
//
// 参数：
//   - ctx: 处理上下文
//   - event: 网络质量变化事件数据
//
// 返回：
//   - error: 处理过程中的错误
func (h *networkQualityHandler) handleNetworkQualityChanged(ctx context.Context, networkData *types.NetworkQualityChangedEventData) error {
	// ==================== 1. 事件数据验证 ====================
	if networkData == nil {
		return fmt.Errorf("网络质量变化事件数据为空")
	}

	if h.logger != nil {
		h.logger.Infof("[NetworkQualityHandler] 解析网络质量变化: change_type=%s, peer_count=%d, health=%.2f",
			networkData.ChangeType, networkData.PeerCount, networkData.NetworkHealth)
	}

	// ==================== 2. 质量评估 ====================
	qualityLevel := h.assessNetworkQuality(networkData)

	if h.logger != nil {
		h.logger.Infof("[NetworkQualityHandler] 网络质量评估结果: level=%s", qualityLevel)
	}

	// ==================== 3. 策略调整 ====================
	err := h.adjustAggregationStrategy(ctx, qualityLevel, networkData)
	if err != nil {
		return fmt.Errorf("调整聚合策略失败: %w", err)
	}

	// ==================== 4. 状态适配 ====================
	err = h.adaptAggregatorState(ctx, qualityLevel)
	if err != nil {
		return fmt.Errorf("适配聚合器状态失败: %w", err)
	}

	if h.logger != nil {
		h.logger.Info("[NetworkQualityHandler] ✅ 网络质量变化事件处理完成")
	}

	return nil
}

// NetworkQualityLevel 网络质量等级
type NetworkQualityLevel string

const (
	NetworkQualityExcellent NetworkQualityLevel = "excellent" // 优秀 (>0.8)
	NetworkQualityGood      NetworkQualityLevel = "good"      // 良好 (0.6-0.8)
	NetworkQualityFair      NetworkQualityLevel = "fair"      // 一般 (0.4-0.6)
	NetworkQualityPoor      NetworkQualityLevel = "poor"      // 较差 (0.2-0.4)
	NetworkQualityCritical  NetworkQualityLevel = "critical"  // 严重 (<0.2)
)

// assessNetworkQuality 评估当前网络质量等级
//
// 🔍 **质量评估标准**：
// - 综合考虑网络健康度、连接节点数量、变化趋势
// - 基于阈值划分不同的质量等级
func (h *networkQualityHandler) assessNetworkQuality(networkData *types.NetworkQualityChangedEventData) NetworkQualityLevel {
	health := networkData.NetworkHealth
	peerCount := networkData.PeerCount

	// 综合健康度和节点数量评估
	switch {
	case health == "excellent" && peerCount >= 5:
		return NetworkQualityExcellent
	case health == "good" && peerCount >= 3:
		return NetworkQualityGood
	case (health == "good" || health == "fair") && peerCount >= 2:
		return NetworkQualityFair
	case (health == "fair" || health == "poor") && peerCount >= 1:
		return NetworkQualityPoor
	default:
		return NetworkQualityCritical
	}
}

// adjustAggregationStrategy 根据网络质量调整聚合策略
//
// 🎯 **策略调整逻辑**：
//
// - **优秀/良好**：使用标准聚合参数，保持正常节奏
// - **一般**：适当延长候选收集时间，提高网络评分权重
// - **较差**：显著延长收集时间，降低网络要求阈值
// - **严重**：记录警告，考虑暂缓聚合直到网络恢复
func (h *networkQualityHandler) adjustAggregationStrategy(ctx context.Context, level NetworkQualityLevel, networkData *types.NetworkQualityChangedEventData) error {
	switch level {
	case NetworkQualityExcellent, NetworkQualityGood:
		// 网络质量良好，保持标准策略
		if h.logger != nil {
			h.logger.Info("[NetworkQualityHandler] 网络质量良好，维持标准聚合策略")
		}

	case NetworkQualityFair:
		// 网络质量一般，适度调整
		if h.logger != nil {
			h.logger.Info("[NetworkQualityHandler] 网络质量一般，适度调整聚合策略（延长收集时间）")
		}
		// 注意：实际的参数调整需要与具体的配置管理系统集成
		// 这里主要是记录策略变更意图

	case NetworkQualityPoor:
		// 网络质量较差，显著调整
		if h.logger != nil {
			h.logger.Warnf("[NetworkQualityHandler] 网络质量较差，显著调整聚合策略（延长收集时间，降低网络要求）")
		}

	case NetworkQualityCritical:
		// 网络质量严重，考虑暂缓
		if h.logger != nil {
			h.logger.Errorf("[NetworkQualityHandler] 网络质量严重下降，建议暂缓聚合流程: health=%.2f, peers=%d",
				networkData.NetworkHealth, networkData.PeerCount)
		}
	}

	return nil
}

// adaptAggregatorState 根据网络质量适配聚合器状态
//
// 🔄 **状态适配策略**：
//
// 在网络质量严重下降时，可能需要调整聚合器的运行状态，
// 避免在网络条件不佳的情况下强行进行聚合，导致质量问题。
func (h *networkQualityHandler) adaptAggregatorState(ctx context.Context, level NetworkQualityLevel) error {
	currentState := h.stateManager.GetCurrentState()

	switch level {
	case NetworkQualityCritical:
		// 网络质量严重时，如果正在聚合，考虑暂停或延缓
		if currentState == types.AggregationStateCollecting || currentState == types.AggregationStateEvaluating {
			if h.logger != nil {
				h.logger.Warnf("[NetworkQualityHandler] 网络质量严重，建议延缓当前聚合流程")
			}
			// 注意：这里不强制状态转换，而是记录建议
			// 实际的状态管理应该由聚合器主流程决定
		}

	case NetworkQualityPoor, NetworkQualityFair:
		// 网络质量一般或较差时，记录状态但不强制调整
		if h.logger != nil {
			h.logger.Infof("[NetworkQualityHandler] 网络质量欠佳，聚合器保持当前状态: %v", currentState)
		}

	default:
		// 网络质量良好时，无需特殊状态调整
		if h.logger != nil {
			h.logger.Debugf("[NetworkQualityHandler] 网络质量良好，聚合器正常运行: %v", currentState)
		}
	}

	return nil
}
