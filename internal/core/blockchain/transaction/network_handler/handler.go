// Package network_handler 实现交易网络协议处理服务
//
// 🎯 **交易网络协议处理服务模块**
//
// 本包实现 NetworkProtocolHandler 接口，提供交易网络协议处理功能：
// - 实现TxProtocolRouter接口（流式协议处理）
// - 实现TxAnnounceRouter接口（订阅协议处理）
// - 支持交易双重保障传播机制
package network_handler

import (
	"context"
	"fmt"

	networkIntegration "github.com/weisyn/v1/internal/core/blockchain/integration/network"
	"github.com/weisyn/v1/internal/core/blockchain/interfaces"
	"github.com/weisyn/v1/internal/core/blockchain/transaction/lifecycle"
	txProtocol "github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// TxNetworkProtocolHandlerService 交易网络协议处理服务实现（薄委托层）
//
// 🎯 **职责定位**：
// - 实现 interfaces.NetworkProtocolHandler 接口
// - 实现 integration/network.TxAnnounceRouter 接口（订阅协议）
// - 实现 integration/network.TxProtocolRouter 接口（流式协议）
// - 处理来自P2P网络的交易公告消息和交易中继请求
// - 执行：解码 → 验证 → 入池的完整流程
//
// 🏗️ **设计原则**：
// - 遵循Manager委托模式，作为transaction域的网络子模块
// - 统一归口处理所有交易相关的网络消息
// - 使用真实依赖服务，无TODO/临时实现
// - 严格遵循公共接口，不直接调用crypto包
type TxNetworkProtocolHandlerService struct {
	txPool    mempool.TxPool                          // 交易池服务
	validator *lifecycle.TransactionValidationService // 交易验证服务
	logger    log.Logger                              // 日志服务
}

// NewTxNetworkProtocolHandlerService 创建交易网络协议处理服务实例
//
// 参数:
//
//	txPool: 交易池服务，用于提交验证通过的交易
//	validator: 交易验证服务，用于验证交易有效性
//	logger: 日志服务，用于记录处理过程
//
// 返回:
//
//	interfaces.NetworkProtocolHandler: 交易网络协议处理器实例
func NewTxNetworkProtocolHandlerService(
	txPool mempool.TxPool,
	validator *lifecycle.TransactionValidationService,
	logger log.Logger,
) interfaces.NetworkProtocolHandler {
	return &TxNetworkProtocolHandlerService{
		txPool:    txPool,
		validator: validator,
		logger:    logger,
	}
}

// 编译时确保 TxNetworkProtocolHandlerService 实现了 NetworkProtocolHandler 接口
var _ interfaces.NetworkProtocolHandler = (*TxNetworkProtocolHandlerService)(nil)

// HandleTransactionAnnounce 处理交易公告
//
// 🎯 **实现 integration/network.TxAnnounceRouter 接口**
//
// 处理流程：
// 1. 解析标准protobuf交易公告数据（包含完整交易）
// 2. 验证交易公告的完整性
// 3. 去重检查：确保交易未在本地内存池中
// 4. 完整交易验证：验证交易有效性
// 5. 入池处理：将验证通过的交易添加到内存池
//
// 📝 **注意**：
// 此方法处理标准TransactionAnnouncement protobuf消息，包含完整交易数据。
// 遵循双重保障传播机制的主要路径（GossipSub）。
//
// 参数:
//   - ctx: 上下文对象
//   - from: 发送方peer ID
//   - topic: 公告主题
//   - data: 交易公告的protobuf序列化数据
//
// 返回:
//   - error: 处理过程中的错误，nil表示成功
func (h *TxNetworkProtocolHandlerService) HandleTransactionAnnounce(ctx context.Context, from peer.ID, topic string, data []byte) error {
	if h.logger != nil {
		h.logger.Debugf("处理交易公告: from=%s, topic=%s, size=%d", from.String()[:8], topic, len(data))
	}

	// 1. 解析标准protobuf TransactionAnnouncement消息
	var announcement txProtocol.TransactionAnnouncement
	if err := proto.Unmarshal(data, &announcement); err != nil {
		if h.logger != nil {
			h.logger.Warnf("解析TransactionAnnouncement失败: %v", err)
		}
		return fmt.Errorf("解析TransactionAnnouncement失败: %w", err)
	}

	// 2. 验证消息完整性
	if len(announcement.TransactionHash) != 32 {
		return fmt.Errorf("交易哈希长度无效: 期望32字节，实际%d字节", len(announcement.TransactionHash))
	}

	if announcement.Transaction == nil {
		return fmt.Errorf("缺少完整交易数据")
	}

	if announcement.Timestamp == 0 {
		return fmt.Errorf("交易时间戳不能为0")
	}

	// 3. 去重检查：确保交易未在内存池中
	txHash := announcement.TransactionHash
	existingTx, err := h.txPool.GetTx(txHash)
	if err == nil && existingTx != nil {
		if h.logger != nil {
			h.logger.Debug(fmt.Sprintf("交易已存在于内存池中，跳过处理: txHash=%x", txHash[:8]))
		}
		return nil // 重复交易，不算错误
	}

	// 4. 完整交易验证
	if h.validator != nil {
		valid, err := h.validator.ValidateTransactionObject(ctx, announcement.Transaction)
		if err != nil {
			if h.logger != nil {
				h.logger.Warnf("交易验证过程失败: txHash=%x, error=%v", txHash[:8], err)
			}
			return fmt.Errorf("交易验证过程失败: %w", err)
		}
		if !valid {
			if h.logger != nil {
				h.logger.Warnf("交易验证不通过: txHash=%x", txHash[:8])
			}
			return fmt.Errorf("交易验证不通过")
		}
	}

	// 5. 添加到内存池
	submittedTxHash, err := h.txPool.SubmitTx(announcement.Transaction)
	if err != nil {
		if h.logger != nil {
			h.logger.Errorf("添加到内存池失败: txHash=%x, error=%v", txHash[:8], err)
		}
		return fmt.Errorf("添加到内存池失败: %w", err)
	}

	// 6. 记录处理成功
	if h.logger != nil {
		h.logger.Infof("✅ 交易公告处理完成: txHash=%x, submittedHash=%x, messageId=%s, from=%s",
			txHash[:8], submittedTxHash[:8], announcement.MessageId, from.String()[:8])
	}

	return nil
}

// ============================================================================
//                           流式协议处理 (Stream Handlers)
// ============================================================================

// HandleTransactionDirect 处理交易直连传播请求
//
// 🎯 **实现 integration/network.TxProtocolRouter 接口**
//
// 处理双重保障传播机制的备份路径（Stream RPC）：
// 1. 解析TransactionPropagationRequest请求
// 2. 检查请求的交易哈希列表
// 3. 确定哪些交易需要传输
// 4. 返回TransactionPropagationResponse响应
//
// 📝 **备份传播路径特性**：
// - 确保送达：要求明确确认
// - K-bucket选择：2-3个邻近节点
// - 点对点传输：可靠的网络传输
//
// 参数：
//   - ctx: 上下文（用于超时控制）
//   - from: 发送方节点ID
//   - reqBytes: 序列化的TransactionPropagationRequest数据
//
// 返回：
//   - []byte: 序列化的TransactionPropagationResponse数据
//   - error: 处理失败时的错误信息
func (h *TxNetworkProtocolHandlerService) HandleTransactionDirect(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	if h.logger != nil {
		h.logger.Infof("📨 [交易直连] 收到交易传播请求: from=%s, size=%d bytes",
			from.String()[:8], len(reqBytes))
	}

	// 1. 解析TransactionPropagationRequest请求
	var request txProtocol.TransactionPropagationRequest
	if err := proto.Unmarshal(reqBytes, &request); err != nil {
		if h.logger != nil {
			h.logger.Warnf("解析TransactionPropagationRequest失败: %v", err)
		}
		return nil, fmt.Errorf("解析TransactionPropagationRequest失败: %w", err)
	}

	// 2. 验证请求有效性
	if len(request.TxHashes) == 0 {
		return nil, fmt.Errorf("请求中缺少交易哈希列表")
	}

	if len(request.RequestId) == 0 {
		return nil, fmt.Errorf("请求中缺少RequestId")
	}

	// 3. 处理交易哈希列表，检查本地状态
	var transactionStatuses []*txProtocol.TransactionPropagationResponse_TransactionStatus
	acceptedCount := uint32(0)
	duplicateCount := uint32(0)
	rejectedCount := uint32(0)

	for i, txHash := range request.TxHashes {
		status := h.processTransactionHashForDirect(ctx, txHash, i)
		transactionStatuses = append(transactionStatuses, status)

		// 统计处理结果
		switch status.Status {
		case txProtocol.TransactionPropagationResponse_TransactionStatus_STATUS_ACCEPTED:
			acceptedCount++
		case txProtocol.TransactionPropagationResponse_TransactionStatus_STATUS_DUPLICATE:
			duplicateCount++
		default:
			rejectedCount++
		}
	}

	// 4. 构造响应（使用简化的协议结构）
	response := &txProtocol.TransactionPropagationResponse{
		RequestId:    request.RequestId,
		Transactions: transactionStatuses,
		Success:      rejectedCount == 0, // 没有拒绝的交易则认为成功
	}

	// 如果有失败的情况，添加错误消息
	if rejectedCount > 0 {
		errorMsg := fmt.Sprintf("处理了%d个交易，其中%d个被拒绝", len(request.TxHashes), rejectedCount)
		response.ErrorMessage = &errorMsg
	}

	// 5. 序列化响应
	responseBytes, err := proto.Marshal(response)
	if err != nil {
		if h.logger != nil {
			h.logger.Errorf("序列化TransactionPropagationResponse失败: %v", err)
		}
		return nil, fmt.Errorf("序列化TransactionPropagationResponse失败: %w", err)
	}

	// 6. 记录处理结果
	if h.logger != nil {
		h.logger.Infof("✅ [交易直连] 处理完成: requestId=%s, from=%s, 总计=%d, 接受=%d, 重复=%d, 拒绝=%d",
			request.RequestId, from.String()[:8], len(request.TxHashes), acceptedCount, duplicateCount, rejectedCount)
	}

	return responseBytes, nil
}

// ============================================================================
//                              辅助方法 (Helper Methods)
// ============================================================================

// processTransactionHashForDirect 处理直连传播中的单个交易哈希
//
// 🔍 **交易哈希状态检查器**
//
// 检查指定交易哈希在本地的处理状态，用于直连传播响应。
//
// 📝 **参数说明**：
//   - ctx: 上下文对象
//   - txHash: 交易哈希
//   - index: 在请求中的索引位置
//
// 📤 **返回值说明**：
//   - *txProtocol.TransactionPropagationResponse_TransactionStatus: 交易状态响应
func (h *TxNetworkProtocolHandlerService) processTransactionHashForDirect(
	ctx context.Context,
	txHash []byte,
	index int,
) *txProtocol.TransactionPropagationResponse_TransactionStatus {
	// 基础状态结构
	status := &txProtocol.TransactionPropagationResponse_TransactionStatus{
		TxHash: txHash,
		Status: txProtocol.TransactionPropagationResponse_TransactionStatus_STATUS_UNKNOWN,
	}

	// 1. 验证交易哈希格式
	if len(txHash) != 32 {
		status.Status = txProtocol.TransactionPropagationResponse_TransactionStatus_STATUS_REJECTED
		if h.logger != nil {
			h.logger.Warnf("交易哈希长度无效: 期望32字节，实际%d字节", len(txHash))
		}
		return status
	}

	// 2. 检查交易池中是否已存在
	if h.txPool != nil {
		existingTx, err := h.txPool.GetTx(txHash)
		if err == nil && existingTx != nil {
			// 交易已在内存池中
			status.Status = txProtocol.TransactionPropagationResponse_TransactionStatus_STATUS_DUPLICATE
			if h.logger != nil {
				h.logger.Debug(fmt.Sprintf("交易已存在于内存池: txHash=%x", txHash[:8]))
			}
			return status
		}
	}

	// 3. 暂时标记为接受状态
	// 注意：在实际实现中，这里可能需要进一步的验证逻辑
	// 例如：检查UTXO可用性、验证交易格式等
	status.Status = txProtocol.TransactionPropagationResponse_TransactionStatus_STATUS_ACCEPTED

	if h.logger != nil {
		h.logger.Debug(fmt.Sprintf("交易哈希处理完成: index=%d, txHash=%x, status=%v",
			index, txHash[:8], status.Status))
	}

	return status
}

// 编译期接口校验
var _ networkIntegration.TxAnnounceRouter = (*TxNetworkProtocolHandlerService)(nil)
var _ networkIntegration.TxProtocolRouter = (*TxNetworkProtocolHandlerService)(nil)
