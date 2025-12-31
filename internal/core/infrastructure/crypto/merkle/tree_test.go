package merkle

import (
	"bytes"
	"strconv"
	"testing"
)

func TestComputeRoot(t *testing.T) {
	testCases := []struct {
		name      string
		hashes    [][]byte
		expectErr bool
	}{
		{
			name:      "空哈希列表",
			hashes:    [][]byte{},
			expectErr: true,
		},
		{
			name:      "单个哈希",
			hashes:    [][]byte{[]byte("singlehash")},
			expectErr: false,
		},
		{
			name:      "两个哈希",
			hashes:    [][]byte{[]byte("hash1"), []byte("hash2")},
			expectErr: false,
		},
		{
			name:      "奇数个哈希",
			hashes:    [][]byte{[]byte("hash1"), []byte("hash2"), []byte("hash3")},
			expectErr: false,
		},
		{
			name:      "包含空哈希",
			hashes:    [][]byte{[]byte("hash1"), {}, []byte("hash3")},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := ComputeRoot(tc.hashes)

			if tc.expectErr {
				if err == nil {
					t.Errorf("期望错误但没有得到错误")
				}
			} else {
				if err != nil {
					t.Errorf("计算根哈希出错: %v", err)
				}

				if len(root) == 0 {
					t.Errorf("计算得到的根哈希为空")
				}

				// 重复计算，验证幂等性
				root2, _ := ComputeRoot(tc.hashes)
				if !bytes.Equal(root, root2) {
					t.Errorf("计算根哈希结果不一致")
				}
			}
		})
	}
}

func TestTreeCache(t *testing.T) {
	cache := NewTreeCache()

	// 测试节点缓存
	testNode := &Node{
		Hash: []byte("testhash"),
	}

	cacheKey := "testkey"
	cache.SetNode(cacheKey, testNode)

	cachedNode, found := cache.GetNode(cacheKey)
	if !found {
		t.Errorf("无法从缓存获取节点")
	}

	if !bytes.Equal(cachedNode.Hash, testNode.Hash) {
		t.Errorf("缓存节点哈希与原始节点不匹配")
	}

	// 测试根哈希缓存
	rootHash := []byte("roothash")
	rootKey := "rootkey"

	cache.SetRoot(rootKey, rootHash)

	cachedRoot, found := cache.GetRoot(rootKey)
	if !found {
		t.Errorf("无法从缓存获取根哈希")
	}

	if !bytes.Equal(cachedRoot, rootHash) {
		t.Errorf("缓存的根哈希与原始根哈希不匹配")
	}

	// 测试返回的是副本而非引用
	rootHash[0] = 99
	cachedRoot, _ = cache.GetRoot(rootKey)
	if bytes.Equal(cachedRoot, rootHash) {
		t.Errorf("缓存没有返回根哈希的副本，而是返回了引用")
	}
}

func TestCombineHashes(t *testing.T) {
	left := []byte("lefthash")
	right := []byte("righthash")

	combined := combineHashes(left, right)

	if len(combined) == 0 {
		t.Errorf("组合哈希结果为空")
	}

	// 验证结果的一致性
	combined2 := combineHashes(left, right)
	if !bytes.Equal(combined, combined2) {
		t.Errorf("组合哈希结果不一致")
	}

	// 验证顺序敏感性
	reverseCombined := combineHashes(right, left)
	if bytes.Equal(combined, reverseCombined) {
		t.Errorf("组合哈希应当对输入顺序敏感")
	}
}

// 基准测试
func BenchmarkComputeRoot(b *testing.B) {
	// 创建1000个哈希
	hashes := make([][]byte, 1000)
	for i := 0; i < 1000; i++ {
		hashes[i] = []byte("hash" + strconv.Itoa(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeRoot(hashes)
	}
}

func BenchmarkCombineHashes(b *testing.B) {
	left := []byte("lefthash")
	right := []byte("righthash")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		combineHashes(left, right)
	}
}
