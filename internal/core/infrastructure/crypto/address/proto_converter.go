// Package address 提供Proto Address转换功能
package address

import (
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ProtoAddressConverter Proto地址格式转换器
type ProtoAddressConverter struct {
	addressService *AddressService
}

// NewProtoAddressConverter 创建Proto地址转换器
func NewProtoAddressConverter() *ProtoAddressConverter {
	return &ProtoAddressConverter{
		addressService: NewAddressService(nil), // Proto转换器不需要私钥功能
	}
}

// BytesToProtoAddress 将字节数组转换为新的Proto Address格式
func (pac *ProtoAddressConverter) BytesToProtoAddress(addressBytes []byte) (*transaction.Address, error) {
	if len(addressBytes) == 0 {
		return &transaction.Address{}, nil
	}

	if len(addressBytes) != 20 {
		return nil, fmt.Errorf("invalid address length: expected 20 bytes, got %d", len(addressBytes))
	}

	// 使用地址服务生成标准地址字符串
	encodedAddress, err := pac.addressService.BytesToAddress(addressBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to encode address: %w", err)
	}

	// 创建Address结构，使用proto中定义的字段名
	return &transaction.Address{
		RawHash:        addressBytes,
		EncodedAddress: encodedAddress,
		AddressType:    transaction.Address_P2PKH, // 默认为P2PKH
		VersionByte:    uint32(WESP2PKHVersion),
	}, nil
}

// ProtoAddressToBytes 将Proto Address转换为字节数组
func (pac *ProtoAddressConverter) ProtoAddressToBytes(address *transaction.Address) ([]byte, error) {
	if address == nil {
		return nil, fmt.Errorf("address is nil")
	}

	// 验证地址数据
	if len(address.RawHash) != 20 {
		return nil, fmt.Errorf("invalid raw hash length: %d", len(address.RawHash))
	}

	return address.RawHash, nil
}

// StringToProtoAddress 将地址字符串转换为Proto Address
func (pac *ProtoAddressConverter) StringToProtoAddress(addressStr string) (*transaction.Address, error) {
	if addressStr == "" {
		return &transaction.Address{}, nil
	}

	// 验证地址格式
	valid, err := pac.addressService.ValidateAddress(addressStr)
	if err != nil {
		return nil, fmt.Errorf("address validation failed: %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("invalid address format")
	}

	// 获取原始字节
	addressBytes, err := pac.addressService.AddressToBytes(addressStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get address bytes: %w", err)
	}

	// 转换为Proto格式
	return pac.BytesToProtoAddress(addressBytes)
}

// ProtoAddressToString 将Proto Address转换为地址字符串
func (pac *ProtoAddressConverter) ProtoAddressToString(address *transaction.Address) (string, error) {
	if address == nil {
		return "", fmt.Errorf("address is nil")
	}

	// 直接返回编码的地址字符串
	if address.EncodedAddress != "" {
		return address.EncodedAddress, nil
	}

	// 如果没有编码地址，从原始字节生成
	if len(address.RawHash) == 20 {
		return pac.addressService.BytesToAddress(address.RawHash)
	}

	return "", fmt.Errorf("no valid address data")
}

// CreateStandardAddress 创建标准的 Proto Address
func (pac *ProtoAddressConverter) CreateStandardAddress(addressBytes []byte) (*transaction.Address, error) {
	return pac.BytesToProtoAddress(addressBytes)
}

// IsZeroAddress 检查是否为零地址
func (pac *ProtoAddressConverter) IsZeroAddress(address *transaction.Address) bool {
	addressBytes, err := pac.ProtoAddressToBytes(address)
	if err != nil {
		return false
	}

	for _, b := range addressBytes {
		if b != 0 {
			return false
		}
	}

	return true
}

// ValidateProtoAddress 验证Proto Address的有效性
func (pac *ProtoAddressConverter) ValidateProtoAddress(address *transaction.Address) error {
	if address == nil {
		return fmt.Errorf("address is nil")
	}

	addressStr, err := pac.ProtoAddressToString(address)
	if err != nil {
		return err
	}

	valid, err := pac.addressService.ValidateAddress(addressStr)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("invalid address")
	}

	return nil
}

// 全局默认转换器实例
var DefaultProtoConverter = NewProtoAddressConverter()

// 便捷函数

// BytesToAddress 便捷函数：字节数组 → Proto Address
func BytesToAddress(addressBytes []byte) (*transaction.Address, error) {
	return DefaultProtoConverter.BytesToProtoAddress(addressBytes)
}

// AddressToBytes 便捷函数：Proto Address → 字节数组
func AddressToBytes(address *transaction.Address) ([]byte, error) {
	return DefaultProtoConverter.ProtoAddressToBytes(address)
}

// StringToAddress 便捷函数：字符串 → Proto Address
func StringToAddress(addressStr string) (*transaction.Address, error) {
	return DefaultProtoConverter.StringToProtoAddress(addressStr)
}

// AddressToString 便捷函数：Proto Address → 字符串
func AddressToString(address *transaction.Address) (string, error) {
	return DefaultProtoConverter.ProtoAddressToString(address)
}
