// Package cas 实现内容寻址存储（CAS）服务
//
// 🎯 **核心职责**：
// - 实现 InternalCASStorage 接口
// - 提供内容寻址文件存储功能
// - 文件路径构建和管理
//
// 💡 **设计特点**：
// - 并发安全：使用 RWMutex 保护共享状态
// - 性能监控：收集性能指标
// - 日志记录：记录关键操作
// - 幂等性：相同内容只存储一次
package cas

import (
	"sync"

	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	"github.com/weisyn/v1/internal/core/ures/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// Service CASStorage服务实现
//
// 🎯 **核心职责**：
// - 实现 InternalCASStorage 接口
// - 提供内容寻址文件存储功能
//
// 💡 **设计特点**：
// - 并发安全：使用 RWMutex 保护共享状态
// - 日志记录：记录关键操作
type Service struct {
	mu        sync.RWMutex       // 读写锁
	fileStore storage.FileStore  // 文件存储
	hasher    crypto.HashManager // 哈希计算
	logger    log.Logger         // 日志记录
}

// NewService 创建CASStorage服务
//
// 参数：
//   - fileStore: 文件存储服务（必需）
//   - hasher: 哈希计算服务（必需）
//   - logger: 日志服务（可选）
//
// 返回：
//   - interfaces.InternalCASStorage: CASStorage服务实例
//   - error: 初始化错误
//
// 示例：
//
//	casStorage, err := cas.NewService(fileStore, hasher, logger)
//	if err != nil {
//	    return err
//	}
func NewService(
	fileStore storage.FileStore,
	hasher crypto.HashManager,
	logger log.Logger,
) (interfaces.InternalCASStorage, error) {
	// 1. 验证参数
	if fileStore == nil {
		return nil, ErrFileStoreNil
	}
	if hasher == nil {
		return nil, ErrHasherNil
	}

	// 2. 创建服务实例
	s := &Service{
		fileStore: fileStore,
		hasher:    hasher,
		logger:    logger,
	}

	// 3. 日志记录
	if logger != nil {
		logger.Info("✅ CASStorage 服务已创建")
	}

	return s, nil
}

// 编译时检查接口实现
var _ interfaces.InternalCASStorage = (*Service)(nil)

// ============================================================================
// 内存监控接口实现（MemoryReporter）
// ============================================================================

// ModuleName 返回模块名称（实现 MemoryReporter 接口）
func (s *Service) ModuleName() string {
	return "ures"
}

func (s *Service) CollectMemoryStats() metricsiface.ModuleMemoryStats {
	// 当前 CASStorage 实现本身不维护显式的 in-memory 索引或缓存结构，
	// 主要内存占用在底层 FileStore 中，由存储层单独监控。
	// 为避免“拍脑袋估值”误导运维，这里明确返回 0，表示：
	// - Objects:     本模块未跟踪的对象数量
	// - ApproxBytes: 本模块未单独统计的内存字节数
	// - CacheItems:  本模块未维护的缓存条目数
	return metricsiface.ModuleMemoryStats{
		Module:      "ures",
		Layer:       "L4-CoreBusiness",
		Objects:     0,
		ApproxBytes: 0,
		CacheItems:  0,
		QueueLength: 0,
	}
}

