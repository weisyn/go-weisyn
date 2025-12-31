package adapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/weisyn/v1/pkg/types"
)

// UnifiedTransactionFacade Facade接口（M2重构：SDKAdapter只依赖Compose阶段）
type UnifiedTransactionFacade interface {
	// Compose 阶段1：将意图转换为交易草稿
	Compose(ctx context.Context, intents interface{}) (*types.DraftTx, error)
}

// SDKAdapter SDK适配器
//
// 🎯 用途：连接合约SDK到TX Facade（M2重构后）
//
// 功能：
// - 解析SDK draft JSON
// - 转换为TX Draft类型
// - 调用Facade.Compose创建草稿
// - 错误处理和转换
//
// 🔧 架构定位（M2重构）：
// - 归属：ISPC域（internal/core/ispc/hostabi/adapter）
// - 依赖：仅依赖TX L3 Facade.Compose阶段
// - 流程：合约调用 → SDKAdapter.Compose → Facade.Compose → Draft返回
type SDKAdapter struct {
	facade UnifiedTransactionFacade
}

// NewSDKAdapter 创建SDK适配器（M2重构：简化依赖）
func NewSDKAdapter(
	facade UnifiedTransactionFacade,
) *SDKAdapter {
	return &SDKAdapter{
		facade: facade,
	}
}

// SDKDraft SDK侧交易草稿（JSON格式）
type SDKDraft struct {
	Outputs []SDKOutput `json:"outputs"`
	Intents []SDKIntent `json:"intents"`
}

// SDKOutput SDK输出描述符
type SDKOutput struct {
	Type     string `json:"type"`
	To       string `json:"to,omitempty"`       // base64编码的地址
	TokenID  string `json:"token_id,omitempty"` // base64编码的代币ID
	Amount   uint64 `json:"amount,omitempty"`
	StateID  string `json:"state_id,omitempty"` // base64编码的状态ID
	Version  uint64 `json:"version,omitempty"`
	ExecHash string `json:"exec_hash,omitempty"` // base64编码的执行哈希
	Resource string `json:"resource,omitempty"`  // base64编码的资源数据
}

// SDKIntent SDK意图描述符
type SDKIntent struct {
	Type      string `json:"type"`
	From      string `json:"from,omitempty"`     // base64编码的地址
	To        string `json:"to,omitempty"`       // base64编码的地址
	TokenID   string `json:"token_id,omitempty"` // base64编码的代币ID
	Amount    uint64 `json:"amount,omitempty"`
	Staker    string `json:"staker,omitempty"`    // base64编码的地址
	Validator string `json:"validator,omitempty"` // base64编码的地址
}

// BuildTransaction 构建交易（SDK入口，M2重构后）
//
// 参数：
//   - ctx: 上下文
//   - draftJSON: SDK draft JSON数据
//
// 返回：
//   - draft: 交易草稿（Draft）
//   - error: 错误信息
//
// 🎯 M2重构设计：
// - Host模式只负责创建Draft（Compose阶段）
// - 后续六阶段流水线由ISPC Coordinator或外部环境完成
// - 符合"执行即构建"的架构原则
func (a *SDKAdapter) BuildTransaction(
	ctx context.Context,
	draftJSON []byte,
) (*types.DraftTx, error) {
	// 🔧 **修复**：添加 nil facade 检查，避免运行时 panic
	if a.facade == nil {
		return nil, fmt.Errorf("facade未设置")
	}

	// 1. 解析SDK draft
	sdkDraft, err := a.parseSDKDraft(draftJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SDK draft: %w", err)
	}

	// 2. 转换为TX intents
	intents, err := a.convertToTxIntents(ctx, sdkDraft)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to tx intents: %w", err)
	}

	// 3. 调用Facade.Compose创建草稿（M2重构：只调用Compose阶段）
	draft, err := a.facade.Compose(ctx, intents)
	if err != nil {
		return nil, a.convertError(err)
	}

	return draft, nil
}

// parseSDKDraft 解析SDK draft JSON
func (a *SDKAdapter) parseSDKDraft(draftJSON []byte) (*SDKDraft, error) {
	var draft SDKDraft
	if err := json.Unmarshal(draftJSON, &draft); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return &draft, nil
}

// convertToTxIntents 转换SDK draft为TX intents（M2重构）
//
// 🎯 **转换逻辑**：
// - 将SDKDraft转换为适合Facade.Compose的intents结构
// - 由于Facade.Compose接受interface{}类型，直接返回SDKDraft即可
// - SDKDraft包含Intents和Outputs，Facade.Compose会根据这些信息创建DraftTx
//
// 📋 **参数**：
//   - ctx: 上下文
//   - sdkDraft: SDK侧交易草稿
//
// 📋 **返回值**：
//   - interface{}: TX intents（实际类型为*SDKDraft）
//   - error: 转换错误
func (a *SDKAdapter) convertToTxIntents(
	ctx context.Context,
	sdkDraft *SDKDraft,
) (interface{}, error) {
	if sdkDraft == nil {
		return nil, fmt.Errorf("SDK draft不能为空")
	}

	// 验证SDKDraft的基本结构
	if len(sdkDraft.Outputs) == 0 && len(sdkDraft.Intents) == 0 {
		return nil, fmt.Errorf("SDK draft必须包含至少一个输出或意图")
	}

	// 直接返回SDKDraft作为intents
	// Facade.Compose会根据SDKDraft中的Intents和Outputs创建DraftTx
	// 注意：Facade.Compose接受interface{}类型，可以处理SDKDraft结构
	return sdkDraft, nil
}

// 注意：M2重构后移除了convertOutput和convertIntent方法
// 这些方法依赖于旧的HostTransactionBuilder，已被convertToTxIntents替代
// 具体的输出和意图转换逻辑将在M4阶段实现Facade.Compose时补充

// convertError 转换错误为SDK友好的错误消息
func (a *SDKAdapter) convertError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// 错误消息转换
	switch {
	case contains(errMsg, "insufficient balance"):
		return fmt.Errorf("余额不足")
	case contains(errMsg, "invalid parameter"):
		return fmt.Errorf("参数无效")
	case contains(errMsg, "invalid state"):
		return fmt.Errorf("状态无效")
	case contains(errMsg, "not found"):
		return fmt.Errorf("未找到")
	case contains(errMsg, "permission denied"):
		return fmt.Errorf("权限不足")
	default:
		return err
	}
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

// findSubstring 查找子串位置
func findSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(s) < len(substr) {
		return -1
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}

	return -1
}

// decodeBase64 解码base64字符串
//
// ⚠️ **重构说明**：
// 使用标准库 encoding/base64 替换自定义实现，提高可靠性和性能。
//
// 📋 **参数**：
//   - s: base64编码的字符串
//
// 🔧 **返回值**：
//   - []byte: 解码后的字节数组
//   - error: 解码错误
func decodeBase64(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}

	// 使用标准库进行base64解码
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("base64解码失败: %w", err)
	}

	return decoded, nil
}
