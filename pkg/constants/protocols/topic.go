package protocols

import "fmt"

// Topic 表示一个结构化的 GossipSub 主题定义。
//
// 设计目标：
//   - 将原本的 `"weisyn.consensus.latest_block.v1"` 等裸字符串拆分为结构化字段；
//   - 显式区分 namespace / domain / name / version，避免在业务层到处手工拼接字符串；
//   - 为后续 Facade 类型化 API（PublishTopic/SubscribeTopic）提供基础类型。
//
// 字段语义：
//   - Namespace: 网络命名空间，如 "public-testnet-demo" / "mainnet-public" 等；
//   - Domain:    协议域，如 "consensus" / "blockchain" / "network"；
//   - Name:      主题名，如 "latest_block" / "tx_announce"；
//   - Version:   版本号，如 "v1" / "v2"。
type Topic struct {
	Namespace string
	Domain    string
	Name      string
	Version   string
}

// NewTopic 创建一个不带命名空间的基础 Topic 定义。
//
// 例如：
//   NewTopic("consensus", "latest_block", "v1") → weisyn.consensus.latest_block.v1
func NewTopic(domain, name, version string) Topic {
	return Topic{
		Domain:  domain,
		Name:    name,
		Version: version,
	}
}

// WithNamespace 返回带命名空间的新 Topic 副本（不修改原值）。
//
// 例如：
//   NewTopic("consensus", "latest_block", "v1").WithNamespace("public-testnet-demo")
//   → weisyn.public-testnet-demo.consensus.latest_block.v1
func (t Topic) WithNamespace(namespace string) Topic {
	t.Namespace = namespace
	return t
}

// String 将 Topic 转换为实际使用的 GossipSub 主题字符串。
//
// 命名规则：
//   - 无 Namespace：  "weisyn.{domain}.{name}.{version}"
//   - 有 Namespace：  "weisyn.{namespace}.{domain}.{name}.{version}"
//
// 若 Domain/Name/Version 为空，将返回空字符串，调用方不应使用。
func (t Topic) String() string {
	if t.Domain == "" || t.Name == "" || t.Version == "" {
		return ""
	}

	base := "weisyn"
	if t.Namespace != "" {
		return fmt.Sprintf("%s.%s.%s.%s.%s", base, t.Namespace, t.Domain, t.Name, t.Version)
	}
	return fmt.Sprintf("%s.%s.%s.%s", base, t.Domain, t.Name, t.Version)
}

// 基础 Topic 定义（不含 namespace），与现有字符串常量保持语义对齐。
//
// 注意：
//   - 这些 Topic 仅包含 domain/name/version，不包含 namespace；
//   - 具体实例在 Network Facade 或业务层通过 WithNamespace(namespace) 构造。
var (
	// BaseTopicTransactionAnnounce 对应 TopicTransactionAnnounce（weisyn.blockchain.tx_announce.v1）
	BaseTopicTransactionAnnounce = NewTopic("blockchain", "tx_announce", "v1")

	// BaseTopicConsensusResult 对应 TopicConsensusResult（weisyn.consensus.latest_block.v1）
	BaseTopicConsensusResult = NewTopic("consensus", "latest_block", "v1")
)


