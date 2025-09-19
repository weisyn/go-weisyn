package discovery

import (
	offroute "github.com/ipfs/boxo/routing/offline"
	"github.com/ipfs/go-datastore"
	record "github.com/libp2p/go-libp2p-record"
	routing "github.com/libp2p/go-libp2p/core/routing"
)

// NewOfflineRouter 创建一个简易的离线路由器适配器。
// 说明：
// - 使用内存 Datastore；
// - Validator 允许为 nil；
// - 不进行任何网络交互，适合在 DHT 初始化失败时兜底。
func NewOfflineRouter(validator record.Validator) routing.Routing {
	return offroute.NewOfflineRouter(datastore.NewMapDatastore(), validator)
}
