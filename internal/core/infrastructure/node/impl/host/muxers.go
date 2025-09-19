package host

import (
	libp2p "github.com/libp2p/go-libp2p"
	lpyamux "github.com/libp2p/go-libp2p/p2p/muxer/yamux"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// withMuxerOptions 根据配置构建多路复用器选项
// 修复了TROUBLESHOOTING.md中提到的backlog配置问题，确保所有参数都是正值
func withMuxerOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	// 零配置或Yamux未启用时，使用默认配置
	if cfg == nil || !cfg.Host.Muxer.EnableYamux {
		return []libp2p.Option{libp2p.DefaultMuxers}
	}

	mc := cfg.Host.Muxer

	// 使用yamux默认配置作为基础，避免零值导致的backlog错误
	config := *lpyamux.DefaultTransport.Config()

	// 仅在配置值为正数时覆盖默认值，确保参数有效性
	if ws := mc.YamuxWindowSize; ws > 0 {
		// 统一单位：配置中的 window size 以 KB 为单位；此处换算为字节
		windowSize := uint32(ws) * 1024
		if windowSize < 256*1024 {
			windowSize = 256 * 1024 // 最小256KB
		} else if windowSize > 32*1024*1024 {
			windowSize = 32 * 1024 * 1024 // 最大32MB
		}
		config.MaxStreamWindowSize = windowSize
	}

	if ms := mc.YamuxMaxStreams; ms > 0 {
		// 限制最大入站流数量，确保在合理范围内
		maxStreams := uint32(ms)
		if maxStreams < 1 {
			maxStreams = 1 // 至少允许1个流
		} else if maxStreams > 1000000 {
			maxStreams = 1000000 // 最大100万流（防止资源耗尽）
		}
		config.MaxIncomingStreams = maxStreams
	}

	if to := mc.YamuxConnectionTimeout; to > 0 {
		config.ConnectionWriteTimeout = to
	}

	// Transport是yamux.Config的别名，直接创建
	transport := (*lpyamux.Transport)(&config)

	return []libp2p.Option{libp2p.Muxer(lpyamux.ID, transport)}
}
