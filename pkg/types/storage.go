package types

import "time"

// 存储相关通用数据结构

// FileInfo 定义文件的元数据信息
type FileInfo struct {
	Size       int64     // 文件大小（字节）
	CreateTime time.Time // 创建时间
	ModTime    time.Time // 修改时间
	IsDir      bool      // 是否目录
}

// TempFileInfo 定义临时文件的信息
type TempFileInfo struct {
	ID         string    // 临时文件唯一标识
	Size       int64     // 大小
	CreateTime time.Time // 创建时间
	ExpireTime time.Time // 过期时间
}

// ProviderOptions 存储提供者选项（聚合各存储实例配置）
type ProviderOptions struct {
	StorageDir   string                            `json:"storage_dir"`
	BadgerStores map[string]map[string]interface{} `json:"badger_stores"`
	MemoryStores map[string]map[string]interface{} `json:"memory_stores"`
	FileStores   map[string]map[string]interface{} `json:"file_stores"`
	SQLiteStores map[string]map[string]interface{} `json:"sqlite_stores"`
	TempStores   map[string]map[string]interface{} `json:"temp_stores"`
}
