// sync_methods.go - 同步诊断JSON-RPC方法
package methods

import (
	"context"
	"encoding/json"

	"github.com/weisyn/v1/internal/core/chain/sync"
	"go.uber.org/zap"
)

// SyncMethods 同步诊断方法
type SyncMethods struct {
	logger *zap.Logger
}

// NewSyncMethods 创建同步诊断方法实例
func NewSyncMethods(logger *zap.Logger) *SyncMethods {
	return &SyncMethods{
		logger: logger,
	}
}

// GetSyncDiagnostics 获取同步诊断信息
// RPC: wes_getSyncDiagnostics
//
// 返回值：
//   - SyncDiagnostics: 包含当前同步状态、进度、网络高度、失败历史等完整诊断信息
func (m *SyncMethods) GetSyncDiagnostics(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if m.logger != nil {
		m.logger.Debug("获取同步诊断信息")
	}
	
	diag := sync.GetSyncDiagnostics()
	return diag, nil
}

// GetSyncFailureHistory 获取同步失败历史
// RPC: wes_getSyncFailureHistory
//
// 参数：
//   - limit: 可选，返回最近N条记录
//
// 返回值：
//   - []SyncFailureReason: 失败历史列表（按时间顺序）
func (m *SyncMethods) GetSyncFailureHistory(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if m.logger != nil {
		m.logger.Debug("获取同步失败历史")
	}
	
	// 解析参数
	var reqParams struct {
		Limit *int `json:"limit,omitempty"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &reqParams); err != nil {
			return nil, NewInvalidParamsError("invalid params", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
	
	history := sync.GetSyncFailureHistory()
	
	// 应用limit参数
	if reqParams.Limit != nil && *reqParams.Limit > 0 && *reqParams.Limit < len(history) {
		history = history[len(history)-*reqParams.Limit:]
	}
	
	return history, nil
}

// GetNetworkHeightHistory 获取网络高度历史
// RPC: wes_getNetworkHeightHistory
//
// 参数：
//   - limit: 可选，返回最近N条记录
//
// 返回值：
//   - []NetworkHeightRecord: 高度历史列表（按时间顺序）
func (m *SyncMethods) GetNetworkHeightHistory(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if m.logger != nil {
		m.logger.Debug("获取网络高度历史")
	}
	
	// 解析参数
	var reqParams struct {
		Limit *int `json:"limit,omitempty"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &reqParams); err != nil {
			return nil, NewInvalidParamsError("invalid params", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
	
	history := sync.GetNetworkHeightHistory()
	
	// 应用limit参数
	if reqParams.Limit != nil && *reqParams.Limit > 0 && *reqParams.Limit < len(history) {
		history = history[len(history)-*reqParams.Limit:]
	}
	
	return history, nil
}

