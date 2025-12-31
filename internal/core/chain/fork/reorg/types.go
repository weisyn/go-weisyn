package reorg

import (
	"time"
)

// Phase 表示一次 REORG 的阶段（严格对齐设计文档：Prepare/Rollback/Replay/Verify/Commit）
type Phase string

const (
	PhasePrepare Phase = "Prepare"
	PhaseRollback Phase = "Rollback"
	PhaseReplay   Phase = "Replay"
	PhaseVerify   Phase = "Verify"
	PhaseCommit   Phase = "Commit"
)

// ErrorClass 用于对 REORG 失败原因进行分类，便于恢复策略与运维告警。
type ErrorClass string

const (
	ErrClassPrepare  ErrorClass = "prepare_failed"
	ErrClassRollback ErrorClass = "rollback_failed"
	ErrClassReplay   ErrorClass = "replay_failed"
	ErrClassVerify   ErrorClass = "verify_failed"
	ErrClassCommit   ErrorClass = "commit_failed"
	ErrClassAbort    ErrorClass = "abort_failed"
	ErrClassUnknown  ErrorClass = "unknown_error"
)

// ReorgError 表示一次 REORG 中的结构化错误（带阶段与分类）。
type ReorgError struct {
	Class ErrorClass
	Phase Phase
	Err   error
}

func (e *ReorgError) Error() string {
	if e == nil || e.Err == nil {
		return "reorg error: <nil>"
	}
	return string(e.Class) + "@" + string(e.Phase) + ": " + e.Err.Error()
}

func (e *ReorgError) Unwrap() error { return e.Err }

// RollbackHandle 回滚句柄（跨模块统一标识）。
//
// 注意：为了可序列化与可日志化，这里将 Metadata 限制为 string->string（不使用 interface{}）。
type RollbackHandle struct {
	ID        string
	Height    uint64
	CreatedAt time.Time
	Metadata  map[string]string
}

// ReorgSession 代表一次 REORG 会话（包含两个关键回滚点：recovery 与 rollback）。
type ReorgSession struct {
	ID         string
	FromHeight uint64 // 当前主链 tip 高度
	ForkHeight uint64 // 共同祖先高度
	ToHeight   uint64 // 新主链 tip 高度
	CreatedAt  time.Time

	// handles: module_name -> handle
	Handles map[string]RollbackHandle
}

// VerificationResult 表示验证结果（不允许“简化版”——必须携带每项检查的细节）。
type VerificationResult struct {
	Passed bool
	Checks []CheckResult
}

type CheckResult struct {
	Name     string
	Passed   bool
	Expected string
	Actual   string
	Details  string
}

// IndexRollbackPlan 是“索引回滚删除计划”（事务外预收集、事务内原子执行）。
//
// 说明：
// - 这是严格原子化回滚的前置（rollback-plan-refactor）。
// - 计划本身不执行任何写入；执行由协调器/管理器在 BadgerTransaction 中完成。
type IndexRollbackPlan struct {
	TargetHeight uint64
	// keys to delete
	HeightKeys   [][]byte
	HashKeys     [][]byte
	TxKeys       [][]byte
	ResourceKeys [][]byte
	// tip value: height(8)+hash(32)
	TipValue []byte
}


