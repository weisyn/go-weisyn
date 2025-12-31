package methods

import (
	"context"
	"encoding/json"
	"fmt"

	p2piface "github.com/weisyn/v1/pkg/interfaces/p2p"
	"go.uber.org/zap"
)

// NodeStatusMethods 节点状态相关的 JSON-RPC 方法处理器
type NodeStatusMethods struct {
	logger           *zap.Logger
	nodeRuntimeState p2piface.RuntimeState
}

// NewNodeStatusMethods 创建节点状态方法处理器
func NewNodeStatusMethods(
	logger *zap.Logger,
	nodeRuntimeState p2piface.RuntimeState,
) *NodeStatusMethods {
	return &NodeStatusMethods{
		logger:           logger,
		nodeRuntimeState: nodeRuntimeState,
	}
}

// GetNodeStatus 获取节点状态
// Method: wes_getNodeStatus
// Params: []
// Returns: 节点状态快照
func (m *NodeStatusMethods) GetNodeStatus(ctx context.Context, params json.RawMessage) (interface{}, error) {
	snapshot := m.nodeRuntimeState.GetSnapshot()

	return map[string]interface{}{
		"sync_mode":             string(snapshot.SyncMode),
		"sync_status":           string(snapshot.SyncStatus),
		"is_fully_synced":       snapshot.IsFullySynced,
		"is_online":             snapshot.IsOnline,
		"mining_enabled":        snapshot.MiningEnabled,
		"is_consensus_eligible": snapshot.IsConsensusEligible,
		"is_voter_in_round":     snapshot.IsVoterInRound,
		"is_proposer_candidate": snapshot.IsProposerCandidate,
	}, nil
}

// SetSyncMode 设置同步模式
// Method: wes_setSyncMode
// Params: [mode: string]
// mode 必须是: "full" | "light" | "archive" | "pruned"
func (m *NodeStatusMethods) SetSyncMode(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) == 0 {
		return nil, NewInvalidParamsError("missing mode parameter", nil)
	}

	modeStr, ok := args[0].(string)
	if !ok {
		return nil, NewInvalidParamsError("mode must be string", nil)
	}

	// 验证同步模式
	mode := p2piface.SyncMode(modeStr)
	switch mode {
	case p2piface.SyncModeFull, p2piface.SyncModeLight, p2piface.SyncModeArchive, p2piface.SyncModePruned:
		// 有效模式
	default:
		return nil, NewInvalidParamsError("invalid sync mode, must be one of: full, light, archive, pruned", nil)
	}

	// 更新同步模式
	if err := m.nodeRuntimeState.SetSyncMode(ctx, mode); err != nil {
		m.logger.Error("failed to set sync mode", zap.Error(err), zap.String("mode", string(mode)))
		return nil, NewInternalError(fmt.Sprintf("failed to set sync mode: %v", err), nil)
	}

	return map[string]interface{}{
		"message": "sync mode updated successfully",
		"mode":    string(mode),
	}, nil
}

// SetMiningEnabled 设置挖矿开关
// Method: wes_setMiningEnabled
// Params: [enabled: bool]
func (m *NodeStatusMethods) SetMiningEnabled(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) == 0 {
		return nil, NewInvalidParamsError("missing enabled parameter", nil)
	}

	enabled, ok := args[0].(bool)
	if !ok {
		return nil, NewInvalidParamsError("enabled must be boolean", nil)
	}

	// V2：开启挖矿必须通过 wes_startMining（需要矿工地址 + 门闸检查）。
	// 这里作为状态开关接口，仅允许关闭，避免出现“状态开了但挖矿未启动”的错误控制面。
	if enabled {
		return nil, NewInvalidParamsError("V2: enabling mining is not supported via wes_setMiningEnabled; use wes_startMining with miner address", nil)
	}

	// 更新挖矿开关
	if err := m.nodeRuntimeState.SetMiningEnabled(ctx, enabled); err != nil {
		m.logger.Error("failed to set mining enabled", zap.Error(err), zap.Bool("enabled", enabled))
		return nil, NewInternalError(fmt.Sprintf("failed to set mining enabled: %v", err), nil)
	}

	resp := map[string]interface{}{
		"message": "mining status updated successfully",
		"enabled": enabled,
	}
	return resp, nil
}
