package host

import (
	libp2p "github.com/libp2p/go-libp2p"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// 私有网络（PSK）支持占位：
//   - 由于当前 NodeOptions/Host 配置中未定义私网（swarm.key）相关字段，
//     这里先返回空选项以保持默认不启用私网。
//   - 如果未来需要启用 PSK，可以在 internal/config/p2p/config.go 的 HostConfig 中
//     扩展例如 PrivateNetwork.Enabled、PrivateNetwork.PSKPath 等字段，
//     并在此处读取路径并解码 swarm.key（v1）后提供 libp2p.PrivateNetwork(psk) 选项。
//   - 之所以保留该文件，是为了不改变装配顺序与调用方结构（builder 仍可附加本选项）。
func withPrivateNetworkOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	// if cfg == nil || cfg.GetP2POptions() == nil || !cfg.GetP2POptions().PrivateNetwork.Enabled {
	//     return nil
	// }
	// pskPath := cfg.GetP2POptions().PrivateNetwork.PSKPath
	// if pskPath == "" {
	//     return nil
	// }
	// b, err := os.ReadFile(pskPath)
	// if err != nil || len(b) == 0 {
	//     return nil
	// }
	// psk, derr := pnet.DecodeV1PSK(bytes.NewReader(b))
	// if derr != nil {
	//     return nil
	// }
	// return []libp2p.Option{libp2p.PrivateNetwork(psk)}
	return nil
}
