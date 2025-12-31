package draftstore

// getDefaultStoreType 获取默认存储类型
//
// 🎯 **默认值策略**：
// - 默认使用内存存储（memory），适用于单节点场景
// - Redis存储需要在配置中显式启用
func getDefaultStoreType() string {
	return "memory"
}

// getDefaultMemoryConfig 获取默认内存存储配置
func getDefaultMemoryConfig() MemoryDraftStoreConfig {
	return MemoryDraftStoreConfig{
		MaxDrafts:              1000, // 默认最大1000个草稿
		CleanupIntervalSeconds: 3600, // 默认1小时清理一次
	}
}

// getDefaultRedisConfig 获取默认Redis存储配置
//
// 🎯 **默认值策略**：
// - 地址：默认localhost:28791（WES 端口规范，避免占用常用 Redis 默认端口）
// - 生产环境必须通过配置提供Redis地址
func getDefaultRedisConfig() RedisDraftStoreConfig {
	return RedisDraftStoreConfig{
		Addr:         "localhost:28791", // 默认开发环境地址
		Password:     "",                // 默认无密码
		DB:           0,                 // 默认数据库0
		KeyPrefix:    "weisyn:draft:",   // 默认键前缀
		DefaultTTL:   3600,              // 默认1小时TTL
		PoolSize:     10,                // 默认连接池大小10
		MinIdleConns: 5,                 // 默认最小空闲连接5
		DialTimeout:  5,                 // 默认连接超时5秒
		ReadTimeout:  3,                 // 默认读超时3秒
		WriteTimeout: 3,                 // 默认写超时3秒
	}
}
