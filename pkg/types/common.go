// Package types provides common type definitions used across the project.
package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// 注意：以下类型已被移除（未使用）：
// - FileID: 文件标识符类型
// - TaskID: 任务标识符类型
// - UserID: 用户标识符类型
// 如需使用，可从 git 历史中恢复

// Timestamp 时间戳类型（已废弃，请使用 RFC3339Time）
// 统一的时间表示，支持JSON序列化
// 注意：此类型与 RFC3339Time 功能重复，建议统一使用 RFC3339Time
type Timestamp time.Time

// String 实现 fmt.Stringer 接口
func (t Timestamp) String() string {
	return time.Time(t).Format(time.RFC3339)
}

// MarshalJSON 实现JSON序列化
func (t Timestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).Format(time.RFC3339))
}

// UnmarshalJSON 实现JSON反序列化
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	*t = Timestamp(parsed)
	return nil
}

// Size 文件大小类型
// 以字节为单位的文件大小
type Size int64

// String 实现 fmt.Stringer 接口
func (s Size) String() string {
	return fmt.Sprintf("%d", int64(s))
}

// MarshalJSON 实现JSON序列化
func (s Size) MarshalJSON() ([]byte, error) {
	return json.Marshal(int64(s))
}

// UnmarshalJSON 实现JSON反序列化
func (s *Size) UnmarshalJSON(data []byte) error {
	var i int64
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}
	*s = Size(i)
	return nil
}

// Hash 哈希值类型 - 业务层字符串表示（十六进制）
// 用于内容寻址和完整性验证
//
// ⚠️ 注意：此类型与 pb/blockchain/block/transaction/transaction.proto 中的 Hash message 不同：
// - pb定义: message Hash { bytes value = 1; } （二进制格式）
// - 本定义: type Hash string （十六进制字符串格式）
//
// 使用建议：
// - 协议层通信：使用 transaction.Hash（bytes）
// - 业务层表示：使用 types.Hash（string，用于JSON API）
// - 需要转换时，通过适配函数进行类型转换
type Hash string

// String 实现 fmt.Stringer 接口
func (h Hash) String() string {
	return string(h)
}

// MarshalJSON 实现JSON序列化
func (h Hash) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(h))
}

// UnmarshalJSON 实现JSON反序列化
func (h *Hash) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*h = Hash(s)
	return nil
}

// 注意：Percentage 类型已被移除（未使用）
// 如需使用，可从 git 历史中恢复

// ContentHash 内容哈希值
type ContentHash string

// String 实现 fmt.Stringer 接口
func (c ContentHash) String() string {
	return string(c)
}

// RFC3339Time 统一的 RFC3339 时间类型（用于 JSON 序列化一致性）
// 推荐使用此类型替代 Timestamp，名称更明确
type RFC3339Time time.Time

// Time 转换为标准 time.Time（必要的转换方法）
func (t RFC3339Time) Time() time.Time {
	return time.Time(t)
}

// IsZero 检查是否为零值时间（必要的检查方法）
func (t RFC3339Time) IsZero() bool {
	return time.Time(t).IsZero()
}

// String 实现 fmt.Stringer 接口
func (t RFC3339Time) String() string {
	return time.Time(t).Format(time.RFC3339)
}

// MarshalJSON 实现JSON序列化
func (t RFC3339Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).Format(time.RFC3339))
}

// UnmarshalJSON 实现JSON反序列化
func (t *RFC3339Time) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*t = RFC3339Time(time.Time{})
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	*t = RFC3339Time(parsed)
	return nil
}

// Duration 统一的时长 JSON 表达（使用 Go 的 duration 字符串，如 "1h2m3s"）
type Duration time.Duration

// String 实现 fmt.Stringer 接口
func (d Duration) String() string {
	return time.Duration(d).String()
}

// MarshalJSON 实现JSON序列化
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON 实现JSON反序列化
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*d = Duration(0)
		return nil
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

// ==================== 日志级别类型 ====================

// LogLevel 日志级别类型（从 pkg/interfaces/infrastructure/log/level.go 迁移）
type LogLevel string

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
	FatalLevel LogLevel = "fatal"
)
