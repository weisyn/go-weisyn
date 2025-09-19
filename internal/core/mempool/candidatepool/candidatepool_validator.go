// 文件说明：
// 本文件定义候选区块池（CandidatePool）的基础安全验证器接口与生产实现，
// 负责对候选区块的格式、哈希、大小、重复性与高度期待值进行基础校验，
// 不涉及业务层面的复杂验证（如交易执行、共识规则等）。
package candidatepool

import (
	"fmt"

	"github.com/weisyn/v1/internal/config/candidatepool"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// BasicCandidateValidator 基础候选区块验证器接口。
// 方法：
// - ValidateFormat：校验区块对象与关键字段完整性；
// - ValidateHash：校验外部计算得到的哈希是否符合预期；
// - ValidateSize：基于配置校验区块估算大小；
// - ValidateDuplicate：通过外部注入函数检测是否重复；
// - ValidateExpectedHeight：校验区块高度是否满足 currentHeight+1。
type BasicCandidateValidator interface {
	ValidateFormat(block *core.Block) error
	ValidateHash(block *core.Block, expectedHash []byte) error
	ValidateSize(block *core.Block) error

	// 返回 (是否重复, 错误)
	ValidateDuplicate(blockHash []byte) (bool, error)
	// 返回 (是否满足期待高度, 错误)
	ValidateExpectedHeight(block *core.Block, currentHeight uint64) (bool, error)
}

// ProductionBasicCandidateValidator 生产级基础候选区块验证器实现。
// 字段说明：
// - config：候选池配置；
// - logger：日志接口；
// - duplicateExistsFn：重复检测回调，由外部在池构造完成后回绑。
type ProductionBasicCandidateValidator struct {
	config            *candidatepool.CandidatePoolOptions
	logger            log.Logger
	duplicateExistsFn func(hash []byte) bool
}

// NewBasicCandidateValidator 创建基础验证器。
// 参数：
// - config：候选池配置；
// - logger：日志接口；
// - duplicateExistsFn：重复检测回调（可先占位，池完成后回绑）。
// 返回：BasicCandidateValidator 实例。
func NewBasicCandidateValidator(
	config *candidatepool.CandidatePoolOptions,
	logger log.Logger,
	duplicateExistsFn func(hash []byte) bool,
) BasicCandidateValidator {
	return &ProductionBasicCandidateValidator{
		config:            config,
		logger:            logger,
		duplicateExistsFn: duplicateExistsFn,
	}
}

// ValidateFormat 验证候选区块格式与关键字段。
// 参数：
// - block：待校验区块。
// 返回：
// - error：格式不合法时返回错误。
func (v *ProductionBasicCandidateValidator) ValidateFormat(block *core.Block) error {
	if block == nil {
		return fmt.Errorf("候选区块不能为空")
	}

	if block.Header == nil {
		return fmt.Errorf("候选区块头不能为空")
	}

	if block.Body == nil {
		return fmt.Errorf("候选区块体不能为空")
	}

	// 基础字段
	if block.Header.Height == 0 {
		return fmt.Errorf("候选区块高度无效")
	}

	return nil
}

// ValidateHash 校验外部计算的哈希是否满足预期。
// 参数：
// - block：候选区块；
// - expectedHash：外部计算出的区块哈希。
// 返回：
// - error：预期哈希缺失或将来扩展校验失败时返回错误。
func (v *ProductionBasicCandidateValidator) ValidateHash(block *core.Block, expectedHash []byte) error {
	if len(expectedHash) == 0 {
		return fmt.Errorf("预期哈希不能为空")
	}
	// 可在此处加入更严格的哈希一致性验证
	return nil
}

// ValidateSize 基于配置校验区块估算大小。
// 参数：
// - block：候选区块。
// 返回：
// - error：当估算大小超过配置上限时返回错误。
func (v *ProductionBasicCandidateValidator) ValidateSize(block *core.Block) error {
	if v.config == nil {
		return fmt.Errorf("配置不能为空")
	}

	maxSize := v.config.MaxBlockSize
	if maxSize == 0 {
		maxSize = 10 * 1024 * 1024 // 默认10MB
	}

	estimatedSize := estimateBlockSize(block)
	if estimatedSize > uint64(maxSize) {
		return fmt.Errorf("候选区块大小 %d 超过限制 %d", estimatedSize, maxSize)
	}

	return nil
}

// ValidateDuplicate 使用外部重复检测回调判断区块是否重复。
// 参数：
// - blockHash：区块哈希。
// 返回：
// - bool：true 表示重复；false 表示不重复；
// - error：回调未注入或输入非法时返回错误。
func (v *ProductionBasicCandidateValidator) ValidateDuplicate(blockHash []byte) (bool, error) {
	if len(blockHash) == 0 {
		return false, fmt.Errorf("区块哈希不能为空")
	}
	if v.duplicateExistsFn == nil {
		return false, fmt.Errorf("重复检测函数未注入")
	}
	return v.duplicateExistsFn(blockHash), nil
}

// ValidateExpectedHeight 校验区块高度是否等于 currentHeight+1。
// 参数：
// - block：候选区块；
// - currentHeight：当前链高度。
// 返回：
// - bool：true 表示满足期待高度；
// - error：区块或区块头为空时返回错误。
func (v *ProductionBasicCandidateValidator) ValidateExpectedHeight(block *core.Block, currentHeight uint64) (bool, error) {
	if block == nil || block.Header == nil {
		return false, fmt.Errorf("候选区块或区块头不能为空")
	}
	expectedHeight := currentHeight + 1
	if block.Header.Height != expectedHeight {
		return false, nil
	}
	return true, nil
}

// estimateBlockSize 估算区块大小（近似）。
// 参数：
// - block：候选区块。
// 返回：
// - uint64：估算字节数。
func estimateBlockSize(block *core.Block) uint64 {
	if block == nil {
		return 0
	}

	// 基础区块头大小
	headerSize := uint64(200)

	// 交易数量影响（简单线性近似）
	txSize := uint64(len(block.Body.Transactions)) * 500 // 每个交易约500字节

	// 其他元数据
	metadataSize := uint64(100)

	return headerSize + txSize + metadataSize
}
