// Package format 提供 WES API 层的标识符格式化工具
//
// 遵循 WES 标识符表示规范 (IDENTIFIER_REPRESENTATION_GUIDE.md)：
// - 哈希类标识：64 位 hex 字符串，不带 0x 前缀，小写
// - 地址类标识：Base58Check 编码字符串，不带 0x 前缀
package format

import (
	"encoding/hex"

	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

// HashToHex 将哈希字节转换为纯 hex 字符串（不带 0x 前缀）
//
// 适用于：TxId, BlockId, StateRoot, ContentHash, MerkleRoot 等哈希类标识
func HashToHex(hash []byte) string {
	if len(hash) == 0 {
		return ""
	}
	return hex.EncodeToString(hash)
}

// AddressToBase58 将地址字节转换为 Base58Check 字符串
//
// 适用于：账户地址、合约地址、调用方地址等地址类标识
func AddressToBase58(addrBytes []byte, addrMgr cryptointf.AddressManager) (string, error) {
	if len(addrBytes) == 0 {
		return "", nil
	}
	return addrMgr.BytesToAddress(addrBytes)
}

// MustAddressToBase58 将地址字节转换为 Base58Check 字符串（错误时返回空字符串）
//
// 用于无法处理错误的场景，如 map 字面量中
func MustAddressToBase58(addrBytes []byte, addrMgr cryptointf.AddressManager) string {
	if addrMgr == nil || len(addrBytes) == 0 {
		return ""
	}
	addr, err := addrMgr.BytesToAddress(addrBytes)
	if err != nil {
		return ""
	}
	return addr
}

