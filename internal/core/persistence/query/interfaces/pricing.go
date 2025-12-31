package interfaces

import (
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// InternalPricingQuery 内部定价查询接口（Phase 2）
// 继承公共接口 persistence.PricingQuery，遵循代码组织规范
type InternalPricingQuery interface {
	persistence.PricingQuery // 嵌入公共接口

	// 当前无额外内部方法，后续如果需要可在此扩展
}


