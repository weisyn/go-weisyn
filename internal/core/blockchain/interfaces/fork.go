package interfaces

import (
	"context"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// InternalForkService 内部分叉处理服务接口
//
// 🔄 **静默分叉处理接口**
//
// 分叉处理原则：
// - 静默后台处理，不需要复杂的状态查询
// - 处理期间通过ChainInfo.Status="fork_processing", IsReady=false标识链不可用
// - 处理完成通过integration/event通知，恢复ChainInfo.Status="normal", IsReady=true
// - 其他组件只需检查ChainInfo.IsReady了解链是否可用
type InternalForkService interface {
	// HandleFork 处理分叉区块
	//
	// 🎯 **静默异步处理分叉**
	//
	// 此方法触发后台处理，立即返回：
	// 1. 设置链状态为不可用 (ChainInfo.IsReady = false, Status = "fork_processing")
	// 2. 后台完成UTXO重构、验证、链切换等所有操作
	// 3. 处理完成后恢复链状态 (ChainInfo.IsReady = true, Status = "normal")
	// 4. 通过integration/event通知处理完成
	//
	// 参数：
	//   - ctx: 处理上下文
	//   - forkBlock: 分叉区块
	//
	// 返回：
	//   - error: 触发失败的错误（nil表示成功触发后台处理）
	HandleFork(ctx context.Context, forkBlock *core.Block) error
}
