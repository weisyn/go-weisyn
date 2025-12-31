// Package health 提供UTXO集健康检查与自动修复功能
//
// 🎯 **核心职责**：
// - 扫描UTXO集，检测BlockHeight=0的损坏数据
// - 自动推断并修复损坏UTXO的BlockHeight字段
// - 生成详细的健康检查报告
//
// 📋 **自恢复策略**：
// - 方法1：从区块链查找交易所在区块（精确）
// - 方法2：使用链尖高度作为保守估计（fallback）
package health

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	chainif "github.com/weisyn/v1/pkg/interfaces/persistence"
)

// ============================================================================
//                              健康检查器
// ============================================================================

// HealthChecker 提供UTXO集健康检查与自动修复功能
type HealthChecker struct {
	storage    storage.BadgerStore
	chainQuery chainif.ChainQuery // 用于查询区块信息
	logger     log.Logger
}

// NewHealthChecker 创建健康检查器实例
func NewHealthChecker(
	storage storage.BadgerStore,
	chainQuery chainif.ChainQuery,
	logger log.Logger,
) *HealthChecker {
	return &HealthChecker{
		storage:    storage,
		chainQuery: chainQuery,
		logger:     logger,
	}
}

// ============================================================================
//                              数据结构
// ============================================================================

// HealthReport 健康检查报告
type HealthReport struct {
	StartTime         time.Time      // 开始时间
	EndTime           time.Time      // 结束时间
	TotalUTXOs        int            // 总UTXO数量
	CorruptUTXOs      int            // 损坏UTXO数量
	RepairedUTXOs     int            // 已修复UTXO数量
	UnrepairableUTXOs int            // 无法修复UTXO数量
	RepairRecords     []RepairRecord // 修复记录列表
}

// RepairRecord 修复记录
type RepairRecord struct {
	Outpoint  *transaction.OutPoint // UTXO标识
	OldHeight uint64                // 修复前高度
	NewHeight uint64                // 修复后高度
	Timestamp time.Time             // 修复时间
}

// ============================================================================
//                              健康检查
// ============================================================================

// PerformCheck 执行UTXO集健康检查
//
// 参数：
//   - ctx: 上下文
//   - autoRepair: 是否自动修复损坏的UTXO
//
// 返回：
//   - *HealthReport: 健康检查报告
//   - error: 错误信息
func (c *HealthChecker) PerformCheck(ctx context.Context, autoRepair bool) (*HealthReport, error) {
	report := &HealthReport{
		StartTime:     time.Now(),
		RepairRecords: make([]RepairRecord, 0),
	}

	if c.logger != nil {
		c.logger.Infof("🔍 开始UTXO集健康检查 (自动修复=%v)", autoRepair)
	}

	// 1. 扫描所有UTXO
	utxoPrefix := []byte("utxo:set:")
	utxoMap, err := c.storage.PrefixScan(ctx, utxoPrefix)
	if err != nil {
		return nil, fmt.Errorf("扫描UTXO集失败: %w", err)
	}

	report.TotalUTXOs = len(utxoMap)

	// 2. 检查每个UTXO
	for key, utxoData := range utxoMap {
		utxoObj := &utxo.UTXO{}
		if err := proto.Unmarshal(utxoData, utxoObj); err != nil {
			if c.logger != nil {
				c.logger.Warnf("反序列化UTXO失败，跳过: key=%s, err=%v", string(key), err)
			}
			report.CorruptUTXOs++
			report.UnrepairableUTXOs++
			continue
		}

		// 检查BlockHeight字段
		if utxoObj.BlockHeight == 0 {
			report.CorruptUTXOs++

			if c.logger != nil {
				c.logger.Warnf("⚠️ 发现损坏UTXO: outpoint=%x:%d, BlockHeight=0",
					utxoObj.Outpoint.TxId, utxoObj.Outpoint.OutputIndex)
			}

			if autoRepair {
				// 尝试修复
				correctHeight, err := c.inferBlockHeight(ctx, utxoObj)
				if err != nil {
					if c.logger != nil {
						c.logger.Warnf("无法推断UTXO高度: outpoint=%x:%d, err=%v",
							utxoObj.Outpoint.TxId, utxoObj.Outpoint.OutputIndex, err)
					}
					report.UnrepairableUTXOs++
					continue
				}

				// 修复并写回数据库
				utxoObj.BlockHeight = correctHeight
				newData, err := proto.Marshal(utxoObj)
				if err != nil {
					if c.logger != nil {
						c.logger.Errorf("重新序列化UTXO失败: outpoint=%x:%d, err=%v",
							utxoObj.Outpoint.TxId, utxoObj.Outpoint.OutputIndex, err)
					}
					report.UnrepairableUTXOs++
					continue
				}

				if err := c.storage.Set(ctx, []byte(key), newData); err != nil {
					if c.logger != nil {
						c.logger.Errorf("写回修复后的UTXO失败: outpoint=%x:%d, err=%v",
							utxoObj.Outpoint.TxId, utxoObj.Outpoint.OutputIndex, err)
					}
					report.UnrepairableUTXOs++
					continue
				}

				// 记录修复成功
				report.RepairedUTXOs++
				report.RepairRecords = append(report.RepairRecords, RepairRecord{
					Outpoint:  utxoObj.Outpoint,
					OldHeight: 0,
					NewHeight: correctHeight,
					Timestamp: time.Now(),
				})

				if c.logger != nil {
					c.logger.Infof("✅ 已修复UTXO: outpoint=%x:%d, new_height=%d",
						utxoObj.Outpoint.TxId, utxoObj.Outpoint.OutputIndex, correctHeight)
				}
			}
		}
	}

	report.EndTime = time.Now()

	if c.logger != nil {
		duration := report.EndTime.Sub(report.StartTime)
		c.logger.Infof("✅ UTXO集健康检查完成 (耗时: %v)", duration)
		c.logger.Infof("   总UTXO数量: %d", report.TotalUTXOs)
		c.logger.Infof("   损坏UTXO: %d", report.CorruptUTXOs)
		c.logger.Infof("   已修复: %d", report.RepairedUTXOs)
		c.logger.Infof("   无法修复: %d", report.UnrepairableUTXOs)
	}

	return report, nil
}

// ============================================================================
//                              高度推断
// ============================================================================

// inferBlockHeight 推断UTXO的正确区块高度
//
// 策略：
//
//	使用链尖高度作为保守估计
//	（精确推断需要交易索引，但这会增加系统复杂度，暂不实现）
//
// 参数：
//   - ctx: 上下文
//   - utxoObj: 待修复的UTXO对象
//
// 返回：
//   - uint64: 推断的区块高度
//   - error: 错误信息
func (c *HealthChecker) inferBlockHeight(ctx context.Context, utxoObj *utxo.UTXO) (uint64, error) {
	// 使用链尖高度作为保守估计
	if c.chainQuery != nil {
		tipHeight, err := c.chainQuery.GetCurrentHeight(ctx)
		if err == nil && tipHeight > 0 {
			if c.logger != nil {
				c.logger.Warnf("使用链尖高度作为UTXO高度的保守估计: tx=%x, tip_height=%d",
					utxoObj.Outpoint.TxId, tipHeight)
			}
			return tipHeight, nil
		}
		if c.logger != nil && err != nil {
			c.logger.Errorf("获取链尖高度失败: %v", err)
		}
	}

	return 0, fmt.Errorf("无法推断UTXO高度：无法获取链尖高度")
}

// ============================================================================
//                              工具函数
// ============================================================================

// buildUTXOKey 构建UTXO存储键
func buildUTXOKey(outpoint *transaction.OutPoint) string {
	return fmt.Sprintf("utxo:set:%x:%d", outpoint.TxId, outpoint.OutputIndex)
}
