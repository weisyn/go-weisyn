// Package types provides cryptographic type definitions.
package types

import (
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// Address 地址类型别名（指向protobuf生成的Address类型）
type Address = transaction.Address

// AddressType 地址类型枚举（从 pkg/interfaces/infrastructure/crypto/address.go 迁移）
type AddressType int

const (
	AddressTypeBitcoin AddressType = iota
	AddressTypeInvalid
)

func (t AddressType) String() string {
	switch t {
	case AddressTypeBitcoin:
		return "bitcoin_style"
	default:
		return "invalid"
	}
}

// ⚠️ SignatureHashType 类型映射说明
//
// 此类型与 pb/blockchain/block/transaction/transaction.proto 中的 SignatureHashType 枚举重复。
// 为了保持向后兼容性和类型转换便利，这里提供业务层的类型定义。
//
// 🎯 设计目标：
// - 在pkg/types层提供uint32类型，便于业务层计算和转换
// - 在pb层提供标准protobuf枚举，用于网络传输和持久化
// - 两者之间可以进行安全的类型转换：transaction.SignatureHashType(types_value)
//
// 📋 使用建议：
// - 业务逻辑层使用 types.SignatureHashType (本定义)
// - 网络传输层使用 transaction.SignatureHashType (pb定义)
// - 需要转换时：transaction.SignatureHashType(types_value)

type SignatureHashType uint32

const (
	SigHashAll                SignatureHashType = 0x01
	SigHashNone               SignatureHashType = 0x02
	SigHashSingle             SignatureHashType = 0x03
	SigHashAnyoneCanPay       SignatureHashType = 0x80
	SigHashAllAnyoneCanPay    SignatureHashType = 0x81
	SigHashNoneAnyoneCanPay   SignatureHashType = 0x82
	SigHashSingleAnyoneCanPay SignatureHashType = 0x83
)

// ToProtobuf 转换为protobuf枚举类型
func (s SignatureHashType) ToProtobuf() transaction.SignatureHashType {
	return transaction.SignatureHashType(s)
}

// FromProtobuf 从protobuf枚举类型转换
func SignatureHashTypeFromProtobuf(pb transaction.SignatureHashType) SignatureHashType {
	return SignatureHashType(pb)
}
