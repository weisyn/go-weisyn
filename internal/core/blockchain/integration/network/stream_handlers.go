package network

import (
	"context"

	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

// 协议常量已迁移至 protocols.go 统一管理
// 使用 protocols.go 中定义的协议常量，与Proto定义严格对齐

// SyncProtocolRouter 同步协议路由器接口
// 由 sync/network/handler.go 提供具体实现，基于pb/network/protocol/sync.proto
type SyncProtocolRouter interface {
	// HandleKBucketSync K-bucket智能同步请求处理
	// 输入: KBucketSyncRequest (序列化后的字节数组)
	// 输出: IntelligentPaginationResponse (序列化后的字节数组)
	HandleKBucketSync(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error)

	// HandleRangePaginated 智能分页区块范围同步处理
	// 输入: KBucketSyncRequest (序列化后的字节数组)
	// 输出: IntelligentPaginationResponse (序列化后的字节数组)
	HandleRangePaginated(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error)
}

// TxProtocolRouter 交易协议路由器接口
// 由 transaction/network/handler.go 提供具体实现，基于pb/network/protocol/transaction.proto
type TxProtocolRouter interface {
	// HandleTransactionDirect 交易直连传播处理（备用传播路径）
	// 输入: TransactionPropagationRequest (序列化后的字节数组)
	// 输出: TransactionPropagationResponse (序列化后的字节数组)
	HandleTransactionDirect(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error)
}

// RegisterSyncStreamHandlers 注册区块同步流式协议处理器
// 纯粹的integration层：仅负责协议注册和路由转发，基于Proto定义
func RegisterSyncStreamHandlers(
	network netiface.Network,
	router SyncProtocolRouter,
	logger log.Logger,
) error {
	if network == nil || router == nil {
		if logger != nil {
			logger.Warn("同步协议路由器未提供，跳过注册")
		}
		return nil
	}

	// 1) K-bucket智能同步协议 - 转发给sync/network/handler.go
	if err := network.RegisterStreamHandler(protocols.ProtocolKBucketSync, func(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
		if logger != nil {
			logger.Debugf("📚 [同步集成] 接收K-bucket同步请求，来自: %s, 数据大小: %d字节", from, len(reqBytes))
		}
		return router.HandleKBucketSync(ctx, from, reqBytes)
	}); err != nil {
		if logger != nil {
			logger.Errorf("❗ K-bucket同步协议注册失败: %v", err)
		}
		return err
	}

	// 2) 智能分页范围同步协议 - 转发给sync/network/handler.go
	if err := network.RegisterStreamHandler(protocols.ProtocolRangePaginated, func(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
		if logger != nil {
			logger.Debugf("📚 [同步集成] 接收分页范围同步请求，来自: %s, 数据大小: %d字节", from, len(reqBytes))
		}
		return router.HandleRangePaginated(ctx, from, reqBytes)
	}); err != nil {
		if logger != nil {
			logger.Errorf("❗ 分页范围同步协议注册失败: %v", err)
		}
		return err
	}

	if logger != nil {
		logger.Info("✅ 区块同步流式协议处理器注册完成：K-bucket同步 + 分页范围同步")
	}
	return nil
}

// RegisterTxStreamHandlers 注册交易传播流式协议处理器
// 纯粹的integration层：仅负责协议注册和路由转发，实现双重保障传播的备份路径
func RegisterTxStreamHandlers(
	network netiface.Network,
	router TxProtocolRouter,
	logger log.Logger,
) error {
	if network == nil || router == nil {
		if logger != nil {
			logger.Warn("交易协议路由器未提供，跳过注册")
		}
		return nil
	}

	// 交易直连传播协议（备用传播路径） - 转发给transaction/network/handler.go
	if err := network.RegisterStreamHandler(protocols.ProtocolTransactionDirect, func(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
		if logger != nil {
			logger.Debugf("💰 [交易集成] 接收交易直连传播请求，来自: %s, 数据大小: %d字节", from, len(reqBytes))
		}
		return router.HandleTransactionDirect(ctx, from, reqBytes)
	}); err != nil {
		if logger != nil {
			logger.Errorf("❗ 交易直连传播协议注册失败: %v", err)
		}
		return err
	}

	if logger != nil {
		logger.Info("✅ 交易传播流式协议处理器注册完成：双重保障传播的备份路径(Stream RPC)")
	}
	return nil
}
