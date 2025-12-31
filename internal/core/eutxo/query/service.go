// Package query 提供 UTXO 查询服务的简化实现
//
// ⚠️ **重要说明**：
// - 此实现仅供 EUTXO 模块内部使用
// - 后续 Query 模块实施时，会迁移到 pkg/interfaces/query
//
// 🎯 **设计目的**：
// - 满足 UTXOSnapshot 的查询需求
// - 避免依赖冲突
// - 提供简单实现
package query

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/weisyn/v1/internal/core/eutxo/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"
	"google.golang.org/protobuf/proto"
)

// Service UTXO 查询服务（简化实现）
//
// 🎯 **核心职责**：
// - 提供 UTXO 查询功能
// - 供 UTXOSnapshot 使用
//
// 💡 **实现方式**：
// - 直接从 Storage 查询
// - 简化的查询逻辑
type Service struct {
	storage storage.BadgerStore
	logger  log.Logger
}

// NewService 创建 UTXO 查询服务
func NewService(storage storage.BadgerStore, logger log.Logger) (interfaces.InternalUTXOQuery, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage 不能为空")
	}

	s := &Service{
		storage: storage,
		logger:  logger,
	}

	if logger != nil {
		logger.Info("✅ UTXOQuery 服务已创建（简化版）")
	}

	return s, nil
}

// GetUTXO 获取单个 UTXO
//
// 实现 interfaces.InternalUTXOQuery.GetUTXO
func (s *Service) GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxo.UTXO, error) {
	// 1. 验证参数
	if outpoint == nil || outpoint.TxId == nil {
		return nil, fmt.Errorf("无效的 OutPoint")
	}

	// 2. 构造存储键
	utxoKey := buildUTXOKey(outpoint)

	// 3. 从 Storage 获取
	data, err := s.storage.Get(ctx, []byte(utxoKey))
	if err != nil {
		return nil, fmt.Errorf("查询 UTXO 失败: %w", err)
	}

	// 4. 检查数据是否存在
	if data == nil || len(data) == 0 {
		return nil, fmt.Errorf("UTXO 不存在")
	}

	// 5. 反序列化
	utxoObj := &utxo.UTXO{}
	if err := proto.Unmarshal(data, utxoObj); err != nil {
		return nil, fmt.Errorf("反序列化 UTXO 失败: %w", err)
	}

	return utxoObj, nil
}

// GetUTXOsByAddress 按地址查询 UTXO 列表（P0 修复：使用地址索引）
//
// 实现 eutxo.UTXOQuery.GetUTXOsByAddress
//
// 🎯 **查询策略**：
// 1. 使用地址索引键 `index:address:{address}` 查询索引
// 2. 解析索引值获取所有 outpoint
// 3. 根据每个 outpoint 查询对应的 UTXO
// 4. 根据 category 过滤结果
// 5. 返回 UTXO 列表
//
// ⚠️ **includeSpent 参数说明**：
// 当前实现中，地址索引只维护未消费的 UTXO（已消费的 UTXO 会从索引中移除）。
// 因此：
// - includeSpent=false: 返回未消费的 UTXO（当前实现支持）
// - includeSpent=true: 需要维护已消费 UTXO 的历史状态（当前实现不支持）
//
// 如果未来需要支持 includeSpent=true，需要：
// 1. 在删除 UTXO 时保留到 spent 索引（如 `index:address:spent:{address}`）
// 2. 查询时合并未消费和已消费的 UTXO 列表
func (s *Service) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, includeSpent bool) ([]*utxo.UTXO, error) {
	// 1. 验证参数
	if len(address) == 0 {
		return nil, fmt.Errorf("地址不能为空")
	}
	
	// 2. 检查 includeSpent 参数（当前不支持 true）
	if includeSpent {
		if s.logger != nil {
			s.logger.Warnf("⚠️ includeSpent=true 当前不支持，将只返回未消费的 UTXO")
		}
	}

	// 3. 构建地址索引键
	// 格式：index:address:{address}
	addressIndexKey := fmt.Sprintf("index:address:%x", address)

	// 4. 从 Storage 获取索引数据
	indexData, err := s.storage.Get(ctx, []byte(addressIndexKey))
	if err != nil {
		// 索引不存在，返回空列表（不是错误）
		if s.logger != nil {
			s.logger.Debugf("地址 %x 的索引不存在，返回空列表", address)
		}
		return []*utxo.UTXO{}, nil
	}

	if len(indexData) == 0 {
		if s.logger != nil {
			s.logger.Debugf("地址 %x 的索引为空，返回空列表", address)
		}
		return []*utxo.UTXO{}, nil
	}

	// 5. 解析索引数据，获取所有 outpoint
	outpoints, err := s.decodeOutPointList(indexData)
	if err != nil {
		return nil, fmt.Errorf("解析地址索引数据失败: %w", err)
	}

	if len(outpoints) == 0 {
		if s.logger != nil {
			s.logger.Debugf("地址 %x 的索引中没有 outpoint，返回空列表", address)
		}
		return []*utxo.UTXO{}, nil
	}

	// 6. 根据每个 outpoint 查询对应的 UTXO
	utxos := make([]*utxo.UTXO, 0, len(outpoints))
	for _, outpoint := range outpoints {
		utxoObj, err := s.GetUTXO(ctx, outpoint)
		if err != nil {
			// 如果某个 UTXO 查询失败，记录警告但继续处理其他 UTXO
			if s.logger != nil {
				s.logger.Warnf("查询 UTXO 失败 (txHash=%x, index=%d): %v", outpoint.TxId, outpoint.OutputIndex, err)
			}
			continue
		}
		if utxoObj == nil {
			continue
		}

		// 7. 过滤：按 category（如果指定）
		if category != nil {
			output := utxoObj.GetCachedOutput()
			if output == nil {
				continue
			}
			// 根据输出类型判断 category
			var utxoCategory utxo.UTXOCategory
			if output.GetAsset() != nil {
				utxoCategory = utxo.UTXOCategory_UTXO_CATEGORY_ASSET
			} else if output.GetResource() != nil {
				utxoCategory = utxo.UTXOCategory_UTXO_CATEGORY_RESOURCE
			} else if output.GetState() != nil {
				utxoCategory = utxo.UTXOCategory_UTXO_CATEGORY_STATE
			}
			if utxoCategory != *category {
				continue
			}
		}

		// 注：includeSpent 参数在当前实现中不需要额外处理
		// 因为地址索引只维护未消费的 UTXO（已消费的会从索引中移除）
		
		utxos = append(utxos, utxoObj)
	}

	if s.logger != nil {
		s.logger.Debugf("📋 按地址查询 UTXO: address=%x, count=%d", address, len(utxos))
	}

	return utxos, nil
}

// ListUTXOs 列出指定高度的所有 UTXO（P3-11：使用高度索引查询）
//
// 实现 interfaces.InternalUTXOQuery.ListUTXOs
//
// 🎯 **查询策略**：
// 1. 使用高度索引键 `index:height:{height}` 查询索引
// 2. 解析索引值获取所有 outpoint
// 3. 根据每个 outpoint 查询对应的 UTXO
// 4. 返回 UTXO 列表
func (s *Service) ListUTXOs(ctx context.Context, height uint64) ([]*utxo.UTXO, error) {
	// 1. 构建高度索引键
	// 格式：index:height:{height}
	heightIndexKey := fmt.Sprintf("index:height:%d", height)

	// 2. 从 Storage 获取索引数据
	indexData, err := s.storage.Get(ctx, []byte(heightIndexKey))
	if err != nil {
		// 索引不存在，返回空列表（不是错误）
		if s.logger != nil {
			s.logger.Debugf("高度 %d 的索引不存在，返回空列表", height)
		}
		return []*utxo.UTXO{}, nil
	}

	if len(indexData) == 0 {
		if s.logger != nil {
			s.logger.Debugf("高度 %d 的索引为空，返回空列表", height)
		}
		return []*utxo.UTXO{}, nil
	}

	// 3. 解析索引数据，获取所有 outpoint
	outpoints, err := s.decodeOutPointList(indexData)
	if err != nil {
		return nil, fmt.Errorf("解析高度索引数据失败: %w", err)
	}

	if len(outpoints) == 0 {
		if s.logger != nil {
			s.logger.Debugf("高度 %d 的索引中没有 outpoint，返回空列表", height)
		}
		return []*utxo.UTXO{}, nil
	}

	// 4. 根据每个 outpoint 查询对应的 UTXO
	utxos := make([]*utxo.UTXO, 0, len(outpoints))
	for _, outpoint := range outpoints {
		utxoObj, err := s.GetUTXO(ctx, outpoint)
		if err != nil {
			// 如果某个 UTXO 查询失败，记录警告但继续处理其他 UTXO
			if s.logger != nil {
				s.logger.Warnf("查询 UTXO 失败 (txHash=%x, index=%d): %v", outpoint.TxId, outpoint.OutputIndex, err)
			}
			continue
		}
		if utxoObj != nil {
			utxos = append(utxos, utxoObj)
		}
	}

	if s.logger != nil {
		s.logger.Debugf("📋 查询 UTXO 列表: height=%d, count=%d", height, len(utxos))
	}

	return utxos, nil
}

// GetReferenceCount 获取 UTXO 的引用计数
//
// 实现 interfaces.InternalUTXOQuery.GetReferenceCount
func (s *Service) GetReferenceCount(ctx context.Context, outpoint *transaction.OutPoint) (uint64, error) {
	// 1. 验证参数
	if outpoint == nil || outpoint.TxId == nil {
		return 0, fmt.Errorf("无效的 OutPoint")
	}

	// 2. 构造引用计数键
	refKey := buildReferenceKey(outpoint)

	// 3. 从 Storage 获取
	data, err := s.storage.Get(ctx, []byte(refKey))
	if err != nil {
		// 如果不存在，返回 0
		return 0, nil
	}

	// 4. 如果数据为空或 nil，返回 0
	if data == nil || len(data) == 0 {
		return 0, nil
	}

	// 5. 解析引用计数（使用 BigEndian）
	if len(data) != 8 {
		return 0, fmt.Errorf("引用计数数据长度错误: 期望8字节，实际%d字节", len(data))
	}

	refCount := uint64(data[0])<<56 | uint64(data[1])<<48 | uint64(data[2])<<40 | uint64(data[3])<<32 |
		uint64(data[4])<<24 | uint64(data[5])<<16 | uint64(data[6])<<8 | uint64(data[7])

	return refCount, nil
}

// decodeOutPointList 解码索引数据中的 outpoint 列表
//
// 🔧 索引数据格式：多个固定36字节的 outpoint 序列
// 每个 outpoint: [32字节TxId][4字节OutputIndex] = 36字节
// （与 persistence/writer/utxo.go 的 addToAddressIndexInTransaction 保持一致）
//
// 参数：
//   - data: 索引数据
//
// 返回：
//   - []*transaction.OutPoint: outpoint 列表
//   - error: 解码错误
func (s *Service) decodeOutPointList(data []byte) ([]*transaction.OutPoint, error) {
	// 验证数据长度必须是36的倍数
	if len(data)%36 != 0 {
		return nil, fmt.Errorf("索引数据格式错误：长度(%d)不是36的倍数", len(data))
	}

	count := len(data) / 36
	if count == 0 {
		return []*transaction.OutPoint{}, nil
	}

	outpoints := make([]*transaction.OutPoint, 0, count)

	for i := 0; i < count; i++ {
		offset := i * 36
		
		// 读取 TxId（32字节）
		txID := make([]byte, 32)
		copy(txID, data[offset:offset+32])

		// 读取 OutputIndex（4字节，BigEndian）
		outputIndex := binary.BigEndian.Uint32(data[offset+32 : offset+36])

		// 创建 OutPoint
		outpoint := &transaction.OutPoint{
			TxId:        txID,
			OutputIndex: outputIndex,
		}

		outpoints = append(outpoints, outpoint)
	}

	return outpoints, nil
}

// buildUTXOKey 构造 UTXO 存储键
//
// 格式：utxo:set:{txHash}:{outputIndex}
// 符合 docs/system/designs/storage/data-architecture.md 规范
func buildUTXOKey(outpoint *transaction.OutPoint) string {
	return fmt.Sprintf("utxo:set:%x:%d", outpoint.TxId, outpoint.OutputIndex)
}

// buildReferenceKey 构造引用计数存储键
//
// 格式：ref:<txhash>:<index>
func buildReferenceKey(outpoint *transaction.OutPoint) string {
	return fmt.Sprintf("ref:%x:%d", outpoint.TxId, outpoint.OutputIndex)
}

// parseUTXOKey 解析 UTXO 存储键（P3-12：完整实现）
//
// 用于从键中提取信息
// 支持格式：utxo:set:{txHash}:{outputIndex}
//
// 参数：
//   - key: UTXO 存储键
//
// 返回：
//   - txHash: 交易哈希（32字节）
//   - index: 输出索引
//   - err: 解析错误
func parseUTXOKey(key string) (txHash []byte, index uint32, err error) {
	parts := strings.Split(key, ":")
	// 格式：utxo:set:{txHash}:{outputIndex} -> 4 parts
	if len(parts) != 4 || parts[0] != "utxo" || parts[1] != "set" {
		return nil, 0, fmt.Errorf("无效的 UTXO 键格式: %s", key)
	}

	// 解析 txHash (hex string)
	txHashHex := parts[2]
	txHashBytes, err := hex.DecodeString(txHashHex)
	if err != nil {
		return nil, 0, fmt.Errorf("解析交易哈希失败: %w", err)
	}
	if len(txHashBytes) != 32 {
		return nil, 0, fmt.Errorf("交易哈希长度错误: 期望32字节, 实际%d字节", len(txHashBytes))
	}

	// 解析 outputIndex (decimal string)
	var indexVal uint64
	_, err = fmt.Sscanf(parts[3], "%d", &indexVal)
	if err != nil {
		return nil, 0, fmt.Errorf("解析输出索引失败: %w", err)
	}
	if indexVal > uint64(^uint32(0)) {
		return nil, 0, fmt.Errorf("输出索引超出范围: %d", indexVal)
	}

	return txHashBytes, uint32(indexVal), nil
}

// 编译时检查接口实现
var _ interfaces.InternalUTXOQuery = (*Service)(nil)

