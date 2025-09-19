// Package block 提供区块管理的核心实现
//
// 📋 **hash_utils.go - 区块哈希计算辅助工具**
//
// 本文件提供区块哈希计算相关的辅助工具方法，确保哈希计算的标准化和一致性。
// 支持区块哈希、Merkle根哈希、POW验证等关键密码学操作。
//
// 🎯 **核心职责**：
// - 标准化哈希计算：确保跨平台一致的哈希结果
// - Merkle树构建：构建交易的Merkle树并计算根哈希
// - POW难度验证：验证区块哈希是否满足难度要求
// - 哈希格式转换：支持不同格式的哈希表示和转换
// - 性能优化：提供高效的哈希计算实现
//
// 🏗️ **架构特点**：
// - 确定性计算：相同输入保证相同输出
// - 标准兼容：符合区块链标准哈希算法
// - 性能优化：使用高效的哈希实现
// - 安全保证：防止哈希碰撞和攻击
//
// 详细设计文档：internal/core/blockchain/block/README.md
package block

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== 哈希算法常量 ====================

// HashConstants 哈希计算相关常量
const (
	// 标准哈希长度（字节）
	StandardHashLength = 32

	// SHA-256 算法标识
	SHA256Algorithm = "SHA256"

	// Merkle树叶子节点前缀（防止长度扩展攻击）
	MerkleLeafPrefix = 0x00

	// Merkle树内部节点前缀
	MerkleInternalPrefix = 0x01

	// 区块哈希计算前缀
	BlockHashPrefix = "WES_BLOCK:"

	// 交易哈希计算前缀
	TransactionHashPrefix = "WES_TX:"
)

// DifficultyConstants POW难度相关常量
var (
	// 最大目标值（最小难度）
	MaxTarget = big.NewInt(0).Lsh(big.NewInt(1), 256-32) // 2^(256-32)

	// 最小目标值（最大难度）
	MinTarget = big.NewInt(1)

	// 难度调整因子（防止难度变化过大）
	MaxDifficultyAdjustmentFactor = 4.0
	MinDifficultyAdjustmentFactor = 0.25
)

// ==================== 标准化哈希计算 ====================

// computeStandardBlockHash 计算标准区块哈希
//
// 🎯 **标准化区块哈希计算**
//
// 使用标准化的方法计算区块哈希，确保跨平台一致性。
// 调用 gRPC BlockHashService 进行确定性哈希计算。
//
// 🔄 **哈希计算流程**：
//
// 1. **预处理验证**：
//   - 验证区块结构的完整性
//   - 检查必需字段的存在性
//   - 确保区块格式符合协议要求
//
// 2. **gRPC服务调用**：
//   - 构造 ComputeBlockHashRequest
//   - 调用 BlockHashServiceClient.ComputeBlockHash
//   - 获取标准化的哈希结果
//
// 3. **结果验证**：
//   - 验证哈希长度的正确性
//   - 检查哈希计算的有效性标志
//   - 确保结果符合预期格式
//
// 4. **调试信息处理**：
//   - 记录哈希计算的调试信息
//   - 提供性能监控数据
//   - 支持问题排查和优化
//
// 🎯 **标准化保证**：
// - **算法固定**：统一使用SHA-256算法
// - **序列化标准**：使用Protobuf确定性序列化
// - **字段顺序**：按照协议定义的标准顺序
// - **跨平台一致**：不同系统计算相同区块得到相同哈希
//
// 参数：
//
//	ctx: 上下文对象，用于超时控制
//	block: 待计算哈希的区块
//
// 返回值：
//
//	[]byte: 32字节的标准化区块哈希
//	error: 计算过程中的错误，nil 表示计算成功
func (m *Manager) computeStandardBlockHash(ctx context.Context, block *core.Block) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debugf("计算标准区块哈希，高度: %d", block.Header.Height)
	}

	// 验证区块基础结构
	if block == nil {
		return nil, fmt.Errorf("区块为空，无法计算哈希")
	}
	if block.Header == nil {
		return nil, fmt.Errorf("区块头为空，无法计算哈希")
	}
	if block.Body == nil {
		return nil, fmt.Errorf("区块体为空，无法计算哈希")
	}

	// 构造 ComputeBlockHashRequest
	req := &core.ComputeBlockHashRequest{
		Block:            block,
		IncludeDebugInfo: false, // 在生产环境中通常不需要调试信息
	}

	// 调用 gRPC BlockHashService 计算哈希
	resp, err := m.blockHashServiceClient.ComputeBlockHash(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用区块哈希服务失败: %w", err)
	}

	// 验证响应结果
	if resp == nil {
		return nil, fmt.Errorf("区块哈希服务返回空响应")
	}
	if !resp.IsValid {
		return nil, fmt.Errorf("区块哈希计算失败：区块格式无效")
	}
	if len(resp.Hash) != StandardHashLength {
		return nil, fmt.Errorf("区块哈希长度不正确: 期望 %d 字节, 实际 %d 字节",
			StandardHashLength, len(resp.Hash))
	}

	if m.logger != nil {
		m.logger.Debugf("成功计算区块哈希，长度: %d", len(resp.Hash))
	}

	return resp.Hash, nil
}

// computeLocalBlockHash 计算本地区块哈希
//
// 🎯 **本地哈希计算（备用方案）**
//
// 作为gRPC服务的备用方案，提供本地的区块哈希计算能力。
// 实现与gRPC服务相同的算法逻辑。
//
// 计算方法：
// - 使用SHA-256算法
// - 基于区块头的标准化序列化
// - 遵循相同的字段包含/排除规则
//
// 参数：
//
//	block: 待计算哈希的区块
//
// 返回值：
//
//	[]byte: 32字节区块哈希
//	error: 计算过程中的错误
func (m *Manager) computeLocalBlockHash(block *core.Block) ([]byte, error) {
	// TODO: 实现本地区块哈希计算逻辑
	//
	// 实现步骤：
	// 1. 序列化区块头（排除可变字段如nonce）
	// 2. 添加标准前缀
	// 3. 使用SHA-256计算哈希
	// 4. 验证结果长度
	// 5. 返回哈希值

	if m.logger != nil {
		m.logger.Debugf("计算本地区块哈希，高度: %d", block.Header.Height)
	}

	// 简化的哈希计算（占位实现）
	data := fmt.Sprintf("%s%d-%d", BlockHashPrefix, block.Header.Height, block.Header.Timestamp)
	hash := sha256.Sum256([]byte(data))

	return hash[:], nil
}

// ==================== Merkle树计算 ====================

// computeMerkleRoot 计算交易Merkle根
//
// 🎯 **交易完整性保证**
//
// 构建交易列表的Merkle树并计算根哈希，用于验证交易完整性。
//
// 🔄 **Merkle树构建算法**：
//
//  1. **叶子节点生成**：
//     - 计算每个交易的哈希（通过crypto层服务）
//     - 添加叶子节点前缀并计算叶子哈希
//
//  2. **树结构构建**：
//     - 逐层向上构建，直到只剩一个根节点
//     - 奇数节点时复制最后一个节点进行配对
//     - 使用内部节点前缀区分叶子和内部节点
//
// 3. **特殊情况处理**：
//   - **空交易列表**：返回固定的空根哈希
//   - **单个交易**：返回该交易的叶子哈希
//
// 🛡️ **安全性保证**：
// - **前缀区分**：叶子节点和内部节点使用不同前缀，防止长度扩展攻击
// - **标准化计算**：统一使用crypto层服务计算交易哈希
// - **算法固定**：统一使用SHA-256算法
//
// 参数：
//
//	ctx: 上下文对象
//	transactions: 交易列表
//
// 返回值：
//
//	[]byte: 32字节Merkle根哈希
//	error: 计算过程中的错误
func (m *Manager) computeMerkleRoot(ctx context.Context, transactions []*transaction.Transaction) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debugf("计算Merkle根，交易数: %d", len(transactions))
	}

	// 处理空交易列表
	if len(transactions) == 0 {
		emptyRoot := sha256.Sum256([]byte("EMPTY_MERKLE_ROOT"))
		return emptyRoot[:], nil
	}

	// 处理单个交易的情况
	if len(transactions) == 1 {
		txHash, err := m.computeTransactionHash(ctx, transactions[0])
		if err != nil {
			return nil, fmt.Errorf("计算交易哈希失败: %w", err)
		}

		// 生成叶子节点哈希
		leafData := append([]byte{MerkleLeafPrefix}, txHash...)
		leafHash := sha256.Sum256(leafData)
		return leafHash[:], nil
	}

	// 步骤1: 计算所有交易的叶子节点哈希
	leaves := make([][]byte, len(transactions))
	for i, tx := range transactions {
		// 计算交易哈希
		txHash, err := m.computeTransactionHash(ctx, tx)
		if err != nil {
			return nil, fmt.Errorf("计算第 %d 个交易哈希失败: %w", i, err)
		}

		// 生成叶子节点哈希（添加叶子节点前缀）
		leafData := append([]byte{MerkleLeafPrefix}, txHash...)
		leafHash := sha256.Sum256(leafData)
		leaves[i] = leafHash[:]
	}

	// 步骤2: 逐层构建Merkle树
	currentLevel := leaves
	for len(currentLevel) > 1 {
		nextLevel := make([][]byte, 0, (len(currentLevel)+1)/2)

		// 处理成对的节点
		for i := 0; i < len(currentLevel); i += 2 {
			left := currentLevel[i]
			var right []byte

			// 处理奇数节点情况：复制最后一个节点
			if i+1 < len(currentLevel) {
				right = currentLevel[i+1]
			} else {
				right = currentLevel[i] // 自我配对
			}

			// 计算父节点哈希（添加内部节点前缀）
			parentData := append([]byte{MerkleInternalPrefix}, left...)
			parentData = append(parentData, right...)
			parentHash := sha256.Sum256(parentData)

			nextLevel = append(nextLevel, parentHash[:])
		}

		currentLevel = nextLevel
	}

	if m.logger != nil {
		m.logger.Debugf("成功计算Merkle根，哈希长度: %d", len(currentLevel[0]))
	}

	return currentLevel[0], nil
}

// computeTransactionHash 计算交易哈希
//
// 🎯 **交易标识计算**
//
// 计算单个交易的标准化哈希，用于Merkle树构建和交易索引。
// 调用crypto层的TransactionHashService来确保哈希计算的一致性。
//
// 参数：
//
//	ctx: 上下文对象
//	tx: 交易对象
//
// 返回值：
//
//	[]byte: 32字节交易哈希
//	error: 计算过程中的错误
func (m *Manager) computeTransactionHash(ctx context.Context, tx *transaction.Transaction) ([]byte, error) {
	if tx == nil {
		return nil, fmt.Errorf("交易为空，无法计算哈希")
	}

	if m.logger != nil {
		m.logger.Debugf("计算交易哈希")
	}

	// 构造 ComputeHashRequest
	req := &transaction.ComputeHashRequest{
		Transaction:      tx,
		IncludeDebugInfo: false, // 生产环境通常不需要调试信息
	}

	// 调用 gRPC TransactionHashService 计算哈希
	resp, err := m.txHashServiceClient.ComputeHash(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用交易哈希服务失败: %w", err)
	}

	// 验证响应结果
	if resp == nil {
		return nil, fmt.Errorf("交易哈希服务返回空响应")
	}
	if !resp.IsValid {
		return nil, fmt.Errorf("交易哈希计算失败：交易格式无效")
	}
	if len(resp.Hash) != StandardHashLength {
		return nil, fmt.Errorf("交易哈希长度不正确: 期望 %d 字节, 实际 %d 字节",
			StandardHashLength, len(resp.Hash))
	}

	if m.logger != nil {
		m.logger.Debugf("成功计算交易哈希，长度: %d", len(resp.Hash))
	}

	return resp.Hash, nil
}

// buildMerkleTree 构建完整Merkle树结构
//
// 🎯 **Merkle树数据结构**
//
// 构建完整的Merkle树结构，支持Merkle证明的生成和验证。
//
// 树结构：
// - 叶子节点：交易哈希
// - 内部节点：子节点哈希的组合哈希
// - 根节点：整个树的根哈希
//
// 参数：
//
//	txHashes: 交易哈希列表
//
// 返回值：
//
//	interface{}: Merkle树结构（具体类型待定义）
//	error: 构建过程中的错误
func (m *Manager) buildMerkleTree(txHashes [][]byte) (interface{}, error) {
	// TODO: 实现完整Merkle树构建逻辑
	//
	// 构建步骤：
	// 1. 创建树结构数据结构
	// 2. 从交易哈希创建叶子节点
	// 3. 逐层向上构建内部节点
	// 4. 记录每层的节点位置
	// 5. 返回完整的树结构

	if m.logger != nil {
		m.logger.Debugf("构建Merkle树，哈希数: %d", len(txHashes))
	}

	// 占位实现
	return nil, nil
}

// ==================== POW难度验证 ====================

// verifyProofOfWork 验证工作量证明
//
// 🎯 **POW算法验证**
//
// 验证区块的工作量证明是否满足网络难度要求。
//
// 🔄 **POW验证流程**：
//
// 1. **区块哈希计算**：
//
//   - 使用标准方法计算区块哈希
//
//   - 包含nonce字段的完整区块头
//
//   - 确保哈希计算的准确性
//
//     2. **难度目标转换**：
//     ```
//     target = MaxTarget / difficulty
//     // 或者使用compact bits格式
//     target = expandCompactBits(header.bits)
//     ```
//
//     3. **哈希值比较**：
//     ```
//     blockHashInt = new(big.Int).SetBytes(blockHash)
//     return blockHashInt.Cmp(target) <= 0
//     ```
//
// 4. **验证结果判断**：
//   - 哈希值 ≤ 目标值：POW有效
//   - 哈希值 > 目标值：POW无效
//
// 🎯 **难度计算公式**：
// ```
// target = MaxTarget / difficulty
// valid = SHA256(blockHeader) ≤ target
// ```
//
// 参数：
//
//	header: 区块头（包含nonce）
//	difficulty: 目标难度值
//
// 返回值：
//
//	bool: POW验证结果，true表示有效
//	error: 验证过程中的错误
func (m *Manager) verifyProofOfWork(header *core.BlockHeader, difficulty uint64) (bool, error) {
	// TODO: 实现POW验证逻辑
	//
	// 验证步骤：
	// 1. 计算包含nonce的区块哈希
	// 2. 根据难度计算目标值
	// 3. 将哈希转换为大整数
	// 4. 比较哈希与目标值
	// 5. 返回验证结果

	if m.logger != nil {
		m.logger.Debugf("验证POW，难度: %d", difficulty)
	}

	// TODO: 实现具体验证逻辑
	// 1. 创建包含nonce的完整区块（用于哈希计算）
	// 2. 计算区块哈希
	// 3. 计算难度目标
	// 4. 比较哈希与目标

	// 占位实现 - 总是返回true
	return true, nil
}

// calculateDifficultyTarget 计算难度目标值
//
// 🎯 **难度目标计算**
//
// 根据难度值计算对应的目标哈希值。
//
// 计算公式：
// ```
// target = MaxTarget / difficulty
// 其中 MaxTarget = 2^(256-32) = 2^224
// ```
//
// 参数：
//
//	difficulty: 难度值
//
// 返回值：
//
//	*big.Int: 目标值
//	error: 计算错误
func (m *Manager) calculateDifficultyTarget(difficulty uint64) (*big.Int, error) {
	if difficulty == 0 {
		return nil, fmt.Errorf("难度值不能为零")
	}

	// 计算目标值：MaxTarget / difficulty
	target := new(big.Int).Div(MaxTarget, big.NewInt(int64(difficulty)))

	// 确保目标值在合理范围内
	if target.Cmp(MinTarget) < 0 {
		target = new(big.Int).Set(MinTarget)
	}

	if m.logger != nil {
		m.logger.Debugf("计算难度目标，难度: %d, 目标: %x", difficulty, target.Bytes())
	}

	return target, nil
}

// ==================== 哈希格式转换工具 ====================

// hashToHexString 哈希转十六进制字符串
//
// 🎯 **哈希格式转换**
//
// 将字节数组哈希转换为十六进制字符串表示。
//
// 参数：
//
//	hash: 哈希字节数组
//
// 返回值：
//
//	string: 十六进制字符串
func (m *Manager) hashToHexString(hash []byte) string {
	return fmt.Sprintf("%x", hash)
}

// hexStringToHash 十六进制字符串转哈希
//
// 🎯 **哈希格式解析**
//
// 将十六进制字符串解析为哈希字节数组。
//
// 参数：
//
//	hexStr: 十六进制字符串
//
// 返回值：
//
//	[]byte: 哈希字节数组
//	error: 解析错误
func (m *Manager) hexStringToHash(hexStr string) ([]byte, error) {
	// TODO: 实现十六进制字符串解析
	//
	// 解析步骤：
	// 1. 验证字符串格式
	// 2. 去除可能的前缀（如0x）
	// 3. 转换为字节数组
	// 4. 验证长度
	// 5. 返回结果

	if len(hexStr) != StandardHashLength*2 {
		return nil, fmt.Errorf("无效的哈希字符串长度: %d", len(hexStr))
	}

	// 占位实现
	return make([]byte, StandardHashLength), nil
}

// validateHashLength 验证哈希长度
//
// 🎯 **哈希格式验证**
//
// 验证哈希字节数组的长度是否符合标准。
//
// 参数：
//
//	hash: 哈希字节数组
//
// 返回值：
//
//	error: 验证错误，nil表示长度正确
func (m *Manager) validateHashLength(hash []byte) error {
	if len(hash) != StandardHashLength {
		return fmt.Errorf("哈希长度不正确: 期望 %d 字节, 实际 %d 字节",
			StandardHashLength, len(hash))
	}
	return nil
}

// ==================== 性能优化工具 ====================

// batchComputeTransactionHashes 批量计算交易哈希
//
// 🎯 **批量哈希计算优化**
//
// 批量计算多个交易的哈希，调用crypto层服务确保一致性。
// 支持大量交易的高效处理。
//
// 参数：
//
//	ctx: 上下文对象
//	transactions: 交易列表
//
// 返回值：
//
//	[][]byte: 交易哈希列表
//	error: 计算错误
func (m *Manager) batchComputeTransactionHashes(ctx context.Context, transactions []*transaction.Transaction) ([][]byte, error) {
	if len(transactions) == 0 {
		return [][]byte{}, nil
	}

	hashes := make([][]byte, len(transactions))

	for i, tx := range transactions {
		hash, err := m.computeTransactionHash(ctx, tx)
		if err != nil {
			return nil, fmt.Errorf("计算第 %d 个交易哈希失败: %w", i, err)
		}
		hashes[i] = hash
	}

	if m.logger != nil {
		m.logger.Debugf("批量计算交易哈希完成，数量: %d", len(hashes))
	}

	return hashes, nil
}

// ==================== 哈希缓存工具 ====================

// cacheBlockHash 缓存区块哈希
//
// 🎯 **哈希计算缓存**
//
// 缓存计算过的区块哈希，避免重复计算。
//
// 参数：
//
//	ctx: 上下文
//	blockHeight: 区块高度
//	hash: 计算的哈希
//
// 返回值：
//
//	error: 缓存错误
func (m *Manager) cacheBlockHash(ctx context.Context, blockHeight uint64, hash []byte) error {
	// TODO: 实现区块哈希缓存逻辑
	//
	// 缓存策略：
	// 1. 使用区块高度作为键
	// 2. 设置适当的TTL
	// 3. 压缩存储以节省空间
	// 4. 支持批量缓存清理

	if m.logger != nil {
		m.logger.Debugf("缓存区块哈希，高度: %d, 哈希: %x", blockHeight, hash)
	}

	// 占位实现
	return nil
}

// getCachedBlockHash 获取缓存的区块哈希
//
// 🎯 **哈希缓存查询**
//
// 从缓存中获取之前计算的区块哈希。
//
// 参数：
//
//	ctx: 上下文
//	blockHeight: 区块高度
//
// 返回值：
//
//	[]byte: 缓存的哈希，nil表示未找到
//	bool: 是否找到缓存
//	error: 查询错误
func (m *Manager) getCachedBlockHash(ctx context.Context, blockHeight uint64) ([]byte, bool, error) {
	// TODO: 实现哈希缓存查询逻辑
	//
	// 查询步骤：
	// 1. 根据区块高度查询缓存
	// 2. 验证缓存数据的有效性
	// 3. 检查缓存是否过期
	// 4. 返回查询结果

	if m.logger != nil {
		m.logger.Debugf("查询缓存区块哈希，高度: %d", blockHeight)
	}

	// 占位实现
	return nil, false, nil
}
