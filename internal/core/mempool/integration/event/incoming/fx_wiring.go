// 文件说明：
// 本文件用于装配内存池组件的“入站事件”集成（incoming）。
// 职责：在组件启动时订阅外部系统事件（如区块处理完成、链重组等），
// 并在收到事件后回调 TxPool/CandidatePool 等接口以更新本地状态。
// 注意：此处仅提供装配骨架，具体订阅与处理逻辑按需扩展。
package incoming

import (
	"context"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"go.uber.org/fx"
)

// params 定义 Fx 注入参数。
// 字段说明：
// - EventBus：事件总线接口，可选；
// - Logger：日志接口，可选；
// - Lifecycle：应用生命周期，用于注册启动/停止钩子。
type params struct {
	fx.In
	EventBus  event.EventBus `optional:"true"`
	Logger    log.Logger     `optional:"true"`
	Lifecycle fx.Lifecycle
}

// SetupEventIncoming 装配入站事件订阅。
// 参数：
// - p：包含事件总线、日志与生命周期的 Fx 注入参数。
// 返回：
// - error：装配失败时返回错误；当前实现始终返回 nil。
// 逻辑：
// - 在启动阶段注册事件订阅；在停止阶段进行反注册或清理（后续按需实现）。
func SetupEventIncoming(p params) error {
	// 在此处订阅区块链/共识等事件，并回调到 mempool 的接口
	// 先保留空实现，后续逐步补充
	p.Lifecycle.Append(fx.Hook{OnStart: func(ctx context.Context) error { return nil }, OnStop: func(ctx context.Context) error { return nil }})
	return nil
}
