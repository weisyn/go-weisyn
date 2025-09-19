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
	processedBlock, err := s.preprocessBlockTemplate(block)
	if err != nil {
		return nil, fmt.Errorf("预处理区块模板失败: %w", err)
	}

	// 4. 执行PoW计算
	minedBlock, err := s.performBlockMining(ctx, processedBlock)
	if err != nil {
		return nil, fmt.Errorf("区块挖矿失败: %w", err)
	}

	// 5. 后处理和验证
	finalBlock, err := s.postprocessMinedBlock(minedBlock)
	if err != nil {
		return nil, fmt.Errorf("后处理挖矿结果失败: %w", err)
	}

	// 6. 完整性验证
	if err := s.validateCompleteBlock(finalBlock); err != nil {
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
func (s *PoWComputeService) preprocessBlockTemplate(block *core.Block) (*core.Block, error) {
	s.logger.Info("预处理区块模板")

	// 创建区块的深拷贝，避免修改原始模板
	processedBlock := s.createBlockDeepCopy(block)

	// 重置nonce（确保从0开始挖矿）
	processedBlock.Header.Nonce = make([]byte, 8) // 8字节全0

	// 验证Merkle根（确保与交易列表一致）
	if err := s.validateMerkleRoot(processedBlock); err != nil {
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

	// 调用多线程挖矿算法（委托给 mine_block_header.go）
	minedHeader, err := s.mineBlockHeader(ctx, block.Header)
	if err != nil {
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
func (s *PoWComputeService) postprocessMinedBlock(minedBlock *core.Block) (*core.Block, error) {
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
func (s *PoWComputeService) validateCompleteBlock(block *core.Block) error {
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
	if err := s.validateMerkleRoot(block); err != nil {
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
func (s *PoWComputeService) validateMerkleRoot(block *core.Block) error {
	s.logger.Debug("开始验证Merkle根")

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
	transactionHashes, err := s.buildTransactionHashList(block.Body.Transactions)
	if err != nil {
		return fmt.Errorf("构建交易哈希列表失败: %v", err)
	}

	// 4. 使用MerkleTreeManager构建Merkle树
	merkleTree, err := s.merkleTreeManager.NewMerkleTree(transactionHashes)
	if err != nil {
		return fmt.Errorf("构建Merkle树失败: %v", err)
	}

	// 5. 获取计算出的Merkle根
	calculatedRoot := merkleTree.GetRoot()
	if len(calculatedRoot) != 32 {
		return fmt.Errorf("计算出的Merkle根长度异常: %d", len(calculatedRoot))
	}

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
// 🎯 **交易哈希计算**
//
// 为Merkle树构建准备交易哈希列表。每个交易通过Protobuf序列化后
// 计算哈希值，确保与区块链标准兼容。
//
// 参数：
//
//	transactions: 交易列表
//
// 返回值：
//
//	[][]byte: 交易哈希列表
//	error: 计算过程中的错误
func (s *PoWComputeService) buildTransactionHashList(transactions []*transaction.Transaction) ([][]byte, error) {
	transactionHashes := make([][]byte, len(transactions))

	for i, tx := range transactions {
		// 序列化交易
		txBytes, err := proto.Marshal(tx)
		if err != nil {
			return nil, fmt.Errorf("序列化交易[%d]失败: %v", i, err)
		}

		// 使用HashManager计算真正的交易哈希
		transactionHashes[i] = s.hashManager.SHA256(txBytes)
	}

	return transactionHashes, nil
}
