package interfaces

import (
	"context"
	"time"

	"github.com/weisyn/v1/pkg/types"
)

// ==================== 安全验证内部接口 ====================
// 这些接口供execution内部子目录相互调用，不对外暴露

// SecurityValidator 安全验证器接口
// 由security包实现，供coordinator调用
type SecurityValidator interface {
	// 验证执行权限
	ValidateExecution(ctx context.Context, params types.ExecutionParams) error

	// 验证导入权限
	ValidateImports(imports []string) error

	// 验证宿主调用权限
	ValidateHostCall(functionName string, params []interface{}) error

	// 获取安全级别
	GetSecurityLevel() SecurityLevel

	// 获取安全统计
	GetSecurityStats() *SecurityStats
}

// QuotaManager 配额管理器接口
// 由security包实现，供coordinator调用
type QuotaManager interface {
	// 检查配额
	CheckQuota(ctx context.Context, caller string, engineType types.EngineType) error

	// 消费配额
	ConsumeQuota(ctx context.Context, caller string, amount uint64) error

	// 获取剩余配额
	GetRemainingQuota(ctx context.Context, caller string) (uint64, error)

	// 重置配额
	ResetQuota(ctx context.Context, caller string) error

	// 获取配额统计
	GetQuotaStats() *QuotaStats
}

// ThreatDetector 威胁检测器接口
// 由security包实现，供coordinator调用
type ThreatDetector interface {
	// 检测威胁
	DetectThreats(ctx context.Context, params types.ExecutionParams) (*ThreatInfo, error)

	// 更新威胁情报
	UpdateThreatIntelligence(intelligence ThreatIntelligence) error

	// 获取威胁级别
	GetThreatLevel() ThreatLevel

	// 获取威胁统计
	GetThreatStats() *ThreatStats
}

// ==================== 数据结构定义 ====================

// SecurityLevel 安全级别
type SecurityLevel string

const (
	SecurityLevelLow      SecurityLevel = "low"
	SecurityLevelMedium   SecurityLevel = "medium"
	SecurityLevelHigh     SecurityLevel = "high"
	SecurityLevelCritical SecurityLevel = "critical"
)

// SecurityStats 安全统计
type SecurityStats struct {
	TotalChecks      int64            `json:"total_checks"`
	PassedChecks     int64            `json:"passed_checks"`
	FailedChecks     int64            `json:"failed_checks"`
	ViolationsByType map[string]int64 `json:"violations_by_type"`
	LastCheckTime    time.Time        `json:"last_check_time"`
}

// QuotaStats 配额统计
type QuotaStats struct {
	TotalQuotaChecks  int64             `json:"total_quota_checks"`
	QuotaExceeded     int64             `json:"quota_exceeded"`
	TotalConsumed     uint64            `json:"total_consumed"`
	ConsumptionByUser map[string]uint64 `json:"consumption_by_user"`
	LastResetTime     time.Time         `json:"last_reset_time"`
}

// ThreatInfo 威胁信息
type ThreatInfo struct {
	ThreatID    string                 `json:"threat_id"`
	ThreatType  string                 `json:"threat_type"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	Confidence  float64                `json:"confidence"`
	DetectedAt  time.Time              `json:"detected_at"`
	Context     map[string]interface{} `json:"context"`
}

// ThreatIntelligence 威胁情报
type ThreatIntelligence struct {
	Signatures []ThreatSignature `json:"signatures"`
	Patterns   []ThreatPattern   `json:"patterns"`
	Indicators []ThreatIndicator `json:"indicators"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Version    string            `json:"version"`
}

// ThreatSignature 威胁签名
type ThreatSignature struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

// ThreatPattern 威胁模式
type ThreatPattern struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Rules      []string `json:"rules"`
	Confidence float64  `json:"confidence"`
}

// ThreatIndicator 威胁指标
type ThreatIndicator struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	Severity string `json:"severity"`
	Source   string `json:"source"`
}

// ThreatLevel 威胁级别
type ThreatLevel string

const (
	ThreatLevelNone     ThreatLevel = "none"
	ThreatLevelLow      ThreatLevel = "low"
	ThreatLevelMedium   ThreatLevel = "medium"
	ThreatLevelHigh     ThreatLevel = "high"
	ThreatLevelCritical ThreatLevel = "critical"
)

// ThreatStats 威胁统计
type ThreatStats struct {
	TotalDetections      int64            `json:"total_detections"`
	DetectionsByType     map[string]int64 `json:"detections_by_type"`
	DetectionsBySeverity map[string]int64 `json:"detections_by_severity"`
	LastDetectionTime    time.Time        `json:"last_detection_time"`
	CurrentThreatLevel   ThreatLevel      `json:"current_threat_level"`
}
