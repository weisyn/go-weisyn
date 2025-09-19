package registry

import (
	"context"
	"fmt"
)

// negotiation.go
// 协议族与版本协商（方法框架）：
// - 基于协议族与本地/远端版本集进行选择
// - 仅挑选双方共同支持的最高版本（具体比较策略见 compatibility.go）

// VersionNegotiator 版本协商引擎（方法框架）
// 说明：封装版本集合收集与最佳匹配选择
type VersionNegotiator struct{}

// NewVersionNegotiator 创建版本协商引擎（方法框架）
func NewVersionNegotiator() *VersionNegotiator { return &VersionNegotiator{} }

// Negotiate 进行协议版本协商（选择最高兼容版本）
func (n *VersionNegotiator) Negotiate(ctx context.Context, protocolFamily string, localVersions, remoteVersions []string) (string, error) {
	if len(localVersions) == 0 || len(remoteVersions) == 0 {
		return "", fmt.Errorf("no versions provided")
	}
	// 创建版本比较器
	comp := NewVersionComparator()
	// 找到双方都支持的版本交集
	var commonVersions []string
	for _, local := range localVersions {
		for _, remote := range remoteVersions {
			if local == remote {
				commonVersions = append(commonVersions, local)
				break
			}
		}
	}
	if len(commonVersions) == 0 {
		return "", fmt.Errorf("no compatible versions found")
	}
	// 选择最高版本
	best := commonVersions[0]
	for _, v := range commonVersions[1:] {
		cmp, err := comp.Compare(v, best)
		if err != nil {
			continue // 跳过无效版本
		}
		if cmp > 0 {
			best = v
		}
	}
	return best, nil
}
