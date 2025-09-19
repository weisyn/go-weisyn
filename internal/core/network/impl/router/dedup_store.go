package router

import "time"

// dedup_store.go
// 去重存储（方法框架）：
// - 维护 messageID 的 TTL 记录
// - 提供存在性检查与记录写入

// DedupStore 去重存储（方法框架）
type DedupStore struct{}

// NewDedupStore 创建去重存储
func NewDedupStore() *DedupStore { return &DedupStore{} }

// Seen 检查是否已见过该消息
func (s *DedupStore) Seen(messageID string) bool { return false }

// Put 记录消息并设置 TTL
func (s *DedupStore) Put(messageID string, ttl time.Duration) error { return nil }

// Cleanup 清理过期记录
func (s *DedupStore) Cleanup() (int, error) { return 0, nil }
