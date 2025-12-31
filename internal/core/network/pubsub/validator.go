package pubsub

import (
	"sync"
	"time"

	transportpb "github.com/weisyn/v1/pb/network/transport"
	"google.golang.org/protobuf/proto"
)

// validator.go
// PubSub 消息校验器（可用实现，非占位）：
// - 提供 size / rate-limit / dedup /（可选）signature 的统一入口
// - 通过 WithHasher/WithVerifier 注入“统一哈希/验签”实现（与 crypto 模块解耦）
// - 未来如需更严格的 topic 级策略，可在 TopicRules 上继续扩展

// HashFunc/VerifyFunc 由统一加密服务注入
// HashFunc: 返回 hex/bytes 任一，作为去重键；VerifyFunc: 验签

type HashFunc func([]byte) (string, error)

type VerifyFunc func(payload, sig []byte) (bool, error)

// TopicRules 主题级校验规则（轻量实现）
type TopicRules struct {
	MaxMessageSize   int
	RequireSignature bool
	RatePerSec       int
	DedupTTL         time.Duration
}

// 简易令牌桶（按 topic 维度的轻量限流）
type rateLimiter struct {
	ratePerSec int
	tokens     float64
	last       time.Time
}

func (rl *rateLimiter) allow() bool {
	if rl.ratePerSec <= 0 {
		return true
	}
	now := time.Now()
	if rl.last.IsZero() {
		rl.last = now
		rl.tokens = float64(rl.ratePerSec)
	}
	elapsed := now.Sub(rl.last).Seconds()
	rl.tokens += elapsed * float64(rl.ratePerSec)
	if rl.tokens > float64(rl.ratePerSec) {
		rl.tokens = float64(rl.ratePerSec)
	}
	rl.last = now
	if rl.tokens >= 1 {
		rl.tokens -= 1
		return true
	}
	return false
}

// Validator 校验器（并发安全）
type Validator struct {
	mu        sync.RWMutex
	rules     map[string]TopicRules
	lim       map[string]*rateLimiter
	dedup     map[string]map[string]time.Time // topic -> key -> ts
	hashFunc  HashFunc
	verifySig VerifyFunc
}

// NewValidator 创建校验器
func NewValidator() *Validator {
	return &Validator{rules: make(map[string]TopicRules), lim: make(map[string]*rateLimiter), dedup: make(map[string]map[string]time.Time)}
}

// WithHasher 注入统一哈希函数
func (v *Validator) WithHasher(h HashFunc) *Validator { v.hashFunc = h; return v }

// WithVerifier 注入统一验签函数
func (v *Validator) WithVerifier(vf VerifyFunc) *Validator { v.verifySig = vf; return v }

// ConfigureTopic 配置主题规则
func (v *Validator) ConfigureTopic(topic string, r TopicRules) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.rules[topic] = r
	if r.RatePerSec > 0 {
		v.lim[topic] = &rateLimiter{ratePerSec: r.RatePerSec}
	}
	if _, ok := v.dedup[topic]; !ok {
		v.dedup[topic] = make(map[string]time.Time)
	}
}

// Validate 校验主题消息
// 参数 data: 期望为 Envelope 的序列化字节
// 返回：
//   - bool: 是否通过
//   - string: 失败原因（通过时为空）
func (v *Validator) Validate(topic string, data []byte) (bool, string) {
	v.mu.RLock()
	r, ok := v.rules[topic]
	rl := v.lim[topic]
	dedup := v.dedup[topic]
	v.mu.RUnlock()
	if !ok {
		return true, ""
	}
	// 速率限制
	if rl != nil && !rl.allow() {
		return false, "rate_limited"
	}
	// 尝试解 Envelope
	var env transportpb.Envelope
	if err := proto.Unmarshal(data, &env); err != nil {
		// 如果不是 Envelope，回退仅做大小与去重（以原始数据）
		if r.MaxMessageSize > 0 && len(data) > r.MaxMessageSize {
			return false, "size_exceeded"
		}
		return v.checkDedup(topic, r, data, dedup)
	}
	payload := env.GetPayload()
	// 大小限制（针对业务载荷）
	if r.MaxMessageSize > 0 && len(payload) > r.MaxMessageSize {
		return false, "size_exceeded"
	}
	// 签名校验（仅当要求且提供 VerifyFunc）
	if r.RequireSignature && v.verifySig != nil {
		ok, _ := v.verifySig(payload, env.GetSignature())
		if !ok {
			return false, "bad_signature"
		}
	}
	// 去重（优先使用 Envelope.dedup_key，否则对 payload 取哈希/回退原始字节）
	key := env.GetDedupKey()
	if key == "" {
		if v.hashFunc != nil {
			if h, err := v.hashFunc(payload); err == nil {
				key = h
			}
		}
		if key == "" {
			key = string(payload)
		}
	}
	if r.DedupTTL > 0 {
		if hit := v.checkAndMarkDedup(topic, key, r.DedupTTL, dedup); hit {
			return false, "duplicate"
		}
	}
	return true, ""
}

func (v *Validator) checkDedup(topic string, r TopicRules, data []byte, table map[string]time.Time) (bool, string) {
	if r.DedupTTL <= 0 {
		return true, ""
	}
	key := ""
	if v.hashFunc != nil {
		if h, err := v.hashFunc(data); err == nil {
			key = h
		}
	}
	if key == "" {
		key = string(data)
	}
	if v.checkAndMarkDedup(topic, key, r.DedupTTL, table) {
		return false, "duplicate"
	}
	return true, ""
}

func (v *Validator) checkAndMarkDedup(topic, key string, ttl time.Duration, table map[string]time.Time) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	if ts, ok := table[key]; ok {
		if time.Since(ts) < ttl {
			return true
		}
	}
	table[key] = time.Now()
	return false
}

// CleanupExpiredEntries 清理过期的去重条目（防止内存泄漏）
func (v *Validator) CleanupExpiredEntries() {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := time.Now()
	for topic, entries := range v.dedup {
		for k, ts := range entries {
			if rule, ok := v.rules[topic]; ok && rule.DedupTTL > 0 {
				if now.Sub(ts) > rule.DedupTTL {
					delete(entries, k)
				}
			}
		}
		if len(entries) == 0 {
			delete(v.dedup, topic)
		}
	}
}

// RemoveTopic 移除主题的所有校验规则和资源
func (v *Validator) RemoveTopic(topic string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.rules, topic)
	delete(v.lim, topic)
	delete(v.dedup, topic)
}
