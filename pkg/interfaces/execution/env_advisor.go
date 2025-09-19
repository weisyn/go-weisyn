package execution

import (
	"context"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	types "github.com/weisyn/v1/pkg/types"
)

// HeaderAdvice 使用 types 层的定义，保持接口与类型分层
// 兼容别名，便于调用端少改动
type HeaderAdvice = types.HeaderAdvice

// ExecutionEnvAdvisor 为区块构建过程提供执行环境建议
//
// 实现应依据交易分析结果、当前系统/运行时状态给出头部可选字段建议
type ExecutionEnvAdvisor interface {
	AdviseHeader(ctx context.Context, transactions []*transaction.Transaction) (*HeaderAdvice, error)
}
