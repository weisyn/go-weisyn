package interfaces

import (
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// InternalAccountQuery 内部账户查询接口
// 继承公共接口 persistence.AccountQuery，遵循代码组织规范
type InternalAccountQuery interface {
	persistence.AccountQuery // 嵌入公共接口

	// 内部专用方法（如需要可在此添加）
	// 目前仅继承公共接口，无额外内部方法
}

