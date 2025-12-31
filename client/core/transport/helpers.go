package transport

import (
	"strconv"
	"strings"
	"time"
)

// parseUint64FromMap 从 map 中解析 uint64 字段（支持字符串和数字）
func parseUint64FromMap(m map[string]interface{}, key string) (uint64, bool) {
	val, ok := m[key]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case string:
		// 移除 0x 前缀（如果有）
		valStr := strings.TrimPrefix(v, "0x")
		parsed, err := strconv.ParseUint(valStr, 10, 64)
		if err != nil {
			// 如果十进制解析失败，尝试十六进制
			parsed, err = strconv.ParseUint(valStr, 16, 64)
			if err != nil {
				return 0, false
			}
		}
		return parsed, true
	case float64:
		return uint64(v), true
	case uint64:
		return v, true
	case int64:
		return uint64(v), true
	default:
		return 0, false
	}
}

// parseTimeFromMap 从 map 中解析 time.Time 字段（支持多种格式）
func parseTimeFromMap(m map[string]interface{}, key string) (time.Time, bool) {
	val, ok := m[key]
	if !ok {
		return time.Time{}, false
	}
	switch v := val.(type) {
	case string:
		// 尝试解析为 RFC3339 格式
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t, true
		}
		// 如果不是 RFC3339，尝试解析为 Unix 时间戳字符串
		if tsInt, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Unix(tsInt, 0), true
		}
	case float64:
		// Unix 时间戳（秒）
		return time.Unix(int64(v), 0), true
	case int64:
		return time.Unix(v, 0), true
	}
	return time.Time{}, false
}

