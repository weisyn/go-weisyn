package methods

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	minerquorum "github.com/weisyn/v1/internal/core/consensus/miner/quorum"
	"github.com/weisyn/v1/pkg/interfaces/consensus"
	cryptoInterface "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	p2piface "github.com/weisyn/v1/pkg/interfaces/p2p"
	"go.uber.org/zap"
)

// MiningMethods 挖矿相关的 JSON-RPC 方法处理器
type MiningMethods struct {
	logger           *zap.Logger
	minerService     consensus.MinerService
	addressManager   cryptoInterface.AddressManager // 地址管理器（可选）
	nodeRuntimeState p2piface.RuntimeState          // ✅ Phase 2.4：节点运行时状态（状态机模型，由 P2P 模块管理）
	quorumChecker    minerquorum.Checker            // V2：挖矿门闸状态查询（可选，仅查询）
}

// NewMiningMethods 创建挖矿方法处理器
func NewMiningMethods(
	logger *zap.Logger,
	minerService consensus.MinerService,
	addressManager cryptoInterface.AddressManager, // 可选参数
	nodeRuntimeState p2piface.RuntimeState, // ✅ Phase 2.4：节点运行时状态（状态机模型，由 P2P 模块管理）
	quorumChecker minerquorum.Checker, // V2：挖矿门闸状态查询（可选）
) *MiningMethods {
	return &MiningMethods{
		logger:           logger,
		minerService:     minerService,
		addressManager:   addressManager,
		nodeRuntimeState: nodeRuntimeState,
		quorumChecker:    quorumChecker,
	}
}

// GetMiningQuorumStatus 获取挖矿门闸/网络法定人数状态（V2）。
// Method: wes_getMiningQuorumStatus
// Params: []
func (m *MiningMethods) GetMiningQuorumStatus(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if m.quorumChecker == nil {
		return nil, NewInternalError("mining quorum checker not available", nil)
	}
	res, err := m.quorumChecker.Check(ctx)
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("check mining quorum status failed: %v", err), nil)
	}
	if res == nil {
		return nil, NewInternalError("mining quorum status is nil", nil)
	}

	peerHeights := map[string]uint64{}
	for pid, h := range res.Metrics.PeerHeights {
		peerHeights[pid.String()] = h
	}

	return map[string]interface{}{
		"allow_mining":     res.AllowMining,
		"state":           string(res.State),
		"reason":          res.Reason,
		"suggested_action": res.SuggestedAction,
		"metrics": map[string]interface{}{
			"discovered_peers":       res.Metrics.DiscoveredPeers,
			"connected_peers":        res.Metrics.ConnectedPeers,
			"qualified_peers":        res.Metrics.QualifiedPeers,
			"required_quorum_total":  res.Metrics.RequiredQuorumTotal,
			"current_quorum_total":   res.Metrics.CurrentQuorumTotal,
			"quorum_reached":         res.Metrics.QuorumReached,
			"local_height":           res.Metrics.LocalHeight,
			"median_peer_height":     res.Metrics.MedianPeerHeight,
			"height_skew":            res.Metrics.HeightSkew,
			"peer_heights":           peerHeights,
			"discovery_started_at":   res.Metrics.DiscoveryStartedAt.Unix(),
			"quorum_reached_at":      res.Metrics.QuorumReachedAt.Unix(),
		},
		"chain_tip": map[string]interface{}{
			"tip_readable":              res.ChainTip.TipReadable,
			"tip_timestamp":             res.ChainTip.TipTimestamp,
			"tip_age_seconds":           int64(res.ChainTip.TipAge / time.Second),
			"tip_fresh":                 res.ChainTip.TipFresh,
			"tip_healthy_for_handshake": res.ChainTip.TipHealthyForHandshake,
		},
	}, nil
}

// StartMining 启动挖矿
// Method: wes_startMining
// Params: [minerAddress: string]
// minerAddress 格式: Base58格式的WES地址（如CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR）
func (m *MiningMethods) StartMining(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// ✅ V2：挖矿门闸由共识层统一执行（网络法定人数 + 高度一致性 + 链尖前置）。
	// 这里仅保留“轻节点不能挖矿”的硬检查，避免无意义地进入 miner 启动流程。
	if m.nodeRuntimeState != nil {
		// 检查不变式 I4：挖矿前置条件
		// 只有 full/archive/pruned 模式的节点可以开启挖矿
		snapshot := m.nodeRuntimeState.GetSnapshot()
		if snapshot.SyncMode != p2piface.SyncModeFull && snapshot.SyncMode != p2piface.SyncModeArchive && snapshot.SyncMode != p2piface.SyncModePruned {
			return nil, NewInternalError(fmt.Sprintf("轻节点不能开启挖矿 (当前同步模式: %s)", snapshot.SyncMode), nil)
		}
	}

	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) == 0 {
		return nil, NewInvalidParamsError("missing miner address", nil)
	}

	// 解析矿工地址参数（WES使用Base58格式，不兼容ETH的0x前缀格式）
	minerAddressStr, ok := args[0].(string)
	if !ok {
		return nil, NewInvalidParamsError("miner address must be string", nil)
	}

	// 验证并转换Base58格式地址
	if m.addressManager == nil {
		return nil, NewInternalError("address manager not available", nil)
	}

	// 拒绝0x前缀的ETH地址格式
	if len(minerAddressStr) > 2 && (minerAddressStr[:2] == "0x" || minerAddressStr[:2] == "0X") {
		return nil, NewInvalidParamsError("WES地址必须使用Base58格式，不支持0x前缀的ETH地址格式", nil)
	}

	// 验证Base58格式地址
	validAddress, err := m.addressManager.StringToAddress(minerAddressStr)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid miner address format: %v", err), nil)
	}

	// 转换为字节数组
	minerAddress, err := m.addressManager.AddressToBytes(validAddress)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("failed to convert address: %v", err), nil)
	}

	// 验证地址长度（必须是20字节）
	if len(minerAddress) != 20 {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid address length: expected 20 bytes, got %d", len(minerAddress)), nil)
	}

	// 调用MinerService启动挖矿
	if m.minerService == nil {
		m.logger.Warn("MinerService not available - mining module may be disabled in config")
		return nil, NewInternalError("mining功能未启用：请检查节点配置中的consensus和mining设置", nil)
	}

	// ✅ 修复：使用 context.Background() 而不是请求上下文
	// 原因：HTTP请求结束后会取消ctx，导致挖矿服务立即停止
	// 挖矿是长期运行的后台服务，需要独立的上下文
	if err := m.minerService.StartMining(context.Background(), minerAddress); err != nil {
		m.logger.Error("Failed to start mining",
			zap.String("miner_address", hex.EncodeToString(minerAddress)),
			zap.Error(err))
		return nil, NewInternalError(fmt.Sprintf("启动挖矿失败: %v", err), nil)
	}

	// ✅ 启动成功后再更新状态机的挖矿开关（避免“状态开了但挖矿未启动”的假阳性）
	if m.nodeRuntimeState != nil {
		if err := m.nodeRuntimeState.SetMiningEnabled(context.Background(), true); err != nil {
			m.logger.Error("Failed to enable mining in state machine", zap.Error(err))
			// 不回滚 minerService：挖矿已启动，状态机更新失败只影响展示/诊断
		}
	}

	// 🎯 返回Base58格式地址
	var base58Address string
	if m.addressManager != nil {
		addressBytes := minerAddress
		if len(addressBytes) == 20 {
			base58Addr, err := m.addressManager.BytesToAddress(addressBytes)
			if err == nil {
				base58Address = base58Addr
			}
		}
	}
	m.logger.Info("Mining started",
		zap.String("miner_address_base58", base58Address))

	// 返回成功响应（返回Base58格式地址）
	resp := map[string]interface{}{
		"status":        "success",
		"miner_address": base58Address,
		"message":       "mining started",
	}
	return resp, nil
}

// StopMining 停止挖矿
// Method: wes_stopMining
// Params: []
func (m *MiningMethods) StopMining(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 调用MinerService停止挖矿
	if m.minerService == nil {
		m.logger.Warn("MinerService not available")
		return nil, NewInternalError("mining功能未启用：请检查节点配置", nil)
	}

	// ✅ 停止操作使用请求上下文是合理的，因为只是发送停止信号
	if err := m.minerService.StopMining(ctx); err != nil {
		m.logger.Error("Failed to stop mining", zap.Error(err))
		return nil, NewInternalError(fmt.Sprintf("stop mining failed: %v", err), nil)
	}

	// ✅ Phase 2.4：更新状态机的挖矿开关
	if m.nodeRuntimeState != nil {
		if err := m.nodeRuntimeState.SetMiningEnabled(ctx, false); err != nil {
			m.logger.Error("Failed to disable mining in state machine", zap.Error(err))
			// 不返回错误，因为挖矿服务已经停止
		}
	}

	m.logger.Info("Mining stopped")

	// 返回成功响应
	return map[string]interface{}{
		"status":  "success",
		"message": "mining stopped",
	}, nil
}

// GetMiningStatus 获取挖矿状态
// Method: wes_getMiningStatus
// Params: []
func (m *MiningMethods) GetMiningStatus(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 调用MinerService获取状态
	if m.minerService == nil {
		// 如果服务不可用，返回停止状态（而不是错误）
		return map[string]interface{}{
			"is_running":    false,
			"miner_address": "",
			"status":        "mining module disabled",
		}, nil
	}

	isRunning, minerAddress, err := m.minerService.GetMiningStatus(ctx)
	if err != nil {
		m.logger.Error("Failed to get mining status", zap.Error(err))
		return nil, NewInternalError(fmt.Sprintf("get mining status failed: %v", err), nil)
	}

	// 构造响应（返回Base58格式地址）
	response := map[string]interface{}{
		"is_running": isRunning,
	}

	if len(minerAddress) > 0 && m.addressManager != nil {
		base58Addr, err := m.addressManager.BytesToAddress(minerAddress)
		if err == nil {
			response["miner_address"] = base58Addr
		} else {
			response["miner_address"] = ""
		}
	} else {
		response["miner_address"] = ""
	}

	return response, nil
}

// ✅ Phase 2.4：已删除 checkMiningCapability 方法
// 现在使用状态机模型（NodeRuntimeState）来检查挖矿资格
// 检查逻辑已集成到 StartMining 方法中
