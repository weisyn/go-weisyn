// mine_block_header.go 实现区块头挖矿和PoW验证的委托逻辑
//
// 🎯 **PoW引擎委托实现**
//
// 本文件实现：
// - MineBlockHeader：委托给注入的POWEngine进行挖矿计算
// - VerifyBlockHeader：委托给注入的POWEngine进行PoW验证
// - 遵循公共接口约束，不直接使用crypto包
//
// 🏗️ **架构合规性**：
// - 使用注入的POWEngine处理所有哈希计算
// - 避免直接调用crypto/sha256
// - 委托模式确保加密逻辑的统一性
//
// 🚫 **移除的旧实现**：
// - 移除了所有直接使用crypto/sha256的方法
// - 移除了自定义的序列化、哈希计算、nonce搜索等逻辑
// - 移除了多线程并行计算的复杂实现（现由POWEngine处理）
package pow_handler

import (
	"context"
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// mineBlockHeader 委托给POWEngine进行区块头挖矿计算
func (s *PoWComputeService) mineBlockHeader(ctx context.Context, header *core.BlockHeader) (*core.BlockHeader, error) {
	s.logger.Info("开始PoW挖矿计算")

	// 参数校验
	if header == nil {
		return nil, fmt.Errorf("区块头不能为空")
	}

	if s.powEngine == nil {
		return nil, fmt.Errorf("POW引擎未注入")
	}

	// 委托给注入的POWEngine进行挖矿计算
	// POWEngine内部处理所有哈希计算、nonce搜索、并行计算等逻辑
	minedHeader, err := s.powEngine.MineBlockHeader(ctx, header)
	if err != nil {
		s.logger.Errorf("POW引擎挖矿失败: %v", err)
		return nil, fmt.Errorf("POW引擎挖矿失败: %v", err)
	}

	s.logger.Info("PoW挖矿计算完成")
	return minedHeader, nil
}

// verifyBlockHeader 委托给POWEngine进行区块头PoW验证
func (s *PoWComputeService) verifyBlockHeader(header *core.BlockHeader) (bool, error) {
	s.logger.Info("验证区块头PoW")

	// 参数校验
	if header == nil {
		return false, fmt.Errorf("区块头不能为空")
	}

	if s.powEngine == nil {
		return false, fmt.Errorf("POW引擎未注入")
	}

	// 委托给注入的POWEngine进行PoW验证
	// POWEngine内部处理所有哈希计算和难度验证逻辑
	isValid, err := s.powEngine.VerifyBlockHeader(header)
	if err != nil {
		s.logger.Errorf("POW引擎验证失败: %v", err)
		return false, fmt.Errorf("POW引擎验证失败: %v", err)
	}

	if isValid {
		s.logger.Info("PoW验证成功")
	} else {
		s.logger.Info("PoW验证失败")
	}

	return isValid, nil
}

// ==================== 架构重构说明 ====================
//
// 🚫 **已移除的旧版本复杂实现**：
//
// 以下方法已被移除，因为它们直接使用了crypto/sha256，违反了架构约束：
//
// 1. **哈希计算相关**：
//    - calculateBlockHeaderHash(): 直接使用crypto/sha256
//    - serializeBlockHeader(): 自定义序列化逻辑
//    - batchHashCompute(): 批量哈希计算优化
//
// 2. **难度管理相关**：
//    - calculateTarget(): 难度目标计算
//    - verifyProofOfWork(): 自定义PoW验证
//
// 3. **并行计算相关**：
//    - createMiningTasks(): 任务分配和nonce空间分割
//    - executePoWTask(): 工作器并行计算逻辑
//    - waitForMiningResult(): 结果等待和收集
//    - updateHashingStatistics(): 性能统计更新
//
// 4. **数据结构相关**：
//    - PoWTask, PoWResult, PoWWorker: 并行计算数据结构
//    - PerformanceMonitor, HashPool: 性能优化组件
//
// ✅ **新的架构模式**：
//
// 1. **委托模式**：所有PoW计算委托给注入的POWEngine
// 2. **接口统一**：遵循pkg/interfaces/infrastructure/crypto/pow.go定义
// 3. **职责分离**：PoWComputeService只负责业务编排，不处理底层计算
// 4. **依赖注入**：通过构造函数注入POWEngine，确保可测试性
//
// 🎯 **收益**：
// - 消除直接crypto依赖，符合架构约束
// - 降低代码复杂度，提高可维护性
// - 统一哈希计算逻辑，避免重复实现
// - 支持POW算法的热插拔和升级
// - 便于单元测试和mock
//
// ==================== 文件结束 ====================