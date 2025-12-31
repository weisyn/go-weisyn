// Package writer 实现资源写入服务
//
// 🎯 **核心职责**：
// - 实现 InternalResourceWriter 接口
// - 提供资源文件存储功能（内容寻址存储）
//
// 💡 **设计特点**：
// - 并发安全：使用 Mutex 保护共享状态
// - 依赖 CASStorage：使用内容寻址存储
// - 性能监控：收集性能指标
// - 日志记录：记录关键操作
// - 职责明确：只负责文件存储，资源索引更新由 DataWriter 统一处理
package writer

import (
	"github.com/weisyn/v1/internal/core/ures/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// Service ResourceWriter服务实现
//
// 🎯 **核心职责**：
// - 实现 InternalResourceWriter 接口
// - 提供资源文件存储功能（内容寻址存储）
//
// 💡 **设计特点**：
// - 依赖 CASStorage：使用内容寻址存储
// - 日志记录：记录关键操作
// - 职责明确：只负责文件存储，资源索引更新由 DataWriter 统一处理
type Service struct {
	casStorage interfaces.InternalCASStorage // CAS存储
	hasher     crypto.HashManager            // 哈希计算
	logger     log.Logger                    // 日志记录
}

// NewService 创建ResourceWriter服务
//
// 参数：
//   - casStorage: CASStorage服务（必需）
//   - hasher: 哈希计算服务（必需）
//   - logger: 日志服务（可选）
//
// 返回：
//   - interfaces.InternalResourceWriter: ResourceWriter服务实例
//   - error: 初始化错误
//
// 示例：
//
//	resourceWriter, err := writer.NewService(casStorage, hasher, logger)
//	if err != nil {
//	    return err
//	}
func NewService(
	casStorage interfaces.InternalCASStorage,
	hasher crypto.HashManager,
	logger log.Logger,
) (interfaces.InternalResourceWriter, error) {
	// 1. 验证参数
	if casStorage == nil {
		return nil, ErrCASStorageNil
	}
	if hasher == nil {
		return nil, ErrHasherNil
	}

	// 2. 创建服务实例
	s := &Service{
		casStorage: casStorage,
		hasher:     hasher,
		logger:     logger,
	}

	// 3. 日志记录
	if logger != nil {
		logger.Info("✅ ResourceWriter 服务已创建")
	}

	return s, nil
}

// 编译时检查接口实现
var _ interfaces.InternalResourceWriter = (*Service)(nil)

