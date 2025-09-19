package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// FileID 文件标识符类型
// 基于内容哈希的唯一标识符
type FileID string

// String 返回FileID的字符串表示
func (f FileID) String() string {
	return string(f)
}

// IsValid 检查FileID是否有效
func (f FileID) IsValid() bool {
	return len(strings.TrimSpace(string(f))) > 0
}

// MarshalJSON 实现JSON序列化
func (f FileID) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(f))
}

// UnmarshalJSON 实现JSON反序列化
func (f *FileID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*f = FileID(s)
	return nil
}

// TaskID 任务标识符类型
// 用于跟踪上传和下载任务
type TaskID string

// String 返回TaskID的字符串表示
func (t TaskID) String() string {
	return string(t)
}

// IsValid 检查TaskID是否有效
func (t TaskID) IsValid() bool {
	return len(strings.TrimSpace(string(t))) > 0
}

// MarshalJSON 实现JSON序列化
func (t TaskID) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(t))
}

// UnmarshalJSON 实现JSON反序列化
func (t *TaskID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*t = TaskID(s)
	return nil
}

// UserID 用户标识符类型
// 基于公钥哈希的用户标识
type UserID string

// String 返回UserID的字符串表示
func (u UserID) String() string {
	return string(u)
}

// IsValid 检查UserID是否有效
func (u UserID) IsValid() bool {
	return len(strings.TrimSpace(string(u))) > 0
}

// MarshalJSON 实现JSON序列化
func (u UserID) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(u))
}

// UnmarshalJSON 实现JSON反序列化
func (u *UserID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*u = UserID(s)
	return nil
}

// Timestamp 时间戳类型
// 统一的时间表示，支持JSON序列化
type Timestamp time.Time

// Now 返回当前时间的Timestamp
func Now() Timestamp {
	return Timestamp(time.Now())
}

// Time 转换为标准time.Time
func (t Timestamp) Time() time.Time {
	return time.Time(t)
}

// String 返回时间的字符串表示
func (t Timestamp) String() string {
	return time.Time(t).Format(time.RFC3339)
}

// IsZero 检查是否为零值时间
func (t Timestamp) IsZero() bool {
	return time.Time(t).IsZero()
}

// Before 检查是否在指定时间之前
func (t Timestamp) Before(other Timestamp) bool {
	return time.Time(t).Before(time.Time(other))
}

// After 检查是否在指定时间之后
func (t Timestamp) After(other Timestamp) bool {
	return time.Time(t).After(time.Time(other))
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

// String 返回人类可读的大小表示
func (s Size) String() string {
	const unit = 1024
	if s < unit {
		return fmt.Sprintf("%d B", s)
	}
	div, exp := int64(unit), 0
	for n := s / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(s)/float64(div), "KMGTPE"[exp])
}

// Bytes 返回字节数
func (s Size) Bytes() int64 {
	return int64(s)
}

// IsValid 检查大小是否有效
func (s Size) IsValid() bool {
	return s >= 0
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

// ⚠️ Hash 类型存在重复定义问题
//
// 此类型与 pb/blockchain/block/transaction/transaction.proto 中的 Hash message 存在冲突：
// - pb定义: message Hash { bytes value = 1; }
// - 本定义: type Hash string
//
// 建议：
// 1. 对于协议层通信，使用 transaction.Hash
// 2. 对于业务层抽象，继续使用 types.Hash (本定义)
// 3. 需要转换时，通过适配函数进行类型转换
//
// Hash 哈希值类型 - 业务层字符串表示
// 用于内容寻址和完整性验证
type Hash string

// String 返回哈希值的字符串表示
func (h Hash) String() string {
	return string(h)
}

// IsValid 检查哈希值是否有效
func (h Hash) IsValid() bool {
	s := strings.TrimSpace(string(h))
	// 假设使用SHA256，长度应该是64个字符
	return len(s) == 64 && isHexString(s)
}

// isHexString 检查字符串是否为有效的十六进制字符串
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
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

// Percentage 百分比类型
// 用于表示进度、可用性等百分比值 (0-100)
type Percentage float64

// Float64 返回浮点数值
func (p Percentage) Float64() float64 {
	return float64(p)
}

// String 返回百分比的字符串表示
func (p Percentage) String() string {
	return fmt.Sprintf("%.1f%%", float64(p))
}

// IsValid 检查百分比是否在有效范围内
func (p Percentage) IsValid() bool {
	return p >= 0 && p <= 100
}

// MarshalJSON 实现JSON序列化
func (p Percentage) MarshalJSON() ([]byte, error) {
	return json.Marshal(float64(p))
}

// UnmarshalJSON 实现JSON反序列化
func (p *Percentage) UnmarshalJSON(data []byte) error {
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	*p = Percentage(f)
	return nil
}

// ContentHash 内容哈希值
type ContentHash string

// IsValid 验证内容哈希是否有效
func (c ContentHash) IsValid() bool {
	return len(c) == 64 // 假设是SHA256哈希的十六进制表示
}

// String 返回内容哈希的字符串表示
func (c ContentHash) String() string {
	return string(c)
}

// ===== 新增：统一的时间/时长 JSON 类型 =====

// RFC3339Time 统一的 RFC3339 时间类型（用于 JSON 序列化一致性）
type RFC3339Time time.Time

func (t RFC3339Time) Time() time.Time { return time.Time(t) }
func (t RFC3339Time) IsZero() bool    { return time.Time(t).IsZero() }
func (t RFC3339Time) String() string  { return time.Time(t).Format(time.RFC3339) }

func (t RFC3339Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).Format(time.RFC3339))
}

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

func (d Duration) Duration() time.Duration { return time.Duration(d) }
func (d Duration) String() string          { return time.Duration(d).String() }

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

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
