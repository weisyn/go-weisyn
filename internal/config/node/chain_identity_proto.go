// Package node 提供 ChainIdentity 与 protobuf 的转换函数
package node

import (
	"github.com/weisyn/v1/pkg/types"
	syncpb "github.com/weisyn/v1/pb/network/protocol"
)

// ToProtoChainIdentity 将 ChainIdentity 转换为 protobuf 格式
func ToProtoChainIdentity(identity types.ChainIdentity) *syncpb.ChainIdentity {
	return &syncpb.ChainIdentity{
		ChainId:          identity.ChainID,
		NetworkNamespace: identity.NetworkNamespace,
		NetworkId:        identity.NetworkID,
		ChainMode:        string(identity.ChainMode),
		GenesisHash:      identity.GenesisHash,
		VersionTag:       identity.VersionTag,
	}
}

// FromProtoChainIdentity 从 protobuf 格式转换为 ChainIdentity
func FromProtoChainIdentity(proto *syncpb.ChainIdentity) types.ChainIdentity {
	if proto == nil {
		return types.ChainIdentity{}
	}
	return types.ChainIdentity{
		ChainID:          proto.ChainId,
		NetworkNamespace: proto.NetworkNamespace,
		NetworkID:        proto.NetworkId,
		ChainMode:        types.ChainMode(proto.ChainMode),
		GenesisHash:      proto.GenesisHash,
		VersionTag:       proto.VersionTag,
	}
}

