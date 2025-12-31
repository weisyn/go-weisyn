package address

import (
	"encoding/hex"
	"testing"

	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

func TestWESAddressGeneration(t *testing.T) {
	addressService := NewAddressService(nil) // 测试不需要私钥功能

	// 测试用例：使用Genesis-Founder的数据
	testPublicKey := "5c09ebc499a5c427660546fb0f155db604f4e2300d897a9fc711a5ce1380eac2cae1dde1df9dfa7542d8ade1da86083cb2161b9f7bbd6d5cf8230d3e300ad664"
	expectedAddress := "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn"

	// 解码公钥
	publicKeyBytes, err := hex.DecodeString(testPublicKey)
	if err != nil {
		t.Fatalf("解码公钥失败: %v", err)
	}

	if len(publicKeyBytes) != 64 {
		t.Fatalf("公钥长度错误: 期望 64 字节, 实际 %d 字节", len(publicKeyBytes))
	}

	// 生成地址
	generatedAddress, err := addressService.PublicKeyToAddress(publicKeyBytes)
	if err != nil {
		t.Fatalf("生成地址失败: %v", err)
	}

	if generatedAddress != expectedAddress {
		t.Errorf("地址不匹配:\n期望: %s\n实际: %s", expectedAddress, generatedAddress)
	}

	t.Logf("✅ 地址生成成功: %s", generatedAddress)
}

func TestWESAddressValidation(t *testing.T) {
	addressService := NewAddressService(nil) // 测试不需要私钥功能

	testCases := []struct {
		address     string
		shouldValid bool
		description string
	}{
		{
			address:     "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
			shouldValid: true,
			description: "有效的地址",
		},
		{
			address:     "CRqzBsipoq6t2qPxUpEgkX51qxkJbBqCWV",
			shouldValid: true,
			description: "有效的地址（Genesis-Investor）",
		},
		{
			address:     "invalid_address_format",
			shouldValid: false,
			description: "无效的地址格式",
		},
		{
			address:     "",
			shouldValid: false,
			description: "空地址",
		},
		{
			address:     "1234567890",
			shouldValid: false,
			description: "太短的地址",
		},
		{
			address:     "0x1234567890abcdef1234567890abcdef12345678",
			shouldValid: false,
			description: "Ethereum风格地址（应该被拒绝）",
		},
	}

	for _, tc := range testCases {
		valid, err := addressService.ValidateAddress(tc.address)
		if tc.shouldValid {
			if !valid || err != nil {
				t.Errorf("%s: 应该有效但验证失败, valid=%v, err=%v", tc.description, valid, err)
			} else {
				t.Logf("✅ %s: 验证通过", tc.description)
			}
		} else {
			if valid {
				t.Errorf("%s: 应该无效但验证通过", tc.description)
			} else {
				t.Logf("✅ %s: 正确拒绝", tc.description)
			}
		}
	}
}

func TestAddressConversion(t *testing.T) {
	addressService := NewAddressService(nil) // 测试不需要私钥功能

	// 测试地址到字节的转换
	testAddress := "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn"
	addressBytes, err := addressService.AddressToBytes(testAddress)
	if err != nil {
		t.Fatalf("地址转字节失败: %v", err)
	}

	if len(addressBytes) != 20 {
		t.Errorf("地址字节长度错误: 期望 20, 实际 %d", len(addressBytes))
	}

	t.Logf("地址字节: %x", addressBytes)

	// 测试字节到地址的转换
	convertedAddress, err := addressService.BytesToAddress(addressBytes)
	if err != nil {
		t.Fatalf("字节转地址失败: %v", err)
	}

	if convertedAddress != testAddress {
		t.Errorf("地址转换不一致:\n原始: %s\n转换: %s", testAddress, convertedAddress)
	}

	t.Logf("✅ 地址双向转换成功: %s ↔ %x", testAddress, addressBytes)
}

// 🔧 已删除TestAddressJSONSerialization测试函数
// 原因：违反protobuf序列化规范，地址应该使用pb/blockchain/block/transaction/transaction.proto中的Address消息

func TestAddressTypeDetection(t *testing.T) {
	addressService := NewAddressService(nil) // 测试不需要私钥功能

	testCases := []struct {
		address      string
		expectedType cryptointf.AddressType
		shouldError  bool
		description  string
	}{
		{
			address:      "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
			expectedType: cryptointf.AddressTypeBitcoin,
			shouldError:  false,
			description:  "标准地址",
		},
		{
			address:      "invalid_format",
			expectedType: cryptointf.AddressTypeInvalid,
			shouldError:  true,
			description:  "无效地址格式",
		},
		{
			address:      "",
			expectedType: cryptointf.AddressTypeInvalid,
			shouldError:  true,
			description:  "空地址",
		},
	}

	for _, tc := range testCases {
		actualType, err := addressService.GetAddressType(tc.address)

		if tc.shouldError {
			if err == nil {
				t.Errorf("%s: 应该返回错误，但没有错误", tc.description)
			} else {
				t.Logf("✅ %s: 正确返回错误", tc.description)
			}
		} else {
			if err != nil {
				t.Errorf("%s: 不应该有错误，但得到: %v", tc.description, err)
			}
			if actualType != tc.expectedType {
				t.Errorf("%s: 类型不匹配，期望 %s, 实际 %s",
					tc.description, tc.expectedType, actualType)
			} else {
				t.Logf("✅ %s: 类型检测正确", tc.description)
			}
		}
	}
}

func TestAddressComparison(t *testing.T) {
	addressService := NewAddressService(nil) // 测试不需要私钥功能

	testCases := []struct {
		addr1       string
		addr2       string
		shouldEqual bool
		description string
	}{
		{
			addr1:       "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
			addr2:       "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
			shouldEqual: true,
			description: "相同地址比较",
		},
		{
			addr1:       "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
			addr2:       "CRqzBsipoq6t2qPxUpEgkX51qxkJbBqCWV",
			shouldEqual: false,
			description: "不同地址比较",
		},
	}

	for _, tc := range testCases {
		equal, err := addressService.CompareAddresses(tc.addr1, tc.addr2)
		if err != nil {
			t.Errorf("%s: 比较失败 %v", tc.description, err)
			continue
		}

		if equal != tc.shouldEqual {
			t.Errorf("%s: 比较结果错误，期望 %v, 实际 %v", tc.description, tc.shouldEqual, equal)
		} else {
			t.Logf("✅ %s: 比较正确", tc.description)
		}
	}
}

func TestZeroAddressDetection(t *testing.T) {
	addressService := NewAddressService(nil) // 测试不需要私钥功能

	// 创建零地址
	zeroBytes := make([]byte, 20)
	zeroAddress, err := addressService.BytesToAddress(zeroBytes)
	if err != nil {
		t.Fatalf("创建零地址失败: %v", err)
	}

	// 测试零地址检测
	if !addressService.IsZeroAddress(zeroAddress) {
		t.Errorf("零地址检测失败: %s 应该被识别为零地址", zeroAddress)
	}

	// 测试非零地址
	nonZeroAddress := "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn"
	if addressService.IsZeroAddress(nonZeroAddress) {
		t.Errorf("非零地址检测失败: %s 不应该被识别为零地址", nonZeroAddress)
	}

	t.Logf("✅ 零地址检测正确: %s", zeroAddress)
}

func TestAddressHexConversion(t *testing.T) {
	addressService := NewAddressService(nil) // 测试不需要私钥功能

	testAddress := "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn"

	// 转换为十六进制
	hexString, err := addressService.AddressToHexString(testAddress)
	if err != nil {
		t.Fatalf("地址转十六进制失败: %v", err)
	}

	if len(hexString) != 40 {
		t.Errorf("十六进制长度错误: 期望 40, 实际 %d", len(hexString))
	}

	t.Logf("地址十六进制: %s", hexString)

	// 从十六进制转回地址
	convertedAddress, err := addressService.HexStringToAddress(hexString)
	if err != nil {
		t.Fatalf("十六进制转地址失败: %v", err)
	}

	if convertedAddress != testAddress {
		t.Errorf("十六进制转换不一致:\n原始: %s\n转换: %s", testAddress, convertedAddress)
	}

	t.Logf("✅ 十六进制转换成功: %s ↔ %s", testAddress, hexString)
}

func TestIsETHStyleAddress(t *testing.T) {
	testCases := []struct {
		address  string
		isETH    bool
		description string
	}{
		{
			address:     "0x1234567890abcdef1234567890abcdef12345678",
			isETH:       true,
			description: "小写 0x 前缀",
		},
		{
			address:     "0X1234567890ABCDEF1234567890ABCDEF12345678",
			isETH:       true,
			description: "大写 0X 前缀",
		},
		{
			address:     "CU27c4fBqvPmLM6N3A4YsYCfpz6RaU8ND8",
			isETH:       false,
			description: "Base58Check 地址",
		},
		{
			address:     "1234567890abcdef",
			isETH:       false,
			description: "纯 hex 字符串（无前缀）",
		},
		{
			address:     "",
			isETH:       false,
			description: "空字符串",
		},
		{
			address:     "0",
			isETH:       false,
			description: "太短（单个字符）",
		},
	}

	for _, tc := range testCases {
		result := IsETHStyleAddress(tc.address)
		if result != tc.isETH {
			t.Errorf("%s: 预期 %v, 实际 %v", tc.description, tc.isETH, result)
		} else {
			t.Logf("✅ %s: 正确识别为 isETH=%v", tc.description, result)
		}
	}
}

func TestStringToAddress_RejectsETHStyle(t *testing.T) {
	addressService := NewAddressService(nil)

	ethAddresses := []string{
		"0x1234567890abcdef1234567890abcdef12345678",
		"0X1234567890ABCDEF1234567890ABCDEF12345678",
		"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
	}

	for _, addr := range ethAddresses {
		_, err := addressService.StringToAddress(addr)
		if err == nil {
			t.Errorf("应该拒绝 ETH 风格地址: %s", addr)
		} else if err != ErrETHAddressNotSupported {
			t.Logf("✅ 正确拒绝 ETH 地址 %s，错误: %v", addr, err)
		} else {
			t.Logf("✅ 正确拒绝 ETH 地址 %s，错误: %v", addr, err)
		}
	}
}
