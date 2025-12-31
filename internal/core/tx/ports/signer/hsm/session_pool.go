//go:build !android && !ios && cgo
// +build !android,!ios,cgo

// Package hsm 提供 HSM（Hardware Security Module）签名器实现
//
// session_pool.go: PKCS#11 Session 池管理
//
// 🎯 **核心职责**：高效管理和复用 PKCS#11 Session
//
// 💡 **设计理念**：
// - Session 是有限资源，需要高效复用
// - 使用连接池模式管理 Session
// - 支持并发安全的 Session 获取和释放
package hsm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/miekg/pkcs11"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// SessionPool PKCS#11 Session 池
//
// 🎯 **核心功能**：管理 HSM Session 的创建、复用和清理
//
// 💡 **设计原则**：
// - 池大小可配置（默认10个Session）
// - 自动清理空闲Session
// - 并发安全
type SessionPool struct {
	ctx       *PKCS11Context    // PKCS#11 上下文
	slotID    uint               // Slot ID
	pin       string              // PIN码（已解密）
	maxSize   int                 // 最大Session数量
	sessions  []pkcs11.SessionHandle // Session列表
	inUse     map[pkcs11.SessionHandle]bool // 使用中的Session
	mu        sync.RWMutex        // 读写锁
	cond      *sync.Cond           // 条件变量（用于等待可用Session）
	logger    log.Logger          // 日志服务
	cleanupInterval time.Duration // 清理间隔
	stopCleanup     chan struct{} // 停止清理信号
}

// SessionPoolConfig Session池配置
type SessionPoolConfig struct {
	MaxSize         int           // 最大Session数量
	PIN             string        // PIN码（明文，将从配置中解密）
	CleanupInterval time.Duration // 清理间隔（默认5分钟）
}

// NewSessionPool 创建 Session 池
//
// 参数：
//   - ctx: PKCS#11 上下文
//   - slotID: Slot ID
//   - config: Session池配置
//   - logger: 日志服务
//
// 返回：
//   - *SessionPool: Session池实例
//   - error: 创建失败的原因
func NewSessionPool(
	ctx *PKCS11Context,
	slotID uint,
	config *SessionPoolConfig,
	logger log.Logger,
) (*SessionPool, error) {
	if ctx == nil {
		return nil, fmt.Errorf("PKCS#11上下文不能为空")
	}

	maxSize := config.MaxSize
	if maxSize <= 0 {
		maxSize = 10 // 默认10个Session
	}

	cleanupInterval := config.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute // 默认5分钟
	}

	pool := &SessionPool{
		ctx:            ctx,
		slotID:         slotID,
		pin:            config.PIN,
		maxSize:        maxSize,
		sessions:       make([]pkcs11.SessionHandle, 0, maxSize),
		inUse:          make(map[pkcs11.SessionHandle]bool),
		logger:         logger,
		cleanupInterval: cleanupInterval,
		stopCleanup:    make(chan struct{}),
	}
	// ✅ 修复：初始化条件变量
	pool.cond = sync.NewCond(&pool.mu)

	// 启动清理协程
	go pool.cleanupLoop()

	if logger != nil {
		logger.Infof("✅ Session池初始化成功，最大Session数: %d", maxSize)
	}

	return pool, nil
}

// AcquireSession 获取一个可用的Session
//
// 参数：
//   - ctx: 上下文对象（用于超时控制）
//
// 返回：
//   - pkcs11.SessionHandle: Session句柄
//   - error: 获取失败的原因
func (p *SessionPool) AcquireSession(ctx context.Context) (pkcs11.SessionHandle, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 1. 尝试复用空闲Session
	for _, session := range p.sessions {
		if !p.inUse[session] && p.isSessionValid(session) {
			p.inUse[session] = true
			if p.logger != nil {
				p.logger.Debugf("复用Session: %d", session)
			}
			return session, nil
		}
	}

	// 2. 创建新Session（如果未达到上限）
	if len(p.sessions) < p.maxSize {
		session, err := p.createSession()
		if err != nil {
			return 0, fmt.Errorf("创建Session失败: %w", err)
		}
		p.sessions = append(p.sessions, session)
		p.inUse[session] = true
		if p.logger != nil {
			p.logger.Debugf("创建新Session: %d (总数: %d/%d)", session, len(p.sessions), p.maxSize)
		}
		return session, nil
	}

	// 3. 达到上限，等待可用Session（带超时）
	// ✅ 修复：使用条件变量等待可用Session
	// 注意：由于 context 超时控制复杂，这里先实现基本的等待机制
	// 调用方应通过 context 控制总体超时
	for {
		// 检查是否有可用Session
		for _, session := range p.sessions {
			if !p.inUse[session] && p.isSessionValid(session) {
				p.inUse[session] = true
				if p.logger != nil {
					p.logger.Debugf("等待后获取Session: %d", session)
				}
				return session, nil
			}
		}

		// 检查超时
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("获取Session超时: %w", ctx.Err())
		default:
		}

		// 等待Session释放（使用条件变量）
		p.cond.Wait()
	}
}

// ReleaseSession 释放Session（标记为空闲）
//
// 参数：
//   - session: Session句柄
func (p *SessionPool) ReleaseSession(session pkcs11.SessionHandle) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.inUse[session] {
		p.inUse[session] = false
		if p.logger != nil {
			p.logger.Debugf("释放Session: %d", session)
		}
		// ✅ 修复：通知等待的 goroutine
		p.cond.Signal()
	}
}

// CloseSession 关闭Session（从池中移除）
//
// 参数：
//   - session: Session句柄
func (p *SessionPool) CloseSession(session pkcs11.SessionHandle) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 从池中移除
	for i, s := range p.sessions {
		if s == session {
			p.sessions = append(p.sessions[:i], p.sessions[i+1:]...)
			break
		}
	}
	delete(p.inUse, session)

	// 关闭Session
	return p.ctx.CloseSession(session)
}

// Close 关闭所有Session并清理资源
func (p *SessionPool) Close() error {
	// 停止清理协程
	close(p.stopCleanup)

	p.mu.Lock()
	defer p.mu.Unlock()

	// 关闭所有Session
	for _, session := range p.sessions {
		if err := p.ctx.CloseSession(session); err != nil {
			if p.logger != nil {
				p.logger.Warnf("关闭Session失败: %v", err)
			}
		}
	}

	p.sessions = p.sessions[:0]
	p.inUse = make(map[pkcs11.SessionHandle]bool)

	if p.logger != nil {
		p.logger.Info("✅ Session池已关闭")
	}

	return nil
}

// createSession 创建新Session
func (p *SessionPool) createSession() (pkcs11.SessionHandle, error) {
	const CKF_SERIAL_SESSION = 0x00000004
	const CKF_RW_SESSION = 0x00000002
	session, err := p.ctx.OpenSession(CKF_SERIAL_SESSION | CKF_RW_SESSION)
	if err != nil {
		return 0, fmt.Errorf("OpenSession失败: %w", err)
	}

	// 登录（如果需要）
	if p.pin != "" {
		if err := p.ctx.Login(session, p.pin); err != nil {
			p.ctx.CloseSession(session)
			return 0, fmt.Errorf("Login失败: %w", err)
		}
	}

	return session, nil
}

// isSessionValid 检查Session是否有效
func (p *SessionPool) isSessionValid(session pkcs11.SessionHandle) bool {
	// ✅ **真实实现**：调用 PKCS#11 API 检查Session状态
	// 使用 C_GetSessionInfo 获取 Session 信息，检查 Session 是否仍然有效
	info, err := p.ctx.GetSessionInfo(session)
	if err != nil {
		// 如果获取 Session 信息失败，认为 Session 无效
		if p.logger != nil {
			p.logger.Debugf("Session %d 无效: %v", session, err)
		}
		return false
	}

	// 检查 Session 状态
	// CKS_RO_PUBLIC_SESSION: 只读公共 Session（未登录）
	// CKS_RO_USER_FUNCTIONS: 只读用户 Session（已登录）
	// CKS_RW_PUBLIC_SESSION: 读写公共 Session（未登录）
	// CKS_RW_USER_FUNCTIONS: 读写用户 Session（已登录）
	// CKS_RW_SO_FUNCTIONS: 读写安全官 Session
	// 如果 Session 状态为 0 或无效值，认为 Session 无效
	if info.State == 0 {
		if p.logger != nil {
			p.logger.Debugf("Session %d 状态无效: State=0", session)
		}
		return false
	}

	// Session 有效
	return true
}

// cleanupLoop 定期清理无效Session
func (p *SessionPool) cleanupLoop() {
	ticker := time.NewTicker(p.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanupInvalidSessions()
		case <-p.stopCleanup:
			return
		}
	}
}

// cleanupInvalidSessions 清理无效Session
func (p *SessionPool) cleanupInvalidSessions() {
	p.mu.Lock()
	defer p.mu.Unlock()

	validSessions := make([]pkcs11.SessionHandle, 0, len(p.sessions))
	for _, session := range p.sessions {
		if p.inUse[session] {
			// 使用中的Session保留
			validSessions = append(validSessions, session)
			continue
		}

		// 检查Session是否有效
		if !p.isSessionValid(session) {
			// 关闭无效Session
			if err := p.ctx.CloseSession(session); err != nil {
				if p.logger != nil {
					p.logger.Warnf("清理无效Session失败: %v", err)
				}
			} else {
				if p.logger != nil {
					p.logger.Debugf("清理无效Session: %d", session)
				}
			}
			delete(p.inUse, session)
		} else {
			// 有效Session保留
			validSessions = append(validSessions, session)
		}
	}

	p.sessions = validSessions
}

// GetStats 获取Session池统计信息
func (p *SessionPool) GetStats() (total, inUse, idle int) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total = len(p.sessions)
	inUse = 0
	for _, used := range p.inUse {
		if used {
			inUse++
		}
	}
	idle = total - inUse

	return total, inUse, idle
}

