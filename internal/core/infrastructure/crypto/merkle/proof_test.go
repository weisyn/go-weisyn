package merkle

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func TestGenerateProof(t *testing.T) {
	testCases := []struct {
		name      string
		hashes    [][]byte
		targetIdx int
		expectErr bool
	}{
		{
			name:      "空哈希列表",
			hashes:    [][]byte{},
			targetIdx: 0,
			expectErr: true,
		},
		{
			name:      "单个哈希",
			hashes:    [][]byte{[]byte("singlehash")},
			targetIdx: 0,
			expectErr: false,
		},
		{
			name:      "多个哈希",
			hashes:    [][]byte{[]byte("hash1"), []byte("hash2"), []byte("hash3"), []byte("hash4")},
			targetIdx: 2,
			expectErr: false,
		},
		{
			name:      "索引越界",
			hashes:    [][]byte{[]byte("hash1"), []byte("hash2")},
			targetIdx: 5,
			expectErr: true,
		},
		{
			name:      "负索引",
			hashes:    [][]byte{[]byte("hash1"), []byte("hash2")},
			targetIdx: -1,
			expectErr: true,
		},
		{
			name:      "包含空哈希",
			hashes:    [][]byte{[]byte("hash1"), {}, []byte("hash3")},
			targetIdx: 0,
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			proof, directions, err := GenerateProof(tc.hashes, tc.targetIdx)

			if tc.expectErr {
				if err == nil {
					t.Errorf("期望错误但没有得到错误")
				}
			} else {
				if err != nil {
					t.Errorf("生成证明出错: %v", err)
				}

				// 对于有效情况，验证证明路径和方向长度匹配
				if len(proof) != len(directions) {
					t.Errorf("证明路径和方向长度不匹配: %d vs %d", len(proof), len(directions))
				}

				// 验证证明
				if !tc.expectErr && len(tc.hashes) > 0 {
					root, _ := ComputeRoot(tc.hashes)
					if root != nil {
						valid := VerifyProof(tc.hashes[tc.targetIdx], proof, directions, root)
						if !valid {
							t.Errorf("生成的证明无效")
						}
					}
				}
			}
		})
	}
}

func TestGenerateProofWithOptions(t *testing.T) {
	// 创建测试数据
	hashes := make([][]byte, 16)
	for i := 0; i < 16; i++ {
		hashes[i] = []byte("hash" + strconv.Itoa(i))
	}

	// 测试默认选项
	proof1, _, err := GenerateProofWithOptions(context.Background(), hashes, 5, nil)
	if err != nil {
		t.Errorf("使用默认选项生成证明失败: %v", err)
	}

	// 测试自定义选项
	customOptions := &ProofOptions{
		Parallel:           false,
		ConstantTimeVerify: false,
		Timeout:            10 * time.Second,
	}
	proof2, _, err := GenerateProofWithOptions(context.Background(), hashes, 5, customOptions)
	if err != nil {
		t.Errorf("使用自定义选项生成证明失败: %v", err)
	}

	// 验证两种方式生成的证明应该相同
	if len(proof1) != len(proof2) {
		t.Errorf("默认选项和自定义选项生成的证明长度不同")
	}

	// 测试超时取消
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond) // 确保超时

	_, _, err = GenerateProofWithOptions(ctx, hashes, 5, nil)
	if err == nil {
		t.Errorf("预期上下文超时错误，但没有得到错误")
	}

	// 测试取消
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, _, err = GenerateProofWithOptions(cancelCtx, hashes, 5, nil)
	if err == nil {
		t.Errorf("预期上下文取消错误，但没有得到错误")
	}
}

func TestVerifyProof(t *testing.T) {
	// 创建测试数据
	hashes := [][]byte{
		[]byte("hash1"),
		[]byte("hash2"),
		[]byte("hash3"),
		[]byte("hash4"),
		[]byte("hash5"),
		[]byte("hash6"),
		[]byte("hash7"),
		[]byte("hash8"),
	}

	targetIdx := 3
	targetHash := hashes[targetIdx]

	// 生成证明
	proof, directions, err := GenerateProof(hashes, targetIdx)
	if err != nil {
		t.Fatalf("生成证明失败: %v", err)
	}

	// 计算根哈希
	root, err := ComputeRoot(hashes)
	if err != nil {
		t.Fatalf("计算根哈希失败: %v", err)
	}

	// 验证有效证明
	valid := VerifyProof(targetHash, proof, directions, root)
	if !valid {
		t.Errorf("验证有效证明失败")
	}

	// 验证无效数据
	invalid1 := VerifyProof([]byte("wronghash"), proof, directions, root)
	if invalid1 {
		t.Errorf("验证错误目标哈希应当失败")
	}

	invalid2 := VerifyProof(targetHash, proof, directions, []byte("wrongroot"))
	if invalid2 {
		t.Errorf("验证错误根哈希应当失败")
	}

	// 测试方向与证明长度不匹配
	invalid3 := VerifyProof(targetHash, proof, directions[:len(directions)-1], root)
	if invalid3 {
		t.Errorf("验证不匹配的方向和证明应当失败")
	}
}

func TestVerifyProofWithOptions(t *testing.T) {
	// 创建测试数据
	hashes := [][]byte{
		[]byte("hash1"),
		[]byte("hash2"),
		[]byte("hash3"),
		[]byte("hash4"),
	}

	targetIdx := 2
	targetHash := hashes[targetIdx]

	// 生成证明
	proof, directions, _ := GenerateProof(hashes, targetIdx)
	root, _ := ComputeRoot(hashes)

	// 测试默认选项
	valid1 := VerifyProofWithOptions(context.Background(), targetHash, proof, directions, root, nil)
	if !valid1 {
		t.Errorf("使用默认选项验证证明失败")
	}

	// 测试自定义选项
	customOptions := &ProofOptions{
		ConstantTimeVerify: false,
		Timeout:            10 * time.Second,
	}
	valid2 := VerifyProofWithOptions(context.Background(), targetHash, proof, directions, root, customOptions)
	if !valid2 {
		t.Errorf("使用自定义选项验证证明失败")
	}

	// 测试超时
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond) // 确保超时

	valid3 := VerifyProofWithOptions(ctx, targetHash, proof, directions, root, nil)
	if valid3 {
		t.Errorf("验证应当因上下文超时而失败")
	}
}

func TestProofCache(t *testing.T) {
	cache := NewProofCache()

	// 测试缓存
	proof := [][]byte{[]byte("proof1"), []byte("proof2")}
	directions := []bool{true, false}
	key := "testkey"

	// 设置缓存
	cache.Set(key, proof, directions)

	// 获取缓存
	cachedProof, cachedDirections, found := cache.Get(key)
	if !found {
		t.Errorf("无法从缓存获取证明")
	}

	// 验证缓存内容
	if len(cachedProof) != len(proof) {
		t.Errorf("缓存的证明长度不匹配")
	}

	if len(cachedDirections) != len(directions) {
		t.Errorf("缓存的方向长度不匹配")
	}

	// 验证缓存是副本而非引用
	proof[0][0] = 99
	cachedProof, _, _ = cache.Get(key)
	if cachedProof[0][0] == 99 {
		t.Errorf("缓存没有返回证明的副本，而是返回了引用")
	}
}

// 基准测试
func BenchmarkGenerateProof(b *testing.B) {
	// 创建测试数据
	hashes := make([][]byte, 1000)
	for i := 0; i < 1000; i++ {
		hashes[i] = []byte("hash" + strconv.Itoa(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateProof(hashes, 500)
	}
}

func BenchmarkVerifyProof(b *testing.B) {
	// 创建测试数据
	hashes := make([][]byte, 1000)
	for i := 0; i < 1000; i++ {
		hashes[i] = []byte("hash" + strconv.Itoa(i))
	}

	targetIdx := 500
	targetHash := hashes[targetIdx]
	proof, directions, _ := GenerateProof(hashes, targetIdx)
	root, _ := ComputeRoot(hashes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		VerifyProof(targetHash, proof, directions, root)
	}
}
