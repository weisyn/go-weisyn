// produce_block.go 实现从候选模板生成完整区块的具体逻辑
//
// 🏗️ **区块生成和PoW计算流程实现**
//
// 本文件实现：
// - 候选区块模板验证和处理
// - 类型安全检查和转换
// - 多线程PoW计算调用
// - 完整区块组装和验证
// - 区块完整性和一致性检查
package pow_handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pb/blockchain/block/transaction"
	"google.golang.org/protobuf/proto"
)

// produceBlockFromTemplate 从候选模板生成完整区块的具体实现
func (s *PoWComputeService) produceBlockFromTemplate(ctx context.Context, candidateBlock interface{}) (interface{}, error) {
	s.logger.Info("开始从模板生成区块")

	// 1. 类型验证和转换
	block, err := s.validateAndConvertTemplate(candidateBlock)
	if err != nil {
		return nil, fmt.Errorf("模板验证失败: %w", err)
	}

	// 2. 检查引擎运行状态
	if !s.IsRunning() {
		return nil, fmt.Errorf("PoW引擎未启动，请先启动矿工服务")
	}

	// 3. 预处理区块模板
	processedBlock, err := s.preprocessBlockTemplate(ctx, block)
	if err != nil {
		return nil, fmt.Errorf("预处理区块模板失败: %w", err)
	}

	// 4. 执行PoW计算
	minedBlock, err := s.performBlockMining(ctx, processedBlock)
	if err != nil {
		return nil, fmt.Errorf("区块挖矿失败: %w", err)
	}

	// 5. 后处理和验证
	finalBlock, err := s.postprocessMinedBlock(ctx, minedBlock)
	if err != nil {
		return nil, fmt.Errorf("后处理挖矿结果失败: %w", err)
	}

	// 6. 完整性验证
	if err := s.validateCompleteBlock(ctx, finalBlock); err != nil {
		return nil, fmt.Errorf("完整性验证失败: %w", err)
	}

	s.logger.Info("从模板生成区块完成")
	return finalBlock, nil
}

// validateAndConvertTemplate 验证和转换候选模板
func (s *PoWComputeService) validateAndConvertTemplate(candidateBlock interface{}) (*core.Block, error) {
	// 类型断言：仅支持 *core.Block
	block, ok := candidateBlock.(*core.Block)
	if !ok {
		return nil, fmt.Errorf("不支持的候选区块类型，仅支持 *core.Block，实际类型: %T", candidateBlock)
	}

	// 基础有效性检查
	if block == nil {
		return nil, fmt.Errorf("区块不能为空")
	}

	if block.Header == nil {
		return nil, fmt.Errorf("区块头不能为空")
	}

	// 区块头关键字段检查
	if block.Header.Version == 0 {
		return nil, fmt.Errorf("区块版本号不能为0")
	}

	if len(block.Header.PreviousHash) == 0 {
		return nil, fmt.Errorf("前区块哈希不能为空")
	}

	if len(block.Header.MerkleRoot) == 0 {
		return nil, fmt.Errorf("Merkle根不能为空")
	}

	if block.Header.Timestamp == 0 {
		return nil, fmt.Errorf("时间戳不能为0")
	}

	if block.Header.Difficulty == 0 {
		return nil, fmt.Errorf("难度值不能为0，请检查区块创建流程")
	}

	// 区块体检查
	if block.Body == nil {
		return nil, fmt.Errorf("区块体不能为空")
	}

	s.logger.Info("候选区块模板验证通过")
	return block, nil
}

// preprocessBlockTemplate 预处理区块模板
func (s *PoWComputeService) preprocessBlockTemplate(ctx context.Context, block *core.Block) (*core.Block, error) {
	s.logger.Info("预处理区块模板")

	// 创建区块的深拷贝，避免修改原始模板
	processedBlock := s.createBlockDeepCopy(block)

	// 重置nonce（确保从0开始挖矿）
	processedBlock.Header.Nonce = make([]byte, 8) // 8字节全0

	// 验证Merkle根（确保与交易列表一致）
	if err := s.validateMerkleRoot(ctx, processedBlock); err != nil {
		return nil, fmt.Errorf("Merkle根验证失败: %w", err)
	}

	// 设置挖矿开始时的时间戳（可选，保持原时间戳）
	// processedBlock.Header.Timestamp = uint64(time.Now().Unix())

	s.logger.Info("区块模板预处理完成")
	return processedBlock, nil
}

// performBlockMining 执行区块挖矿
func (s *PoWComputeService) performBlockMining(ctx context.Context, block *core.Block) (*core.Block, error) {
	s.logger.Info("开始区块挖矿计算")

	// 不在 PoW 计算层强制设置超时。
	// - PoW 是概率过程，强制超时会在高难度/低算力场景下造成持续 cancel，表现为“卡高度”；
	// - 需要停止挖矿时，应由外层 ctx 取消（例如节点关闭、运维明确配置的 mining_timeout）。
	miningCtx := ctx

	// 调用挖矿算法（委托给 mine_block_header.go）
	minedHeader, err := s.mineBlockHeader(miningCtx, block.Header)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("区块头挖矿超时: %w", err)
		}
		return nil, fmt.Errorf("区块头挖矿失败: %w", err)
	}

	// 创建挖矿后的完整区块
	minedBlock := &core.Block{
		Header: minedHeader,
		Body:   block.Body, // 保持原始区块体不变
	}

	s.logger.Info("区块挖矿计算完成")
	return minedBlock, nil
}

// postprocessMinedBlock 后处理挖矿后的区块
func (s *PoWComputeService) postprocessMinedBlock(ctx context.Context, minedBlock *core.Block) (*core.Block, error) {
	s.logger.Info("后处理挖矿区块")

	// 验证挖矿结果
	isValid, err := s.verifyBlockHeader(minedBlock.Header)
	if err != nil {
		return nil, fmt.Errorf("验证挖矿结果失败: %w", err)
	}

	if !isValid {
		return nil, fmt.Errorf("挖矿结果PoW验证失败")
	}

	// 创建最终区块（再次深拷贝，确保数据安全）
	finalBlock := s.createBlockDeepCopy(minedBlock)

	s.logger.Info("挖矿区块后处理完成")
	return finalBlock, nil
}

// validateCompleteBlock 验证完整区块
func (s *PoWComputeService) validateCompleteBlock(ctx context.Context, block *core.Block) error {
	s.logger.Info("验证完整区块")

	// 1. 基础结构验证
	if block == nil || block.Header == nil || block.Body == nil {
		return fmt.Errorf("区块结构不完整")
	}

	// 2. PoW验证
	isValid, err := s.verifyBlockHeader(block.Header)
	if err != nil {
		return fmt.Errorf("PoW验证出错: %w", err)
	}

	if !isValid {
		return fmt.Errorf("PoW验证失败")
	}

	// 3. Merkle根一致性验证
	if err := s.validateMerkleRoot(ctx, block); err != nil {
		return fmt.Errorf("Merkle根验证失败: %w", err)
	}

	// 4. 区块头字段合理性检查
	if err := s.validateBlockHeaderFields(block.Header); err != nil {
		return fmt.Errorf("区块头字段验证失败: %w", err)
	}

	s.logger.Info("完整区块验证通过")
	return nil
}

// createBlockDeepCopy 创建区块的深拷贝
//
// 🎯 **拷贝目的**：
// 1. **数据隔离**：避免对原始输入区块的意外修改（特别是nonce重置）
// 2. **线程安全**：确保并行PoW计算时不会互相干扰
// 3. **防止数据竞争**：避免多个挖矿线程同时修改同一区块对象
//
// ⚠️ **重要性**：
// - 在预处理阶段：避免修改原始模板（特别是nonce重置）
// - 在后处理阶段：确保输出数据不被外部修改
func (s *PoWComputeService) createBlockDeepCopy(block *core.Block) *core.Block {
	if block == nil {
		return nil
	}

	// 使用protobuf的Clone方法进行完整深拷贝
	// 这种方式更安全、简洁，且能自动处理所有字段（包括未来新增的）
	blockCopy := proto.Clone(block).(*core.Block)

	return blockCopy
}

// validateMerkleRoot 验证Merkle根
func (s *PoWComputeService) validateMerkleRoot(ctx context.Context, block *core.Block) error {
	s.logger.Debug("开始验证Merkle根")

	// 在长计算前先检查上下文是否已取消，避免在已超时/取消的挖矿轮次上继续消耗资源
	if err := ctx.Err(); err != nil {
		return err
	}

	// 1. 参数校验
	if len(block.Header.MerkleRoot) != 32 {
		return fmt.Errorf("Merkle根长度应为32字节，实际长度: %d", len(block.Header.MerkleRoot))
	}

	if s.merkleTreeManager == nil {
		return fmt.Errorf("MerkleTreeManager未注入")
	}

	// 2. 特殊情况：没有交易时
	if len(block.Body.Transactions) == 0 {
		// 空区块的Merkle根应该是全零
		emptyRoot := make([]byte, 32)
		if !bytes.Equal(block.Header.MerkleRoot, emptyRoot) {
			return fmt.Errorf("空区块的Merkle根应为全零")
		}
		return nil
	}

	// 3. 构建交易哈希列表
	s.logger.Infof("🔧 [PoWHandler] 开始验证Merkle根，交易数: %d", len(block.Body.Transactions))
	transactionHashes, err := s.buildTransactionHashList(ctx, block.Body.Transactions)
	if err != nil {
		return fmt.Errorf("构建交易哈希列表失败: %v", err)
	}

	if len(transactionHashes) > 0 {
		s.logger.Infof("🔧 [PoWHandler] 第一笔交易哈希: %x", transactionHashes[0][:16])
	}

	// 4. 🔧 直接从交易哈希构建 Merkle 树（与 BlockBuilder 保持一致）
	// 注意：不使用 merkleTreeManager.NewMerkleTree，因为它会对哈希再做一次 SHA256
	calculatedRoot, err := s.buildMerkleTreeFromHashes(transactionHashes)
	if err != nil {
		return fmt.Errorf("构建Merkle树失败: %v", err)
	}

	// 5. 验证 Merkle 根长度
	if len(calculatedRoot) != 32 {
		return fmt.Errorf("计算出的Merkle根长度异常: %d", len(calculatedRoot))
	}

	s.logger.Infof("🔧 [PoWHandler] 计算的Merkle根: %x", calculatedRoot[:16])
	s.logger.Infof("🔧 [PoWHandler] 区块中的Merkle根: %x", block.Header.MerkleRoot[:16])

	// 6. 比较Merkle根
	if !bytes.Equal(block.Header.MerkleRoot, calculatedRoot) {
		s.logger.Errorf("Merkle根不匹配，期望: %x, 实际: %x",
			block.Header.MerkleRoot, calculatedRoot)
		return fmt.Errorf("Merkle根不匹配")
	}

	s.logger.Debug("Merkle根验证成功")
	return nil
}

// validateBlockHeaderFields 验证区块头字段合理性
func (s *PoWComputeService) validateBlockHeaderFields(header *core.BlockHeader) error {
	// 版本号检查
	if header.Version == 0 || header.Version > 1000 {
		return fmt.Errorf("区块版本号异常: %d", header.Version)
	}

	// 哈希长度检查
	if len(header.PreviousHash) != 32 {
		return fmt.Errorf("前区块哈希长度应为32字节")
	}

	if len(header.MerkleRoot) != 32 {
		return fmt.Errorf("Merkle根长度应为32字节")
	}

	// nonce长度检查
	if len(header.Nonce) != 8 {
		return fmt.Errorf("nonce长度应为8字节")
	}

	// 难度值检查
	if header.Difficulty == 0 {
		return fmt.Errorf("难度值不能为0")
	}

	// 时间戳合理性检查（不能太早或太晚）
	// currentTime := uint64(time.Now().Unix())
	// if header.Timestamp > currentTime + 300 { // 不能超过当前时间5分钟
	//     return fmt.Errorf("区块时间戳过于未来: %d", header.Timestamp)
	// }

	return nil
}

// ==================== 辅助方法 ====================

// buildTransactionHashList 构建交易哈希列表
//
// 🎯 **交易哈希计算（统一路径）**
//
// 为Merkle树构建准备交易哈希列表。使用统一的交易哈希服务计算哈希，
// 确保与区块头构建阶段完全一致（避免Merkle根不匹配问题）。
//
// ⚠️ **重要**：必须使用 TransactionHashServiceClient 统一计算交易哈希，
// 不能使用本地序列化+哈希的方式，以保证确定性和一致性。
//
// 参数：
//
//	transactions: 交易列表
//
// 返回值：
//
//	[][]byte: 交易哈希列表
//	error: 计算过程中的错误
func (s *PoWComputeService) buildTransactionHashList(ctx context.Context, transactions []*transaction.Transaction) ([][]byte, error) {
	transactionHashes := make([][]byte, len(transactions))

	for i, tx := range transactions {
		// 在循环中尊重 ctx，允许上层在取消挖矿轮次时尽快中断计算
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// 使用统一的交易哈希服务计算哈希（确定性）
		// 这确保了与区块头构建阶段的哈希计算完全一致
		req := &transaction.ComputeHashRequest{
			Transaction:      tx,
			IncludeDebugInfo: false,
		}

		resp, err := s.txHashClient.ComputeHash(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("计算交易[%d]哈希失败: %v", i, err)
		}

		if resp == nil || !resp.IsValid || len(resp.Hash) == 0 {
			return nil, fmt.Errorf("交易[%d]哈希无效", i)
		}

		transactionHashes[i] = resp.Hash
	}

	return transactionHashes, nil
}

// buildMerkleTreeFromHashes 从交易哈希列表构建Merkle树
// 🔧 与 BlockBuilder 保持完全一致的算法
func (s *PoWComputeService) buildMerkleTreeFromHashes(hashes [][]byte) ([]byte, error) {
	// 如果节点数为奇数，复制最后一个节点
	if len(hashes)%2 == 1 {
		hashes = append(hashes, hashes[len(hashes)-1])
	}

	// 基础情况：2个节点配对后返回
	if len(hashes) == 2 {
		combined := append(hashes[0], hashes[1]...)
		parentHash := s.hashManager.SHA256(combined)
		return parentHash, nil
	}

	// 计算下一层节点
	nextLevel := make([][]byte, 0, len(hashes)/2)
	for i := 0; i < len(hashes); i += 2 {
		// 连接两个子节点的哈希
		combined := append(hashes[i], hashes[i+1]...)

		// 计算父节点哈希
		parentHash := s.hashManager.SHA256(combined)

		nextLevel = append(nextLevel, parentHash)
	}

	// 递归处理下一层
	return s.buildMerkleTreeFromHashes(nextLevel)
}
