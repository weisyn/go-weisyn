// Package eutxo 实现EUTXO查询服务
package eutxo

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"sort"

	runtimectx "github.com/weisyn/v1/internal/core/infrastructure/runtime"
	"github.com/weisyn/v1/internal/core/block/merkle"
	"github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/constants"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"google.golang.org/protobuf/proto"
)

// Service EUTXO查询服务
type Service struct {
	storage storage.BadgerStore
	hasher  crypto.HashManager // ✅ 状态根计算：添加 HashManager 依赖
	logger  log.Logger
}

// NewService 创建EUTXO查询服务
func NewService(storage storage.BadgerStore, hasher crypto.HashManager, logger log.Logger) (interfaces.InternalUTXOQuery, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage 不能为空")
	}
	if hasher == nil {
		return nil, fmt.Errorf("hasher 不能为空（状态根计算需要）")
	}

	s := &Service{
		storage: storage,
		hasher:  hasher,
		logger:  logger,
	}

	if logger != nil {
		logger.Info("✅ UTXOQuery 服务已创建")
	}

	return s, nil
}

// GetUTXO 根据OutPoint精确获取UTXO
func (s *Service) GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxo.UTXO, error) {
	// 1. 验证参数
	if outpoint == nil || outpoint.TxId == nil {
		return nil, fmt.Errorf("无效的 OutPoint")
	}

	// 2. 构造存储键（遵循 data-architecture.md 规范）
	// 键格式：utxo:set:{txHash}:{outputIndex}
	// 符合 docs/system/designs/storage/data-architecture.md 规范
	utxoKey := fmt.Sprintf("utxo:set:%x:%d", outpoint.TxId, outpoint.OutputIndex)

	// 3. 从存储获取
	data, err := s.storage.Get(ctx, []byte(utxoKey))
	if err != nil {
		return nil, fmt.Errorf("查询 UTXO 失败: %w", err)
	}

	// 4. 反序列化
	utxoObj := &utxo.UTXO{}
	if err := proto.Unmarshal(data, utxoObj); err != nil {
		return nil, fmt.Errorf("反序列化 UTXO 失败: %w", err)
	}

	return utxoObj, nil
}

// GetUTXOsByAddress 获取地址拥有的UTXO列表（P3-16：基于索引的地址UTXO查询）
//
// 🎯 **查询策略**：
// 1. 使用地址索引键 `index:address:{address}` 查询索引
// 2. 解析索引值获取所有 outpoint
// 3. 根据每个 outpoint 查询对应的 UTXO
// 4. 根据 category 过滤（如果指定）
// 5. 根据 onlyAvailable 过滤状态（只返回 AVAILABLE 状态）
func (s *Service) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error) {
	// 1. 验证参数
	if len(address) == 0 {
		return nil, fmt.Errorf("地址不能为空")
	}

	// 2. 构建地址索引键
	// 格式：index:address:{address}
	addressIndexKey := fmt.Sprintf("index:address:%x", address)

	// 3. 从 Storage 获取索引数据
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

	// 4. 解析索引数据，获取所有 outpoint
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

	// 5. 根据每个 outpoint 查询对应的 UTXO，并应用过滤条件
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

		// 5.1 根据 category 过滤（如果指定）
		if category != nil {
			if utxoObj.GetCategory() != *category {
				continue // 类别不匹配，跳过
			}
		}

		// 5.2 根据 onlyAvailable 过滤状态
		if onlyAvailable {
			// 只返回 AVAILABLE 状态的 UTXO
			if utxoObj.GetStatus() != utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE {
				continue // 状态不匹配，跳过
			}
		}

		utxos = append(utxos, utxoObj)
	}

	if s.logger != nil {
		s.logger.Debugf("查询地址 UTXO: address=%x, category=%v, onlyAvailable=%v, count=%d",
			address, category, onlyAvailable, len(utxos))
	}

	return utxos, nil
}

// decodeOutPointList 解码索引数据中的 outpoint 列表
//
// 🔧 索引数据格式：多个固定36字节的 outpoint 序列
// 每个 outpoint: [32字节TxId][4字节OutputIndex] = 36字节
// （与 writer/utxo.go 的 addToAddressIndexInTransaction 保持一致）
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

// GetSponsorPoolUTXOs 获取赞助池UTXO列表（P3-17：完整实现）
//
// 🎯 **查询策略**：
// 1. 使用赞助池地址常量 `constants.SponsorPoolOwner`（全零地址）
// 2. 查询类别为 ASSET 的 UTXO（赞助池只包含资产类型）
// 3. 根据 onlyAvailable 过滤状态
//
// 注意：赞助池 UTXO 具有特殊的 Owner 地址（全零地址），用于标识系统保留的激励池
func (s *Service) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxo.UTXO, error) {
	// 1. 使用赞助池地址常量（全零地址，20字节）
	sponsorPoolAddress := constants.SponsorPoolOwner[:]

	// 2. 查询类别为 ASSET 的 UTXO（赞助池只包含资产类型的 UTXO）
	category := utxo.UTXOCategory_UTXO_CATEGORY_ASSET

	// 3. 复用 GetUTXOsByAddress 方法查询
	utxos, err := s.GetUTXOsByAddress(ctx, sponsorPoolAddress, &category, onlyAvailable)
	if err != nil {
		return nil, fmt.Errorf("查询赞助池 UTXO 失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Debugf("查询赞助池 UTXO: onlyAvailable=%v, count=%d", onlyAvailable, len(utxos))
	}

	return utxos, nil
}

// GetCurrentStateRoot 获取当前UTXO状态根
//
// 🎯 **状态根计算**：
// 基于所有 UTXO 计算 Merkle 根，反映当前 UTXO 集合的状态。
//
// 📋 **计算流程**：
// 1. 扫描所有 UTXO（通过前缀 `utxo:set:`）
// 2. 计算每个 UTXO 的哈希（序列化后哈希）
// 3. 使用 Merkle 树计算根哈希
// 4. 返回32字节状态根
//
// ⚠️ **性能考虑**：
// - 此方法需要扫描所有 UTXO，可能较耗时
// - 建议在 UTXO 变更后异步计算和更新
func (s *Service) GetCurrentStateRoot(ctx context.Context) ([]byte, error) {
	// 1. 获取所有 UTXO（通过前缀扫描）
	// 符合 docs/system/designs/storage/data-architecture.md 规范
	utxoPrefix := []byte("utxo:set:")
	utxoMap, err := s.storage.PrefixScan(ctx, utxoPrefix)
	if err != nil {
		return nil, fmt.Errorf("扫描 UTXO 失败: %w", err)
	}

	// 2. 如果没有 UTXO，返回空哈希
	if len(utxoMap) == 0 {
		if s.logger != nil {
			s.logger.Debug("无 UTXO，返回空状态根")
		}
		return make([]byte, 32), nil
	}

	// 3. 计算每个 UTXO 的哈希
	// ⚠️ 注意：PrefixScan 返回的是 map，遍历顺序不确定。
	// 为了保证 StateRoot 在不同节点上可复现，这里需要对 key 做排序后再计算 Merkle Root。
	utxoHashes := make([][]byte, 0, len(utxoMap))

	// 3.1 收集并排序所有 key，保证遍历顺序确定
	keys := make([]string, 0, len(utxoMap))
	for k := range utxoMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 3.2 按照有序 key 计算每个 UTXO 的哈希
	for _, k := range keys {
		utxoData := utxoMap[k]
		// 验证数据完整性（可选）
		utxoObj := &utxo.UTXO{}
		if err := proto.Unmarshal(utxoData, utxoObj); err != nil {
			if s.logger != nil {
				s.logger.Warnf("反序列化 UTXO 失败，跳过: %v", err)
			}
			continue
		}

		// 计算 UTXO 哈希（使用序列化数据）
		utxoHash := s.hasher.SHA256(utxoData)
		utxoHashes = append(utxoHashes, utxoHash)
	}

	// 4. 使用 Merkle 树计算根哈希
	if len(utxoHashes) == 0 {
		return make([]byte, 32), nil
	}

	// 使用 merkle 包计算 Merkle 根
	hasherAdapter := merkle.NewHashManagerAdapter(s.hasher)
	stateRoot, err := buildMerkleTree(hasherAdapter, utxoHashes)
	if err != nil {
		return nil, fmt.Errorf("计算 Merkle 树失败: %w", err)
	}

	// 确保状态根长度为32字节
	if len(stateRoot) != 32 {
		return nil, fmt.Errorf("状态根长度错误: 期望32字节, 得到%d字节", len(stateRoot))
	}

	if s.logger != nil {
		s.logger.Debugf("✅ 状态根计算完成: %x (UTXO数量=%d)", stateRoot[:8], len(utxoHashes))
	}

	return stateRoot, nil
}

// CheckAssetUTXOConsistency 执行一次资产 UTXO 状态根一致性检查
//
// 设计目标：
// - 通过比较当前计算的 UTXO 状态根与持久化存储的状态根，判断资产 UTXO 是否处于健康状态
// - 如果状态根缺失，则将资产 UTXO 标记为 Degraded（降级但不判定为严重不一致）
// - 如果状态根不一致，则将资产 UTXO 标记为 Inconsistent，便于上层触发自动修复
func (s *Service) CheckAssetUTXOConsistency(ctx context.Context) (bool, error) {
	// 1. 计算当前 UTXO 状态根
	currentRoot, err := s.GetCurrentStateRoot(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnf("AssetUTXO 一致性检查: 计算当前状态根失败: %v", err)
		}
		return false, fmt.Errorf("计算当前 UTXO 状态根失败: %w", err)
	}

	// 2. 读取持久化状态根（由 eutxo.Writer.UpdateStateRoot 维护）
	const stateRootKey = "utxo_state_root"
	storedRoot, err := s.storage.Get(ctx, []byte(stateRootKey))
	if err != nil || len(storedRoot) == 0 {
		// 未找到状态根：处于降级状态，但不视为严重不一致
		runtimectx.SetUTXOHealth(runtimectx.UTXOTypeAsset, runtimectx.UTXOHealthDegraded)
		if s.logger != nil {
			s.logger.Warnf("AssetUTXO 一致性检查: 未找到持久化状态根（key=%s），标记为 Degraded", stateRootKey)
		}
		return false, nil
	}

	// 3. 校验长度
	if len(storedRoot) != len(currentRoot) {
		runtimectx.SetUTXOHealth(runtimectx.UTXOTypeAsset, runtimectx.UTXOHealthInconsistent)
		if s.logger != nil {
			s.logger.Warnf("AssetUTXO 一致性检查: 状态根长度不一致，stored=%d, current=%d",
				len(storedRoot), len(currentRoot))
		}
		return true, nil
	}

	// 4. 比较内容
	if !bytes.Equal(storedRoot, currentRoot) {
		runtimectx.SetUTXOHealth(runtimectx.UTXOTypeAsset, runtimectx.UTXOHealthInconsistent)
		if s.logger != nil {
			s.logger.Warnf("AssetUTXO 一致性检查: 状态根不一致, stored=%x, current=%x",
				storedRoot[:8], currentRoot[:8])
		}
		return true, nil
	}

	// 一致：标记为健康
	runtimectx.SetUTXOHealth(runtimectx.UTXOTypeAsset, runtimectx.UTXOHealthHealthy)
	if s.logger != nil {
		s.logger.Debugf("AssetUTXO 一致性检查通过: 状态根一致=%x", currentRoot[:8])
	}

	return false, nil
}

// RunAssetUTXORepair 执行一次资产 UTXO 自动修复
//
// 当前实现的修复策略：
// - 重新计算当前 UTXO 状态根，并将其写回持久化存储（utxo_state_root）
// - 视当前 UTXO 集合为真实来源，将状态根视为“元数据修复”
// - 不对 UTXO 集合本身做清空和重建（从区块重放的完整重建留待后续扩展）
func (s *Service) RunAssetUTXORepair(ctx context.Context, dryRun bool) error {
	// 1. 计算当前 UTXO 状态根
	currentRoot, err := s.GetCurrentStateRoot(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnf("AssetUTXO 修复: 计算当前状态根失败: %v", err)
		}
		return fmt.Errorf("AssetUTXO 修复时计算状态根失败: %w", err)
	}

	if dryRun {
		if s.logger != nil {
			s.logger.Infof("AssetUTXO 修复（DRY-RUN）: 计算得到状态根=%x，将在非 dry-run 模式下写回 utxo_state_root",
				currentRoot[:8])
		}
		return nil
	}

	// 2. 写回持久化状态根
	const stateRootKey = "utxo_state_root"
	if err := s.storage.Set(ctx, []byte(stateRootKey), currentRoot); err != nil {
		return fmt.Errorf("AssetUTXO 修复: 写回状态根失败: %w", err)
	}

	// 3. 标记为健康
	runtimectx.SetUTXOHealth(runtimectx.UTXOTypeAsset, runtimectx.UTXOHealthHealthy)

	if s.logger != nil {
		s.logger.Infof("✅ AssetUTXO 修复完成: 状态根已更新为当前值=%x（仅修复元数据，不重放区块）", currentRoot[:8])
	}

	return nil
}

// buildMerkleTree 递归构建 Merkle 树
//
// 用于计算 UTXO 状态根的 Merkle 树（使用哈希数组而不是交易列表）
func buildMerkleTree(hasher merkle.Hasher, hashes [][]byte) ([]byte, error) {
	// 基础情况：只有一个节点，返回该节点
	if len(hashes) == 1 {
		return hashes[0], nil
	}

	// 如果节点数为奇数，复制最后一个节点
	if len(hashes)%2 == 1 {
		hashes = append(hashes, hashes[len(hashes)-1])
	}

	// 计算下一层节点
	nextLevel := make([][]byte, 0, len(hashes)/2)
	for i := 0; i < len(hashes); i += 2 {
		// 连接两个子节点的哈希
		combined := append(hashes[i], hashes[i+1]...)

		// 计算父节点哈希
		parentHash, err := hasher.Hash(combined)
		if err != nil {
			return nil, fmt.Errorf("计算父节点哈希失败: %w", err)
		}

		nextLevel = append(nextLevel, parentHash)
	}

	// 递归处理下一层
	return buildMerkleTree(hasher, nextLevel)
}

// 编译时检查接口实现
var _ interfaces.InternalUTXOQuery = (*Service)(nil)
