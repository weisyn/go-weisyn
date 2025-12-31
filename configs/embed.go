package configs

import (
	_ "embed"

	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
// 内嵌链配置（工程打包手段：go:embed）
//
// 说明：
// - “内嵌”是把已有 JSON 文件打包进二进制的技术手段，避免二进制与 JSON 分发割裂；
// - 不是引入新的“内嵌配置文件”语义，更不是在 chains 目录增加一份新的配置。
// ============================================================================

// 公有链默认内嵌配置：公共测试网 demo（test-public-demo）
//
//go:embed chains/test-public-demo.json
var PublicChainConfig []byte

// dev 公链本地配置（示例/开发用）
//
//go:embed chains/dev-public-local.json
var DevPublicLocalChainConfig []byte

// dev 私链本地配置（示例/开发用）
//
//go:embed chains/dev-private-local.json
var DevPrivateLocalChainConfig []byte

// test 联盟链 demo 配置（示例/演示用）
//
//go:embed chains/test-consortium-demo.json
var TestConsortiumDemoChainConfig []byte

//go:embed templates/consortium-chain.json
var ConsortiumChainTemplate []byte

//go:embed templates/private-chain.json
var PrivateChainTemplate []byte

// 官方主网链级身份常量（只读）
// 注意：开源仓库不再内嵌生产主网配置，这些常量仅用于文档和架构说明
// 实际内嵌的 public 默认配置为 test-public-demo（公共测试网 demo）
const (
	OfficialPublicChainID          uint64 = 1
	OfficialPublicNetworkID               = "WES_mainnet_2025"
	OfficialPublicNetworkNamespace        = "mainnet-public"
	OfficialPublicChainMode               = "public"
)

// 内嵌测试网链级身份常量（用于 --chain public 默认配置）
const (
	EmbeddedTestnetChainID          uint64 = 12001
	EmbeddedTestnetNetworkID               = "WES_public_testnet_demo_2025"
	EmbeddedTestnetNetworkNamespace        = "public-testnet-demo"
	EmbeddedTestnetChainMode               = "public"
)

// GetPublicChainConfig 获取公链配置（内嵌）
// 注意：开源仓库内嵌的是公共测试网 demo（test-public-demo），而非生产主网配置
// 生产主网配置需通过 BaaS 或运维工具单独下发
func GetPublicChainConfig() []byte {
	return PublicChainConfig
}

// GetDevPublicLocalChainConfig 获取 dev 公链本地配置（内嵌）
func GetDevPublicLocalChainConfig() []byte {
	return DevPublicLocalChainConfig
}

// GetDevPrivateLocalChainConfig 获取 dev 私链本地配置（内嵌）
func GetDevPrivateLocalChainConfig() []byte {
	return DevPrivateLocalChainConfig
}

// GetTestConsortiumDemoChainConfig 获取 test 联盟链 demo 配置（内嵌）
func GetTestConsortiumDemoChainConfig() []byte {
	return TestConsortiumDemoChainConfig
}

// GetConsortiumChainTemplate 获取联盟链模板（内嵌）
func GetConsortiumChainTemplate() []byte {
	return ConsortiumChainTemplate
}

// GetPrivateChainTemplate 获取私链模板（内嵌）
func GetPrivateChainTemplate() []byte {
	return PrivateChainTemplate
}

// IsOfficialPublicChainConfig 检查 AppConfig 是否匹配官方主网链级身份
//
// 注意：开源仓库不再内嵌生产主网配置，此函数主要用于：
// - 架构文档和设计说明中的概念引用
// - 未来 BaaS/运维工具生成生产主网配置时的校验
// - 防止误将测试网配置当作生产主网使用
//
// 当前内嵌的配置为测试网（test-public-demo），不会匹配此函数
func IsOfficialPublicChainConfig(appConfig *types.AppConfig) (bool, string) {
	if appConfig == nil {
		return false, "AppConfig 为空"
	}

	if appConfig.Network == nil {
		return false, "network 配置为空"
	}

	// 检查 chain_mode
	if appConfig.Network.ChainMode == nil {
		return false, "network.chain_mode 为空"
	}
	if mode := *appConfig.Network.ChainMode; mode != OfficialPublicChainMode {
		return false, "network.chain_mode 与官方主网不一致"
	}

	// 检查 chain_id
	if appConfig.Network.ChainID == nil {
		return false, "network.chain_id 为空"
	}
	if *appConfig.Network.ChainID != OfficialPublicChainID {
		return false, "network.chain_id 与官方主网不一致"
	}

	// 检查 network_id
	if appConfig.Network.NetworkID == nil {
		return false, "network.network_id 为空"
	}
	if *appConfig.Network.NetworkID != OfficialPublicNetworkID {
		return false, "network.network_id 与官方主网不一致"
	}

	// 检查 network_namespace
	if appConfig.Network.NetworkNamespace == nil {
		return false, "network.network_namespace 为空"
	}
	if *appConfig.Network.NetworkNamespace != OfficialPublicNetworkNamespace {
		return false, "network.network_namespace 与官方主网不一致"
	}

	return true, ""
}
