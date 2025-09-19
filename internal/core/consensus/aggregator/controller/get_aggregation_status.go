// get_aggregation_status.go
// 获取聚合状态的业务逻辑实现
//
// 核心业务功能：
// 1. 查询当前聚合器运行状态
// 2. 提供状态的基本信息
// 3. 判断服务健康状况
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package controller

import (
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// aggregationStatusProvider 聚合状态提供器
type aggregationStatusProvider struct {
	logger       log.Logger
	stateManager interfaces.AggregatorStateManager
}

// newAggregationStatusProvider 创建聚合状态提供器
func newAggregationStatusProvider(logger log.Logger, stateManager interfaces.AggregatorStateManager) *aggregationStatusProvider {
	return &aggregationStatusProvider{
		logger:       logger,
		stateManager: stateManager,
	}
}
