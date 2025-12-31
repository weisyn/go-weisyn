package runtime

import (
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
)

// ApplyCgroupMemoryLimit 自动读取 cgroup 内存上限，并设置 Go 运行时的内存上限（debug.SetMemoryLimit）。
//
// 目标：
// - 在容器环境（Docker/K8s）中避免“Go 堆无限增长 -> RSS 顶到 cgroup limit -> 被 OOM killer 直接杀死”。
// - 通过提前收缩，让 GC 更积极、留出 native/页缓存/网络栈等空间。
//
// 规则：
// - 如果用户显式设置了 GOMEMLIMIT，则尊重用户，不做自动设置。
// - reserveRatio 建议 0.7~0.85（默认 0.8）。
//
// 返回：
// - applied: 是否成功设置
// - limitBytes: 检测到的 cgroup limit（未检测到时为 0）
func ApplyCgroupMemoryLimit(reserveRatio float64) (applied bool, limitBytes uint64, err error) {
	if os.Getenv("GOMEMLIMIT") != "" {
		return false, 0, nil
	}
	if reserveRatio <= 0 || reserveRatio >= 1 {
		reserveRatio = 0.8
	}

	limit, ok, readErr := readCgroupMemoryLimitBytes()
	if readErr != nil {
		return false, 0, readErr
	}
	if !ok || limit == 0 {
		return false, 0, nil
	}

	target := int64(float64(limit) * reserveRatio)
	if target <= 0 {
		return false, limit, nil
	}

	debug.SetMemoryLimit(target)
	return true, limit, nil
}

// GetCgroupMemoryLimitBytes 返回 cgroup 内存上限（bytes）。
// ok=false 表示未检测到（或为 unlimited）。
func GetCgroupMemoryLimitBytes() (limit uint64, ok bool, err error) {
	return readCgroupMemoryLimitBytes()
}

func readCgroupMemoryLimitBytes() (limit uint64, ok bool, err error) {
	// cgroup v2
	if b, e := os.ReadFile("/sys/fs/cgroup/memory.max"); e == nil {
		s := strings.TrimSpace(string(b))
		if s == "" || s == "max" {
			return 0, false, nil
		}
		v, perr := strconv.ParseUint(s, 10, 64)
		if perr != nil {
			return 0, false, fmt.Errorf("parse cgroup v2 memory.max failed: %w", perr)
		}
		// 某些环境会用超大值表示“无限制”
		if v > (1 << 60) {
			return 0, false, nil
		}
		return v, true, nil
	}

	// cgroup v1（Docker 旧版本）
	if b, e := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); e == nil {
		s := strings.TrimSpace(string(b))
		if s == "" {
			return 0, false, nil
		}
		v, perr := strconv.ParseUint(s, 10, 64)
		if perr != nil {
			return 0, false, fmt.Errorf("parse cgroup v1 memory.limit_in_bytes failed: %w", perr)
		}
		if v > (1 << 60) {
			return 0, false, nil
		}
		return v, true, nil
	}

	return 0, false, nil
}


