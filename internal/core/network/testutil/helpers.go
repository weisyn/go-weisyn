// Package testutil 提供 network 模块测试的辅助函数
package testutil

import (
	"time"

	libhost "github.com/libp2p/go-libp2p/core/host"
	networkconfig "github.com/weisyn/v1/internal/config/network"
	"github.com/weisyn/v1/internal/core/network/facade"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// NewTestFacade 创建测试用的 Facade 实例
func NewTestFacade(
	host libhost.Host,
	logger logiface.Logger,
	cfg *networkconfig.Config,
	hashMgr crypto.HashManager,
	sigMgr crypto.SignatureManager,
) *facade.Facade {
	return facade.NewFacade(host, logger, cfg, hashMgr, sigMgr)
}

// NewTestNetworkConfig 创建测试用的网络配置
func NewTestNetworkConfig() *networkconfig.Config {
	opts := &networkconfig.NetworkOptions{
		MaxMessageSize:           1024 * 1024, // 1MB
		MessageTimeout:           30 * time.Second,
		DeduplicationCacheTTL:    5 * time.Minute,
		RetryAttempts:            3,
		RetryBackoffBase:         100 * time.Millisecond,
		RetryBackoffMax:          5 * time.Second,
		ConnectTimeout:           10 * time.Second,
		WriteTimeout:             5 * time.Second,
		ReadTimeout:              5 * time.Second,
		MaxConnections:           1000,
		MaxConnectionsPerIP:       50,
		MaxMessagesPerWindow:      100,
		MessageRateLimitWindow:    1 * time.Minute,
	}
	return networkconfig.New(opts)
}

