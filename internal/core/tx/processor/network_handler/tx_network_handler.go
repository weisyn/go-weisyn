// Package network_handler 实现交易网络协议处理服务
//
// 🎯 **交易网络协议处理服务模块**
//
// 本包实现 TxProtocolRouter 和 TxAnnounceRouter 接口，提供交易网络协议处理功能：
// - 实现TxProtocolRouter接口（流式协议处理）
// - 实现TxAnnounceRouter接口（订阅协议处理）
// - 支持交易双重保障传播机制
//
// 设计理念：
// - 薄委托层：只负责网络消息的接收和转发
// - 职责单一：解析protobuf → 去重检查 → 委托验证器 → 提交到池
// - 无状态：不维护交易状态，只做流程编排
package network_handler

import (
	"context"
	"fmt"

	peer "github.com/libp2p/go-libp2p/core/peer"
	txProtocol "github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"google.golang.org/protobuf/proto"
)

// NetworkHandler 交易网络协议处理器
//
// 🎯 **职责定位**：
// - 实现 integration/network.TxAnnounceRouter 接口（订阅协议）
// - 实现 integration/network.TxProtocolRouter 接口（流式协议）
// - 处理来自P2P网络的交易公告消息和交易中继请求
// - 执行：解码 → 去重 → 验证 → 入池的完整流程
//
// 🏗️ **设计原则**：
// - 薄委托层：不实现业务逻辑，只做流程编排
// - 统一归口：处理所有交易相关的网络消息
// - 依赖注入：通过接口获取验证服务和交易池
type NetworkHandler struct {
	txPool mempool.TxPool // 交易池服务
	logger log.Logger     // 日志服务
}

// NewNetworkHandler 创建交易网络协议处理器实例
//
// 参数:
//
//	txPool: 交易池服务，用于提交验证通过的交易
//	logger: 日志服务，用于记录处理过程
//
// 返回:
//
//	*NetworkHandler: 交易网络协议处理器实例
func NewNetworkHandler(
	txPool mempool.TxPool,
	logger log.Logger,
) *NetworkHandler {
	return &NetworkHandler{
		txPool: txPool,
		logger: logger,
	}
}

// HandleTransactionAnnounce 处理交易公告
//
// 🎯 **实现 integration/network.TxAnnounceRouter 接口**
//
// 处理流程：
// 1. 解析标准protobuf交易公告数据（包含完整交易）
// 2. 验证交易公告的完整性
// 3. 去重检查：确保交易未在本地内存池中
// 4. 提交到池：TxPool内部会执行验证和广播
//
// 参数:
//   - ctx: 上下文对象
//   - from: 发送方peer ID
//   - topic: 公告主题
//   - data: 交易公告的protobuf序列化数据
//
// 返回:
//   - error: 处理过程中的错误，nil表示成功
func (h *NetworkHandler) HandleTransactionAnnounce(ctx context.Context, from peer.ID, topic string, data []byte) error {
	// 防御性：计算安全的节点ID短串用于日志
	fromStr := from.String()
	if len(fromStr) > 8 {
		fromStr = fromStr[:8]
	}

	if h.logger != nil {
		h.logger.Debugf("[TxProcessor/Network] 处理交易公告: from=%s, topic=%s, size=%d", fromStr, topic, len(data))
	}

	// 防御性：确保 txPool 已注入
	if h.txPool == nil {
		return fmt.Errorf("txPool 未初始化")
	}

	// 1. 解析标准protobuf TransactionAnnouncement消息
	var announcement txProtocol.TransactionAnnouncement
	if err := proto.Unmarshal(data, &announcement); err != nil {
		if h.logger != nil {
			h.logger.Warnf("[TxProcessor/Network] 解析TransactionAnnouncement失败: %v", err)
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
			h.logger.Debug(fmt.Sprintf("[TxProcessor/Network] 交易已存在于内存池中，跳过处理: txHash=%x", txHash[:8]))
		}
		return nil // 重复交易，不算错误
	}

	// 4. 提交到内存池（TxPool内部会执行验证）
	submittedTxHash, err := h.txPool.SubmitTx(announcement.Transaction)
	if err != nil {
		if h.logger != nil {
			h.logger.Errorf("[TxProcessor/Network] 提交到内存池失败: txHash=%x, error=%v", txHash[:8], err)
		}
		return fmt.Errorf("提交到内存池失败: %w", err)
	}

	// 5. 记录处理成功
	if h.logger != nil {
		h.logger.Infof("[TxProcessor/Network] ✅ 交易公告处理完成: txHash=%x, submittedHash=%x, messageId=%s, from=%s",
			txHash[:8], submittedTxHash[:8], announcement.MessageId, fromStr)
	}

	return nil
}

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
// 参数：
//   - ctx: 上下文（用于超时控制）
//   - from: 发送方节点ID
//   - reqBytes: 序列化的TransactionPropagationRequest数据
//
// 返回：
//   - []byte: 序列化的TransactionPropagationResponse数据
//   - error: 处理失败时的错误信息
func (h *NetworkHandler) HandleTransactionDirect(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	if h.logger != nil {
		h.logger.Infof("[TxProcessor/Network] 📨 收到交易传播请求: from=%s, size=%d bytes",
			from.String()[:8], len(reqBytes))
	}

	// 1. 解析TransactionPropagationRequest请求
	var request txProtocol.TransactionPropagationRequest
	if err := proto.Unmarshal(reqBytes, &request); err != nil {
		if h.logger != nil {
			h.logger.Warnf("[TxProcessor/Network] 解析TransactionPropagationRequest失败: %v", err)
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
		status := h.processTransactionHash(ctx, txHash, i)
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

	// 4. 构造响应
	response := &txProtocol.TransactionPropagationResponse{
		RequestId:    request.RequestId,
		Transactions: transactionStatuses,
		Success:      rejectedCount == 0,
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
			h.logger.Errorf("[TxProcessor/Network] 序列化TransactionPropagationResponse失败: %v", err)
		}
		return nil, fmt.Errorf("序列化TransactionPropagationResponse失败: %w", err)
	}

	// 6. 记录处理结果
	if h.logger != nil {
		h.logger.Infof("[TxProcessor/Network] ✅ 处理完成: requestId=%s, from=%s, 总计=%d, 接受=%d, 重复=%d, 拒绝=%d",
			request.RequestId, from.String()[:8], len(request.TxHashes), acceptedCount, duplicateCount, rejectedCount)
	}

	return responseBytes, nil
}

// processTransactionHash 处理直连传播中的单个交易哈希
func (h *NetworkHandler) processTransactionHash(
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
			h.logger.Warnf("[TxProcessor/Network] 交易哈希长度无效: 期望32字节，实际%d字节", len(txHash))
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
				h.logger.Debug(fmt.Sprintf("[TxProcessor/Network] 交易已存在于内存池: txHash=%x", txHash[:8]))
			}
			return status
		}
	}

	// 3. 标记为接受状态（等待后续完整交易数据）
	status.Status = txProtocol.TransactionPropagationResponse_TransactionStatus_STATUS_ACCEPTED

	if h.logger != nil {
		h.logger.Debug(fmt.Sprintf("[TxProcessor/Network] 交易哈希处理完成: index=%d, txHash=%x, status=%v",
			index, txHash[:8], status.Status))
	}

	return status
}
