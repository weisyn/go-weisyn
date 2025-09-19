package abi

import (
	"testing"
	"time"

	types "github.com/weisyn/v1/pkg/types"
)

// TestABIManager_RegisterABI 测试ABI注册功能
func TestABIManager_RegisterABI(t *testing.T) {
	manager := NewABIManager(nil)

	// 创建测试ABI
	testABI := &types.ContractABI{
		Version: "1.0.0",
		Functions: []types.ContractFunction{
			{
				Name: "transfer",
				Params: []types.ABIParam{
					{Name: "to", Type: "address"},
					{Name: "amount", Type: "uint256"},
				},
				Returns: []types.ABIParam{{Name: "success", Type: "bool"}},
			},
		},
		Events:    []types.ContractEvent{{Name: "Transfer", Params: []types.ABIParam{{Name: "from", Type: "address"}, {Name: "to", Type: "address"}, {Name: "value", Type: "uint256"}}}},
		UpdatedAt: time.Now(),
	}

	contractID := "0x1234567890abcdef"

	t.Run("成功注册ABI", func(t *testing.T) {
		err := manager.RegisterABI(contractID, testABI)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		stats := manager.GetABIStats()
		if stats.TotalABIs != 1 {
			t.Errorf("Expected 1 ABI, got %d", stats.TotalABIs)
		}
	})

	t.Run("获取注册的ABI", func(t *testing.T) {
		retrievedABI, err := manager.abiStore.GetABI(contractID, testABI.Version)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(retrievedABI.Functions) != len(testABI.Functions) {
			t.Errorf("Expected %d functions, got %d", len(testABI.Functions), len(retrievedABI.Functions))
		}
	})

	t.Run("注册无效ABI", func(t *testing.T) {
		invalidABI := &types.ContractABI{}
		err := manager.RegisterABI("0xabcd", invalidABI)
		if err == nil {
			t.Error("Expected error for invalid ABI")
		}
	})
}

// TestABIManager_EncodeParameters 测试参数编码功能
func TestABIManager_EncodeParameters(t *testing.T) {
	manager := NewABIManager(nil)
	// 注册测试ABI
	testABI := &types.ContractABI{
		Version: "1.0.0",
		Functions: []types.ContractFunction{
			{Name: "transfer", Params: []types.ABIParam{{Name: "to", Type: "address"}, {Name: "amount", Type: "uint256"}}},
		},
		UpdatedAt: time.Now(),
	}
	contractID := "0xabcdef"
	_ = manager.RegisterABI(contractID, testABI)

	t.Run("成功编码参数", func(t *testing.T) {
		args := []interface{}{"0xabcdef1234567890", uint64(1000)}
		encoded, err := manager.EncodeParameters(contractID, "transfer", args)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(encoded) == 0 {
			t.Error("Expected non-empty encoded data")
		}
	})

	t.Run("参数数量不匹配", func(t *testing.T) {
		args := []interface{}{"0xabcdef1234567890"}
		_, err := manager.EncodeParameters(contractID, "transfer", args)
		if err == nil {
			t.Error("Expected error for mismatched argument count")
		}
	})

	t.Run("函数不存在", func(t *testing.T) {
		args := []interface{}{"0xabcdef1234567890", uint64(1000)}
		_, err := manager.EncodeParameters(contractID, "nonexistent", args)
		if err == nil {
			t.Error("Expected error for nonexistent function")
		}
	})
}

// TestABIManager_DecodeResult 测试返回值解码功能
func TestABIManager_DecodeResult(t *testing.T) {
	manager := NewABIManager(nil)
	testABI := &types.ContractABI{Version: "1.0.0", Functions: []types.ContractFunction{{Name: "getBalance", Returns: []types.ABIParam{{Name: "balance", Type: "uint256"}}}}, UpdatedAt: time.Now()}
	contractID := "0x1234"
	_ = manager.RegisterABI(contractID, testABI)

	t.Run("成功解码返回值", func(t *testing.T) {
		data := []byte(`[1000]`)
		result, err := manager.DecodeResult(contractID, "getBalance", data)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(result) != 1 {
			t.Errorf("Expected 1 result, got %d", len(result))
		}
	})

	t.Run("解码无效数据", func(t *testing.T) {
		invalid := []byte(`invalid json`)
		_, err := manager.DecodeResult(contractID, "getBalance", invalid)
		if err == nil {
			t.Error("Expected error for invalid data")
		}
	})
}

// TestVersionComparator 简单兼容性行为测试（通过默认兼容服务）
func TestVersionComparator(t *testing.T) {
	cs := newDefaultCompatibilityService()
	t.Run("兼容性简化判断", func(t *testing.T) {
		// 使用相同主版本号，次版本号更高，符合语义化版本控制
		oldABI := &types.ContractABI{Version: "1.0.0", Functions: []types.ContractFunction{{Name: "f"}}}
		newABI := &types.ContractABI{Version: "1.1.0", Functions: []types.ContractFunction{{Name: "f"}, {Name: "g"}}}
		if !cs.IsCompatible(oldABI, newABI) {
			t.Error("Expected compatible when new ABI has more or equal functions with higher minor version")
		}
	})
}

// TestTypeSystem 测试类型系统
func TestTypeSystem(t *testing.T) {
	ts := NewTypeSystem(nil)
	t.Run("类型验证", func(t *testing.T) {
		if err := ts.ValidateType("uint256", uint64(100)); err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if err := ts.ValidateType("uint256", nil); err == nil {
			t.Error("Expected error for nil value")
		}
	})
	t.Run("类型转换", func(t *testing.T) {
		converted, err := ts.ConvertType("uint256", 100)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if converted != 100 {
			t.Errorf("Expected 100, got %v", converted)
		}
	})
}

// TestABIValidator 测试ABI验证器
func TestABIValidator(t *testing.T) {
	validator := NewABIValidator()
	t.Run("有效ABI验证", func(t *testing.T) {
		valid := &types.ContractABI{Version: "1.0.0", UpdatedAt: time.Now()}
		errs := validator.ValidateABI(valid)
		if len(errs) != 0 {
			t.Errorf("Expected no validation errors, got %d", len(errs))
		}
	})
	t.Run("无效ABI验证", func(t *testing.T) {
		invalid := &types.ContractABI{}
		errs := validator.ValidateABI(invalid)
		if len(errs) == 0 {
			t.Error("Expected validation errors")
		}
	})
}
