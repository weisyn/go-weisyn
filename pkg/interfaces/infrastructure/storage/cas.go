package storage

import "context"

// ==================== 内容寻址存储（CAS）公共接口 ====================
//
// 设计目标：
// - 以内容哈希(content_hash)为唯一身份标识，实现环境无关的数据寻址
// - 抽象本地与去中心化存储的统一读写接口，便于无感切换与扩展
// - 保持最小外观：仅定义必要能力，避免实现细节泄漏

// CAS 内容寻址存储最小接口
//
// 约定：
// - Put返回的hash必须为内容确定性哈希（例如SHA-256/32字节），与系统规范一致
// - Get/Has/Remove仅基于hash进行操作
type CAS interface {
	// Put 写入内容，返回内容哈希与大小
	Put(ctx context.Context, content []byte) (hash []byte, size uint64, err error)

	// Get 读取内容
	Get(ctx context.Context, hash []byte) (content []byte, err error)

	// Has 判断内容是否存在
	Has(ctx context.Context, hash []byte) (bool, error)

	// Remove 删除内容（可选能力，允许实现为幂等）
	Remove(ctx context.Context, hash []byte) error
}

// ContentRouter 内容路由最小接口
//
// 设计：
// - 读：本地优先，未命中则回源（如去中心化存储），读后可选择缓存回本地
// - 写：默认写入本地，可选后端复制由实现自行决定（不强制）
type ContentRouter interface {
	// Get 统一读取路径
	Get(ctx context.Context, hash []byte) ([]byte, error)

	// Put 统一写入路径
	Put(ctx context.Context, content []byte) (hash []byte, err error)
}
