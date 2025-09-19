package hash

import (
	"bytes"
	"strconv"
	"testing"
)

func TestSHA256(t *testing.T) {
	hashService := NewHashService()

	testCases := []struct {
		name     string
		input    []byte
		expected int // 预期哈希长度
	}{
		{"空数据", []byte{}, 32},
		{"Hello World", []byte("Hello World"), 32},
		{"数字", []byte("12345"), 32},
		{"中文", []byte("你好，世界"), 32},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hashService.SHA256(tc.input)

			if len(result) != tc.expected {
				t.Errorf("SHA256(%s) 长度 = %d, 期望 %d", tc.input, len(result), tc.expected)
			}

			// 确保相同输入产生相同哈希（幂等性）
			result2 := hashService.SHA256(tc.input)
			if !bytes.Equal(result, result2) {
				t.Errorf("SHA256 不具有幂等性")
			}
		})
	}
}

func TestKeccak256(t *testing.T) {
	hashService := NewHashService()

	testCases := []struct {
		name     string
		input    []byte
		expected int // 预期哈希长度
	}{
		{"空数据", []byte{}, 32},
		{"Hello World", []byte("Hello World"), 32},
		{"数字", []byte("12345"), 32},
		{"中文", []byte("你好，世界"), 32},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hashService.Keccak256(tc.input)

			if len(result) != tc.expected {
				t.Errorf("Keccak256(%s) 长度 = %d, 期望 %d", tc.input, len(result), tc.expected)
			}

			// 确保相同输入产生相同哈希（幂等性）
			result2 := hashService.Keccak256(tc.input)
			if !bytes.Equal(result, result2) {
				t.Errorf("Keccak256 不具有幂等性")
			}
		})
	}
}

func TestDoubleSHA256(t *testing.T) {
	hashService := NewHashService()

	testCases := []struct {
		name     string
		input    []byte
		expected int // 预期哈希长度
	}{
		{"空数据", []byte{}, 32},
		{"Hello World", []byte("Hello World"), 32},
		{"数字", []byte("12345"), 32},
		{"中文", []byte("你好，世界"), 32},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hashService.DoubleSHA256(tc.input)

			if len(result) != tc.expected {
				t.Errorf("DoubleSHA256(%s) 长度 = %d, 期望 %d", tc.input, len(result), tc.expected)
			}

			// 确保相同输入产生相同哈希（幂等性）
			result2 := hashService.DoubleSHA256(tc.input)
			if !bytes.Equal(result, result2) {
				t.Errorf("DoubleSHA256 不具有幂等性")
			}

			// 验证DoubleSHA256确实是两次SHA256
			singleHash := hashService.SHA256(tc.input)
			doubleHash := hashService.SHA256(singleHash)
			if !bytes.Equal(doubleHash, result) {
				t.Errorf("DoubleSHA256 不等于两次SHA256")
			}
		})
	}
}

func TestHashCache(t *testing.T) {
	cache := NewHashCache()

	// 测试缓存存取
	key := "testKey"
	value := []byte{1, 2, 3, 4}

	// 设置缓存
	cache.Set(key, value)

	// 获取缓存
	cached, found := cache.Get(key)
	if !found {
		t.Errorf("未能从缓存中找到键 %s", key)
	}

	if !bytes.Equal(cached, value) {
		t.Errorf("缓存值不匹配: 得到 %v, 期望 %v", cached, value)
	}

	// 测试缓存副本（而非引用）
	value[0] = 99
	cached, _ = cache.Get(key)
	if bytes.Equal(cached, value) {
		t.Errorf("缓存没有返回副本，而是返回了引用")
	}
}

func TestConstantTimeCompare(t *testing.T) {
	a := []byte{1, 2, 3, 4}
	b := []byte{1, 2, 3, 4}
	c := []byte{1, 2, 3, 5}
	d := []byte{1, 2, 3}

	// 相同长度、相同内容
	if !ConstantTimeCompare(a, b) {
		t.Errorf("ConstantTimeCompare 应该返回 true，但返回了 false")
	}

	// 相同长度、不同内容
	if ConstantTimeCompare(a, c) {
		t.Errorf("ConstantTimeCompare 应该返回 false，但返回了 true")
	}

	// 不同长度
	if ConstantTimeCompare(a, d) {
		t.Errorf("ConstantTimeCompare 应该返回 false，但返回了 true")
	}
}

// 基准测试

func BenchmarkSHA256(b *testing.B) {
	hashService := NewHashService()
	data := []byte("benchmark data for SHA256 testing with sufficient length to be meaningful")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hashService.SHA256(data)
	}
}

func BenchmarkKeccak256(b *testing.B) {
	hashService := NewHashService()
	data := []byte("benchmark data for Keccak256 testing with sufficient length to be meaningful")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hashService.Keccak256(data)
	}
}

func BenchmarkDoubleSHA256(b *testing.B) {
	hashService := NewHashService()
	data := []byte("benchmark data for DoubleSHA256 testing with sufficient length to be meaningful")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hashService.DoubleSHA256(data)
	}
}

func BenchmarkHashCacheHit(b *testing.B) {
	hashService := NewHashService()
	data := []byte("benchmark data for cache hit testing")

	// 预热缓存
	hashService.SHA256(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hashService.SHA256(data)
	}
}

func BenchmarkHashCacheMiss(b *testing.B) {
	hashService := NewHashService()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 每次使用不同数据，确保缓存未命中
		data := []byte(strconv.Itoa(i) + "benchmark data")
		hashService.SHA256(data)
	}
}
