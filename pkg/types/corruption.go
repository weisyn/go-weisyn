package types

// CorruptionComponent 标识发生损坏/自愈的组件域
type CorruptionComponent string

const (
	CorruptionComponentPersistence CorruptionComponent = "persistence"
	CorruptionComponentSync        CorruptionComponent = "sync"
	CorruptionComponentValidator   CorruptionComponent = "validator"
	CorruptionComponentUTXO        CorruptionComponent = "utxo"
	CorruptionComponentFork        CorruptionComponent = "fork"
)

// CorruptionPhase 标识发生损坏的阶段（便于快速定位）
type CorruptionPhase string

const (
	CorruptionPhaseReadIndex  CorruptionPhase = "read_index"
	CorruptionPhaseReadBlock  CorruptionPhase = "read_block"
	CorruptionPhaseValidate   CorruptionPhase = "validate"
	CorruptionPhaseApply      CorruptionPhase = "apply"
	CorruptionPhaseReorg      CorruptionPhase = "reorg"
)

// CorruptionSeverity 严重程度（用于告警/自动降级）
type CorruptionSeverity string

const (
	CorruptionSeverityInfo     CorruptionSeverity = "info"
	CorruptionSeverityWarning  CorruptionSeverity = "warning"
	CorruptionSeverityCritical CorruptionSeverity = "critical"
)

// CorruptionEventData corruption.detected 事件载荷
type CorruptionEventData struct {
	Component CorruptionComponent `json:"component"`
	Phase     CorruptionPhase     `json:"phase"`
	Severity  CorruptionSeverity  `json:"severity"`

	// Height/Hash/Key 三者按场景可选其一或组合：
	// - index 错误：Key（如 indices:hash:<hash>）
	// - block 错误：Height + Hash
	Height *uint64 `json:"height,omitempty"`
	Hash   string  `json:"hash,omitempty"` // hex
	Key    string  `json:"key,omitempty"`

	// err_class：便于 RepairManager 路由到对应修复器
	ErrClass string `json:"err_class"`
	Error    string `json:"error"`

	// node_id：可选（由上层注入），用于多节点排查
	NodeID string `json:"node_id,omitempty"`
	At     RFC3339Time `json:"at"`
}

// CorruptionRepairEventData repair 结果事件载荷（repaired / repair_failed）
type CorruptionRepairEventData struct {
	Component CorruptionComponent `json:"component"`
	Phase     CorruptionPhase     `json:"phase"`

	TargetKey   string `json:"target_key,omitempty"`
	TargetHash  string `json:"target_hash,omitempty"`
	TargetHeight *uint64 `json:"target_height,omitempty"`

	Action   string `json:"action"` // e.g. "rebuild_hash_index" / "rebuild_tip" / "rollback_utxo"
	Result   string `json:"result"` // "success" | "failed" | "skipped"
	Details  string `json:"details,omitempty"`
	Error    string `json:"error,omitempty"`

	NodeID string `json:"node_id,omitempty"`
	At     RFC3339Time `json:"at"`
}


