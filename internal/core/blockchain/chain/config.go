// Package chain 节点配置查询实现
package chain

import (
	"context"

	"github.com/weisyn/v1/pkg/types"
)

// getNodeMode 获取当前节点模式
func (m *Manager) getNodeMode(ctx context.Context) (types.NodeMode, error) {
	if m.logger != nil {
		m.logger.Debugf("开始查询节点模式")
	}

	// TODO: 实现节点模式查询逻辑
	// 临时实现
	var nodeMode types.NodeMode

	if m.logger != nil {
		m.logger.Debugf("节点模式查询完成 - mode: %v", nodeMode)
	}

	return nodeMode, nil
}
