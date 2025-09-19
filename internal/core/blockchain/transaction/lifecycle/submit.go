// Package lifecycle 交易生命周期管理 - 提交实现
//
// 🎯 **模块定位**：TransactionManager 接口的交易提交功能实现
//
// 本文件实现交易提交的核心业务逻辑，专注于：
// - 已签名交易的网络提交
// - 基本格式验证（非密码学验证）
// - 内存池提交和网络广播
// - 状态跟踪和错误处理
//
// 🏗️ **架构定位**：
// - 网络层：负责交易的网络传输和广播
// - 格式层：基本的数据完整性检查
// - 状态层：提交状态的跟踪和管理
//
// 🔧 **设计原则**：
// - 职责单一：只做网络提交，不做签名验证
// - 格式优先：基础格式检查，深度验证由mempool负责
// - 网络中立：标准化网络协议，不关心业务逻辑
// - 状态透明：详细的状态跟踪和错误诊断
//
// ⚠️ **职责边界**：
// 本文件只负责网络传输，不处理：
// - 交易签名生成（由sign.go负责）
// - 交易签名验证（由validation层负责）
// - 复杂业务逻辑（由相应业务层负责）
package lifecycle

import (
	"context"
	"fmt"

	// 公共接口
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/interfaces/repository"

	// 协议定义
	"github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbUtxo "github.com/weisyn/v1/pb/blockchain/utxo"
	txProtocol "github.com/weisyn/v1/pb/network/protocol"

	// 内部工具
	"github.com/weisyn/v1/internal/core/blockchain/transaction/internal"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// ============================================================================
//
//	交易提交实现服务
//
// ============================================================================
// TransactionSubmitService 交易提交核心实现服务
//
// 🎯 **服务职责**：
// - 实现 TransactionManager.SubmitTransaction 方法
// - 处理已签名交易的网络提交
// - 管理提交状态和错误处理
// - 执行双路径传播（GossipSub + Stream RPC）
//
// 🔧 **依赖服务**：
// - network：网络通信服务
// - txPool：交易内存池
// - cacheStore：交易缓存
// - logger：日志记录服务
//
// 📝 **使用示例**：
//
//	service := NewTransactionSubmitService(logger, cache, pool, network, ...)
//	err := service.SubmitTransaction(ctx, signedTxHash)
type TransactionSubmitService struct {
	logger      log.Logger                               // 日志记录器（可选）
	cacheStore  storage.MemoryStore                      // 缓存存储
	txPool      mempool.TxPool                           // 交易内存池
	network     network.Network                          // P2P网络服务
	repository  repository.RepositoryManager             // 数据存储管理器
	hashService transaction.TransactionHashServiceClient // 交易哈希服务（依赖注入）
	utxoManager repository.UTXOManager                   // UTXO管理器（用于UTXO状态管理）

	// 真实网络依赖
	host           node.Host                    // 节点Host接口（获取真实节点ID）
	kbucketManager kademlia.RoutingTableManager // 路由表管理器（节点选择）
}

// NewTransactionSubmitService 创建交易提交服务实例
//
// 🏗️ **构造器模式**：
// 使用依赖注入创建服务实例，确保所有依赖都已正确初始化
func NewTransactionSubmitService(
	logger log.Logger,
	cacheStore storage.MemoryStore,
	txPool mempool.TxPool,
	network network.Network,
	repository repository.RepositoryManager,
	hashService transaction.TransactionHashServiceClient,
	utxoManager repository.UTXOManager,
	host node.Host,
	kbucketManager kademlia.RoutingTableManager,
) *TransactionSubmitService {
	return &TransactionSubmitService{
		logger:         logger,
		cacheStore:     cacheStore,
		txPool:         txPool,
		network:        network,
		repository:     repository,
		hashService:    hashService,
		utxoManager:    utxoManager,
		host:           host,
		kbucketManager: kbucketManager,
	}
}

// ============================================================================
//
//	核心交易提交方法实现
//
// ============================================================================
// SubmitTransaction 提交已签名交易到网络
//
// 🎯 **方法职责**：
// 实现 blockchain.TransactionManager.SubmitTransaction 接口
// 将已签名的交易提交到网络进行传播
//
// 📋 **提交流程**：
// 1. 获取已签名的交易数据
// 2. 基本格式验证（非密码学验证）
// 3. 提交到本地内存池
// 4. 执行网络广播传播
// 5. 更新提交状态
//
// 💡 **关键原则**：
// - 只处理**已签名**交易，不进行签名生成
// - 只做**基本格式**检查，深度验证由mempool处理
// - 重点是**网络传输**，不是业务逻辑验证
func (s *TransactionSubmitService) SubmitTransaction(
	ctx context.Context,
	signedTxHash []byte,
) error {
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("开始提交交易到网络 - 哈希: %x", signedTxHash[:8]))
	}

	// 1. 基础参数验证
	if len(signedTxHash) != 32 {
		err := fmt.Errorf("签名交易哈希长度无效: 期望32字节，实际%d字节", len(signedTxHash))
		if s.logger != nil {
			s.logger.Warn(err.Error())
		}
		return err
	}

	// 2. 检查重复提交（通过交易池查询）
	existingTx, err := s.txPool.GetTx(signedTxHash)
	if err != nil {
		// 内存池未命中视为正常，继续从缓存获取并提交
		if s.logger != nil {
			s.logger.Debug("交易不在内存池中，将从缓存获取并提交")
		}
		existingTx = nil
	}
	if existingTx != nil {
		if s.logger != nil {
			s.logger.Info("交易已存在于交易池中，跳过重复提交")
		}
		return nil // 重复提交不算错误，返回成功实现幂等
	}

	// 3. 从缓存获取已签名交易
	tx, err := s.getSignedTransaction(ctx, signedTxHash)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("获取已签名交易失败: %v", err))
		}
		return fmt.Errorf("获取已签名交易失败: %v", err)
	}

	// 4. 基本格式验证（非密码学验证）
	if err := s.validateBasicFormat(tx); err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("交易格式验证失败: %v", err))
		}
		return fmt.Errorf("交易格式验证失败: %v", err)
	}

	// 5. 提交到内存池（内存池会进行深度验证）
	submittedTxHash, err := s.txPool.SubmitTx(tx)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("内存池提交失败: %v", err))
		}
		return fmt.Errorf("内存池提交失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("交易成功提交到内存池 - 哈希: %x", submittedTxHash[:8]))
	}

	// 🔥 5.5. 锁定相关UTXO状态（修复余额显示问题）
	if err := s.lockTransactionUTXOs(ctx, tx); err != nil {
		if s.logger != nil {
			s.logger.Warnf("锁定交易UTXO失败（不阻止提交流程）: %v", err)
		}
		// 注意：UTXO锁定失败不影响交易提交流程，因为内存池已经接受了交易
	}

	// 6. 网络广播传播
	if err := s.broadcastToNetwork(ctx, tx); err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("网络广播失败: %v", err))
		}
		return fmt.Errorf("网络广播失败: %v", err)
	}

	// 7. 提交完成（状态由内存池管理）

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("✅ 交易提交成功 - 哈希: %x", signedTxHash[:8]))
	}

	return nil
}

// ============================================================================
//
//	辅助方法实现
//
// ============================================================================

// getSignedTransaction 获取已签名交易
func (s *TransactionSubmitService) getSignedTransaction(ctx context.Context, txHash []byte) (*transaction.Transaction, error) {
	// 从缓存获取已签名交易
	tx, exists, err := internal.GetSignedTransactionFromCache(ctx, s.cacheStore, txHash, s.logger)
	if err != nil {
		return nil, fmt.Errorf("获取交易失败: %v", err)
	}
	if !exists {
		return nil, fmt.Errorf("已签名交易不存在于缓存中: %x", txHash)
	}
	return tx, nil
}

// validateBasicFormat 基本格式验证（非密码学验证）
func (s *TransactionSubmitService) validateBasicFormat(tx *transaction.Transaction) error {
	if tx == nil {
		return fmt.Errorf("交易对象为空")
	}

	// 基础字段检查
	if tx.Version == 0 {
		return fmt.Errorf("交易版本号无效")
	}
	if len(tx.ChainId) == 0 {
		return fmt.Errorf("链ID为空")
	}
	if tx.CreationTimestamp == 0 {
		return fmt.Errorf("创建时间戳无效")
	}

	// 输入输出基本检查
	if len(tx.Inputs) == 0 && len(tx.Outputs) == 0 {
		return fmt.Errorf("交易既无输入也无输出")
	}

	// 注意：这里不做签名验证，只做格式检查
	// 深度验证（包括签名验证）由内存池或专门的验证服务处理

	return nil
}

// broadcastToNetwork 网络广播传播
func (s *TransactionSubmitService) broadcastToNetwork(ctx context.Context, tx *transaction.Transaction) error {
	// 双路径传播：GossipSub（主要）+ Stream RPC（备份）

	// 1. GossipSub广播（主要传播路径）
	if err := s.broadcastViaGossipSub(ctx, tx); err != nil {
		if s.logger != nil {
			s.logger.Warn(fmt.Sprintf("GossipSub广播失败: %v", err))
		}
		// GossipSub失败不阻断，继续尝试Stream RPC
	}

	// 2. Stream RPC备份传播
	if err := s.broadcastViaStreamRPC(ctx, tx); err != nil {
		if s.logger != nil {
			s.logger.Warn(fmt.Sprintf("Stream RPC传播失败: %v", err))
		}
		// 两种方式都失败才返回错误
		return fmt.Errorf("所有传播路径都失败")
	}

	return nil
}

// broadcastViaGossipSub GossipSub广播
func (s *TransactionSubmitService) broadcastViaGossipSub(ctx context.Context, tx *transaction.Transaction) error {
	// 构造交易广播消息
	announcement := &txProtocol.TransactionAnnouncement{
		MessageId:       fmt.Sprintf("tx_announce_%d", tx.CreationTimestamp),
		TransactionHash: s.calculateTransactionHash(tx),
		Transaction:     tx,
		Timestamp:       uint64(tx.CreationTimestamp),
		SenderPeerId:    []byte(s.host.ID().String()),
		PropagationHop:  1,
	}

	// 序列化消息
	data, err := proto.Marshal(announcement)
	if err != nil {
		return fmt.Errorf("序列化交易广播消息失败: %w", err)
	}

	// 发布到GossipSub主题
	return s.network.Publish(ctx, protocols.TopicTransactionAnnounce, data, nil)
}

// broadcastViaStreamRPC Stream RPC备份传播
func (s *TransactionSubmitService) broadcastViaStreamRPC(ctx context.Context, tx *transaction.Transaction) error {
	// 选择K-bucket近邻节点
	nearbyPeers, err := s.selectNearbyPeers(ctx, 2)
	if err != nil {
		return fmt.Errorf("选择近邻节点失败: %w", err)
	}

	if len(nearbyPeers) == 0 {
		return fmt.Errorf("没有可用的近邻节点")
	}

	// 构造传播请求
	request := &txProtocol.TransactionPropagationRequest{
		RequestId:       fmt.Sprintf("tx_stream_%d", tx.CreationTimestamp),
		TxHashes:        [][]byte{s.calculateTransactionHash(tx)},
		RequesterPeerId: []byte(s.host.ID().String()),
		Timestamp:       uint64(tx.CreationTimestamp),
	}

	requestBytes, err := proto.Marshal(request)
	if err != nil {
		return fmt.Errorf("序列化传播请求失败: %w", err)
	}

	// 向多个节点发送Stream RPC
	for _, peerID := range nearbyPeers {
		if err := s.sendStreamRPC(ctx, peerID, requestBytes); err != nil {
			if s.logger != nil {
				s.logger.Warn(fmt.Sprintf("向节点%s发送Stream RPC失败: %v", peerID, err))
			}
			continue
		}
		// 任一成功即可
		return nil
	}

	return fmt.Errorf("所有Stream RPC调用都失败")
}

// selectNearbyPeers 选择K-bucket近邻节点
func (s *TransactionSubmitService) selectNearbyPeers(ctx context.Context, count int) ([]peer.ID, error) {
	// 使用本地节点ID作为路由键
	localID := s.host.ID()
	routingKey := []byte(localID.String())

	// 查询路由表管理器（直接调用简化方法）
	peerIDs := s.kbucketManager.FindClosestPeers(routingKey, count)

	return peerIDs, nil
}

// sendStreamRPC 发送Stream RPC
func (s *TransactionSubmitService) sendStreamRPC(ctx context.Context, peerID peer.ID, requestBytes []byte) error {
	// 使用网络服务发送Stream RPC
	respBytes, err := s.network.Call(ctx, peerID, protocols.ProtocolTransactionDirect, requestBytes, nil)
	if err != nil {
		return fmt.Errorf("Stream RPC调用失败: %w", err)
	}

	// 解析响应
	var response txProtocol.TransactionPropagationResponse
	if err := proto.Unmarshal(respBytes, &response); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("节点拒绝传播: %v", response.ErrorMessage)
	}

	return nil
}

// calculateTransactionHash 计算交易哈希
func (s *TransactionSubmitService) calculateTransactionHash(tx *transaction.Transaction) []byte {
	// 使用注入的哈希服务计算交易哈希
	hashReq := &transaction.ComputeHashRequest{
		Transaction:      tx,
		IncludeDebugInfo: false,
	}

	ctx := context.Background()
	hashResp, err := s.hashService.ComputeHash(ctx, hashReq)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("计算交易哈希失败: %v", err))
		}
		// 返回零哈希作为fallback
		return make([]byte, 32)
	}

	return hashResp.Hash
}

// lockTransactionUTXOs 锁定交易输入中的AssetUTXO状态
//
// 🔒 **UTXO状态锁定核心实现**
//
// 当交易成功提交到内存池后，将交易输入引用的AssetUTXO状态从AVAILABLE改为REFERENCED，
// 解决用户提交交易后余额显示不准确的问题。
//
// 实现要点：
// - 只处理AssetUTXO（包含原生币和代币）
// - ResourceUTXO已有独立的引用机制，不需要额外处理
// - 锁定失败不影响交易提交流程（已经在内存池中）
// - 使用UTXO管理器的ReferenceUTXO方法实现状态切换
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 交易对象
//
// 返回：
//   - error: 锁定错误（不影响交易提交）
func (s *TransactionSubmitService) lockTransactionUTXOs(ctx context.Context, tx *transaction.Transaction) error {
	if s.utxoManager == nil {
		return fmt.Errorf("UTXO管理器未初始化")
	}

	if s.logger != nil {
		s.logger.Debugf("开始锁定交易UTXO - 输入数量: %d", len(tx.Inputs))
	}

	var lockErrors []error
	lockedCount := 0

	// 遍历交易输入，锁定相关的AssetUTXO
	for i, input := range tx.Inputs {
		if input == nil || input.PreviousOutput == nil {
			continue
		}

		// 🔍 先查询UTXO确认其类型
		utxo, err := s.utxoManager.GetUTXO(ctx, input.PreviousOutput)
		if err != nil {
			lockErrors = append(lockErrors, fmt.Errorf("输入%d: 查询UTXO失败: %w", i, err))
			continue
		}
		if utxo == nil {
			lockErrors = append(lockErrors, fmt.Errorf("输入%d: UTXO不存在", i))
			continue
		}

		// 🎯 只锁定AssetUTXO（ResourceUTXO有独立的引用机制）
		if utxo.GetCategory() == pbUtxo.UTXOCategory_UTXO_CATEGORY_ASSET {
			// 使用ReferenceUTXO方法将状态从AVAILABLE改为REFERENCED
			err := s.utxoManager.ReferenceUTXO(ctx, input.PreviousOutput)
			if err != nil {
				lockErrors = append(lockErrors, fmt.Errorf("输入%d: 锁定AssetUTXO失败: %w", i, err))
				continue
			}
			lockedCount++

			if s.logger != nil {
				s.logger.Debugf("✅ 输入%d AssetUTXO锁定成功 - OutPoint: %x:%d",
					i, input.PreviousOutput.TxId[:8], input.PreviousOutput.OutputIndex)
			}
		} else {
			// ResourceUTXO跳过，它们有独立的引用计数机制
			if s.logger != nil {
				s.logger.Debugf("⏭️ 输入%d 为ResourceUTXO，跳过锁定", i)
			}
		}
	}

	// 记录锁定结果
	if s.logger != nil {
		if len(lockErrors) > 0 {
			s.logger.Warnf("UTXO锁定完成 - 成功: %d个, 失败: %d个", lockedCount, len(lockErrors))
			for _, err := range lockErrors {
				s.logger.Warnf("锁定错误: %v", err)
			}
		} else {
			s.logger.Infof("✅ 所有AssetUTXO锁定成功 - 总计: %d个", lockedCount)
		}
	}

	// 如果有锁定错误，返回汇总错误（但不影响交易提交）
	if len(lockErrors) > 0 {
		return fmt.Errorf("部分UTXO锁定失败 (%d/%d): %v", len(lockErrors), len(tx.Inputs), lockErrors[0])
	}

	return nil
}
