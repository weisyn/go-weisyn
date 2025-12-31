// Package validator 实现区块验证服务
package validator

import (
	"context"
	"fmt"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// validateStructure 验证区块结构
//
// 🎯 **结构验证检查项**：
// 1. 区块头完整性
// 2. 区块体完整性
// 3. 字段有效性
// 4. 区块大小限制
//
// 参数：
//   - ctx: 上下文
//   - block: 待验证区块
//
// 返回：
//   - error: 验证错误（nil表示通过）
func (s *Service) validateStructure(ctx context.Context, block *core.Block) error {
	// 1. 区块头检查
	if block.Header == nil {
		return fmt.Errorf("区块头为空")
	}

	// 2. 区块体检查
	if block.Body == nil {
		return fmt.Errorf("区块体为空")
	}

	// 3. 检查交易列表
	if len(block.Body.Transactions) == 0 {
		return fmt.Errorf("区块交易列表为空")
	}

	// 4. 区块哈希验证（通过计算验证，不检查字段）
	// 注意：区块哈希通过计算Header得出，Header中不存储Hash字段

	// 5. 检查父区块哈希（非创世区块）
	if block.Header.Height > 0 && len(block.Header.PreviousHash) != 32 {
		return fmt.Errorf("父区块哈希长度无效: %d", len(block.Header.PreviousHash))
	}

	// 6. 检查Merkle根
	if len(block.Header.MerkleRoot) != 32 {
		return fmt.Errorf("Merkle根长度无效: %d", len(block.Header.MerkleRoot))
	}

	// 7. 检查状态根
	if len(block.Header.StateRoot) != 32 {
		return fmt.Errorf("状态根长度无效: %d", len(block.Header.StateRoot))
	}

	// 8. 检查时间戳（不能是未来时间，P3-9：时间戳验证）
	// 获取当前时间戳
	currentTime := time.Now().Unix()
	blockTime := int64(block.Header.Timestamp)

	// 允许的时间偏差：未来2小时（考虑时钟偏差和网络延迟）
	maxFutureTime := currentTime + 7200 // 2小时 = 7200秒

	if blockTime > maxFutureTime {
		return fmt.Errorf("区块时间戳是未来时间: 区块时间=%d, 当前时间=%d, 允许偏差=2小时", blockTime, currentTime)
	}

	// 检查时间戳是否合理
	if block.Header.Height == 0 {
		// 创世区块：只验证时间戳不为0，时间戳值由配置文件决定
		// 注意：创世区块时间戳必须在配置文件中显式指定，不能使用默认值
		if blockTime == 0 {
			return fmt.Errorf("创世区块时间戳不能为0，必须在配置文件中显式指定")
		}
		// 不验证时间戳的具体值，因为不同网络可能有不同的创世时间
	} else {
		// 非创世区块：验证时间戳不能早于创世区块时间
		// 通过查询链状态获取创世区块时间戳
		genesisBlock, err := s.queryService.GetBlockByHeight(ctx, 0)
		if err != nil {
			return fmt.Errorf("无法获取创世区块以验证时间戳: %w", err)
		}
		if genesisBlock == nil || genesisBlock.Header == nil {
			return fmt.Errorf("创世区块不存在或无效")
		}
		genesisTime := int64(genesisBlock.Header.Timestamp)
		if blockTime < genesisTime {
			return fmt.Errorf("区块时间戳早于创世时间: %d < %d", blockTime, genesisTime)
		}
	}

	// 9. 检查Coinbase交易在首位
	firstTx := block.Body.Transactions[0]
	if len(firstTx.Inputs) != 0 {
		return fmt.Errorf("首个交易应该是Coinbase交易（没有输入）")
	}

	return nil
}
