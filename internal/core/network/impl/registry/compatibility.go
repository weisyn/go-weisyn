package registry

import (
	"fmt"
	"strconv"
	"strings"
)

// VersionComparator 提供版本号比较与兼容性判定（语义版本）
type VersionComparator struct{}

func NewVersionComparator() *VersionComparator { return &VersionComparator{} }

// Compare 比较两个版本（语义版本规则）
// 返回：-1(a<b), 0(a==b), 1(a>b)
func (c *VersionComparator) Compare(a, b string) (int, error) {
	if a == b {
		return 0, nil
	}
	majorA, minorA, patchA, err := c.Parse(a)
	if err != nil {
		return 0, fmt.Errorf("invalid version %s: %v", a, err)
	}
	majorB, minorB, patchB, err := c.Parse(b)
	if err != nil {
		return 0, fmt.Errorf("invalid version %s: %v", b, err)
	}
	// 主版本比较
	if majorA != majorB {
		if majorA < majorB {
			return -1, nil
		}
		return 1, nil
	}
	// 次版本比较
	if minorA != minorB {
		if minorA < minorB {
			return -1, nil
		}
		return 1, nil
	}
	// 补丁版本比较
	if patchA != patchB {
		if patchA < patchB {
			return -1, nil
		}
		return 1, nil
	}
	return 0, nil
}

// IsCompatible 检查版本兼容性（同主版本兼容）
func (c *VersionComparator) IsCompatible(local, remote string) (bool, error) {
	majorL, _, _, err := c.Parse(local)
	if err != nil {
		return false, err
	}
	majorR, _, _, err := c.Parse(remote)
	if err != nil {
		return false, err
	}
	// 同主版本才兼容
	return majorL == majorR, nil
}

// Parse 解析语义版本号（支持 "v1.2.3" 或 "1.2.3"）
func (c *VersionComparator) Parse(v string) (int, int, int, error) {
	// 移除 'v' 前缀
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid version format, expected x.y.z")
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version: %v", err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minor version: %v", err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid patch version: %v", err)
	}
	return major, minor, patch, nil
}
