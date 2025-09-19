package keyspace

import (
	"bytes"
	"math/big"
	"testing"
)

// TestZeroPrefixLength 测试ZeroPrefixLength函数的正确性
func TestZeroPrefixLength(t *testing.T) {
	// 定义测试用例
	cases := [][]byte{
		{0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00}, // 包含24个前缀零位
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // 包含56个前缀零位
		{0x00, 0x58, 0xFF, 0x80, 0x00, 0x00, 0xF0}, // 包含9个前缀零位
	}
	// 定义期望的前缀零位数
	expected := []int{24, 56, 9}

	// 遍历测试用例进行验证
	for i, c := range cases {
		result := ZeroPrefixLength(c)
		if result != expected[i] {
			t.Errorf("ZeroPrefixLength failed: got %v, expected %v", result, expected[i])
		}
	}
}

// TestCommonPrefixLength 测试CommonPrefixLength函数
func TestCommonPrefixLength(t *testing.T) {
	testCases := []struct {
		a, b     []byte
		expected int
	}{
		{
			a:        []byte{0xFF, 0xFF, 0xFF, 0xFF},
			b:        []byte{0xFF, 0xFF, 0xFF, 0xFF},
			expected: 32,
		},
		{
			a:        []byte{0xFF, 0xFF, 0xFF, 0x00},
			b:        []byte{0xFF, 0xFF, 0xFF, 0xFF},
			expected: 24,
		},
		{
			a:        []byte{0xFF, 0x00, 0x00, 0x00},
			b:        []byte{0xFF, 0xFF, 0xFF, 0xFF},
			expected: 8,
		},
		{
			a:        []byte{0x00, 0x00, 0x00, 0x00},
			b:        []byte{0xFF, 0xFF, 0xFF, 0xFF},
			expected: 0,
		},
	}

	for i, tc := range testCases {
		result := CommonPrefixLength(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("Test case %d failed: got %d, expected %d", i, result, tc.expected)
		}
	}
}

// TestXORKeySpace 测试XOR键空间的基本功能
func TestXORKeySpace(t *testing.T) {
	// 测试键的创建
	id1 := []byte("test1")
	id2 := []byte("test2")

	key1 := XORKeySpace.Key(id1)
	key2 := XORKeySpace.Key(id2)

	// 验证键的属性
	if key1.Space != XORKeySpace {
		t.Error("Key space mismatch")
	}

	if !bytes.Equal(key1.Original, id1) {
		t.Error("Original data mismatch")
	}

	// 测试距离计算
	distance := key1.Distance(key2)
	if distance.Cmp(big.NewInt(0)) == 0 && !bytes.Equal(id1, id2) {
		t.Error("Distance should not be zero for different keys")
	}

	// 测试等价性
	key1Copy := XORKeySpace.Key(id1)
	if !key1.Equal(key1Copy) {
		t.Error("Keys should be equal")
	}

	if key1.Equal(key2) && !bytes.Equal(id1, id2) {
		t.Error("Different keys should not be equal")
	}
}

// TestXORDistance 测试XOR距离计算
func TestXORDistance(t *testing.T) {
	a := []byte{0xFF, 0x00, 0xFF, 0x00}
	b := []byte{0x00, 0xFF, 0x00, 0xFF}
	expected := []byte{0xFF, 0xFF, 0xFF, 0xFF}

	result := XORDistance(a, b)
	if !bytes.Equal(result, expected) {
		t.Errorf("XOR distance calculation failed: got %x, expected %x", result, expected)
	}
}

// TestFastCommonPrefixLength 测试快速公共前缀长度计算
func TestFastCommonPrefixLength(t *testing.T) {
	testCases := []struct {
		a, b     []byte
		expected int
	}{
		{
			a:        []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			b:        []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			expected: 64,
		},
		{
			a:        []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00},
			b:        []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			expected: 56,
		},
	}

	for i, tc := range testCases {
		result := FastCommonPrefixLength(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("Test case %d failed: got %d, expected %d", i, result, tc.expected)
		}
	}
}

// BenchmarkCommonPrefixLength 基准测试公共前缀长度计算
func BenchmarkCommonPrefixLength(b *testing.B) {
	a := make([]byte, 32)
	c := make([]byte, 32)

	// 填充测试数据
	for i := 0; i < 32; i++ {
		a[i] = 0xFF
		c[i] = 0xFF
	}
	c[31] = 0x00 // 最后一位不同

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CommonPrefixLength(a, c)
	}
}

// BenchmarkFastCommonPrefixLength 基准测试快速公共前缀长度计算
func BenchmarkFastCommonPrefixLength(b *testing.B) {
	a := make([]byte, 32)
	c := make([]byte, 32)

	// 填充测试数据
	for i := 0; i < 32; i++ {
		a[i] = 0xFF
		c[i] = 0xFF
	}
	c[31] = 0x00 // 最后一位不同

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FastCommonPrefixLength(a, c)
	}
}
