// Package merkle 提供Merkle树计算和验证功能
package merkle

import (
	"fmt"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

// HashManagerAdapter 将 HashManager 适配为 Hasher 接口
//
// 这个适配器允许我们在需要 Hasher 接口的地方使用 HashManager。
// 默认使用 SHA256 算法进行哈希计算。
type HashManagerAdapter struct {
	manager crypto.HashManager
}

// NewHashManagerAdapter 创建 HashManager 适配器
//
// 参数：
//   - manager: HashManager 实例
//
// 返回：
//   - Hasher: 适配后的 Hasher 接口
func NewHashManagerAdapter(manager crypto.HashManager) Hasher {
	return &HashManagerAdapter{
		manager: manager,
	}
}

// Hash 实现 Hasher 接口
//
// 使用 HashManager.SHA256() 方法计算哈希
func (a *HashManagerAdapter) Hash(data []byte) ([]byte, error) {
	if a.manager == nil {
		return nil, fmt.Errorf("哈希管理器未初始化")
	}

	// 使用 SHA256 算法
	hash := a.manager.SHA256(data)

	// HashManager.SHA256() 不返回错误，这里也不返回错误
	return hash, nil
}

// 编译时检查接口实现
var _ Hasher = (*HashManagerAdapter)(nil)

