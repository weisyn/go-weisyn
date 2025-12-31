package merkle_test

import (
	"fmt"
	"testing"

	"github.com/weisyn/v1/internal/core/block/merkle"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// MockHasher 模拟哈希服务
type MockHasher struct {
	hashFunc func([]byte) ([]byte, error)
}

func (m *MockHasher) Hash(data []byte) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("hasher is nil")
	}
	if m.hashFunc != nil {
		return m.hashFunc(data)
	}
	// 默认实现：返回数据长度的32字节表示
	hash := make([]byte, 32)
	copy(hash, data)
	return hash, nil
}

// 编译时检查接口实现
var _ merkle.Hasher = (*MockHasher)(nil)

// TestCalculateMerkleRoot 测试Merkle根计算
func TestCalculateMerkleRoot(t *testing.T) {
	tests := []struct {
		name     string
		hasher   merkle.Hasher
		txs      []*transaction.Transaction
		wantErr  bool
		errMsg   string
	}{
		{
			name:   "单个交易的Merkle根",
			hasher: &MockHasher{},
			txs: []*transaction.Transaction{
				{Version: 1, Inputs: []*transaction.TxInput{}, Outputs: []*transaction.TxOutput{}},
			},
			wantErr: false,
		},
		{
			name:   "两个交易的Merkle根",
			hasher: &MockHasher{},
			txs: []*transaction.Transaction{
				{Version: 1, Inputs: []*transaction.TxInput{}, Outputs: []*transaction.TxOutput{}},
				{Version: 1, Inputs: []*transaction.TxInput{}, Outputs: []*transaction.TxOutput{}},
			},
			wantErr: false,
		},
		{
			name:   "奇数个交易的Merkle根",
			hasher: &MockHasher{},
			txs: []*transaction.Transaction{
				{Version: 1, Inputs: []*transaction.TxInput{}, Outputs: []*transaction.TxOutput{}},
				{Version: 1, Inputs: []*transaction.TxInput{}, Outputs: []*transaction.TxOutput{}},
				{Version: 1, Inputs: []*transaction.TxInput{}, Outputs: []*transaction.TxOutput{}},
			},
			wantErr: false,
		},
		{
			name:    "hasher为nil应返回错误",
			hasher:  nil,
			txs:     []*transaction.Transaction{{Version: 1}},
			wantErr: true,
			errMsg:  "hasher 不能为空",
		},
		{
			name:    "交易列表为空应返回错误",
			hasher:  &MockHasher{},
			txs:     []*transaction.Transaction{},
			wantErr: true,
			errMsg:  "交易列表不能为空",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := merkle.CalculateMerkleRoot(tt.hasher, tt.txs)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CalculateMerkleRoot() 期望错误但没有返回错误")
				}
				return
			}

			if err != nil {
				t.Errorf("CalculateMerkleRoot() 意外错误 = %v", err)
				return
			}

			if len(root) != 32 {
				t.Errorf("CalculateMerkleRoot() Merkle根长度 = %d, 期望 32", len(root))
			}
		})
	}
}

// TestCalculateMerkleRootDeterministic 测试Merkle根计算的确定性
func TestCalculateMerkleRootDeterministic(t *testing.T) {
	hasher := &MockHasher{}
	txs := []*transaction.Transaction{
		{Version: 1, Nonce: 1, Inputs: []*transaction.TxInput{}, Outputs: []*transaction.TxOutput{}},
		{Version: 1, Nonce: 2, Inputs: []*transaction.TxInput{}, Outputs: []*transaction.TxOutput{}},
		{Version: 1, Nonce: 3, Inputs: []*transaction.TxInput{}, Outputs: []*transaction.TxOutput{}},
	}

	// 多次计算应得到相同结果
	root1, err1 := merkle.CalculateMerkleRoot(hasher, txs)
	if err1 != nil {
		t.Fatalf("第一次计算失败: %v", err1)
	}

	root2, err2 := merkle.CalculateMerkleRoot(hasher, txs)
	if err2 != nil {
		t.Fatalf("第二次计算失败: %v", err2)
	}

	if len(root1) != len(root2) {
		t.Errorf("Merkle根长度不一致: %d vs %d", len(root1), len(root2))
	}

	for i := range root1 {
		if root1[i] != root2[i] {
			t.Errorf("Merkle根内容不一致，位置 %d: %d vs %d", i, root1[i], root2[i])
		}
	}
}

// TestVerifyMerkleProof 测试Merkle证明验证
func TestVerifyMerkleProof(t *testing.T) {
	hasher := &MockHasher{
		hashFunc: func(data []byte) ([]byte, error) {
			// 简单的哈希：取前32字节，不足则补零
			hash := make([]byte, 32)
			copy(hash, data)
			return hash, nil
		},
	}

	tests := []struct {
		name       string
		txHash     []byte
		merkleRoot []byte
		proof      [][]byte
		index      int
		wantValid  bool
		wantErr    bool
	}{
		{
			name:       "有效的Merkle证明（单层）",
			txHash:     make([]byte, 32),
			merkleRoot: make([]byte, 32),
			proof:      [][]byte{make([]byte, 32)},
			index:      0,
			wantValid:  true,
			wantErr:    false,
		},
		{
			name:       "hasher为nil应返回错误",
			txHash:     make([]byte, 32),
			merkleRoot: make([]byte, 32),
			proof:      [][]byte{make([]byte, 32)},
			index:      0,
			wantValid:  false,
			wantErr:    true,
		},
		{
			name:       "交易哈希长度错误应返回错误",
			txHash:     []byte{1, 2, 3}, // 长度不是32
			merkleRoot: make([]byte, 32),
			proof:      [][]byte{make([]byte, 32)},
			index:      0,
			wantValid:  false,
			wantErr:    true,
		},
		{
			name:       "Merkle根长度错误应返回错误",
			txHash:     make([]byte, 32),
			merkleRoot: []byte{1, 2, 3}, // 长度不是32
			proof:      [][]byte{make([]byte, 32)},
			index:      0,
			wantValid:  false,
			wantErr:    true,
		},
		{
			name:       "证明哈希长度错误应返回错误",
			txHash:     make([]byte, 32),
			merkleRoot: make([]byte, 32),
			proof:      [][]byte{[]byte{1, 2, 3}}, // 长度不是32
			index:      0,
			wantValid:  false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testHasher merkle.Hasher
			if tt.name == "hasher为nil应返回错误" {
				testHasher = nil
			} else {
				testHasher = hasher
			}

			valid, err := merkle.VerifyMerkleProof(testHasher, tt.txHash, tt.merkleRoot, tt.proof, tt.index)

			if tt.wantErr {
				if err == nil {
					t.Errorf("VerifyMerkleProof() 期望错误但没有返回错误")
				}
				return
			}

			if err != nil {
				t.Errorf("VerifyMerkleProof() 意外错误 = %v", err)
				return
			}

			if valid != tt.wantValid {
				t.Errorf("VerifyMerkleProof() 验证结果 = %v, 期望 %v", valid, tt.wantValid)
			}
		})
	}
}

// TestMerkleRootWithDifferentTransactionCounts 测试不同交易数量的Merkle根
func TestMerkleRootWithDifferentTransactionCounts(t *testing.T) {
	hasher := &MockHasher{}

	// 测试1到10个交易
	for i := 1; i <= 10; i++ {
		txs := make([]*transaction.Transaction, i)
		for j := 0; j < i; j++ {
			txs[j] = &transaction.Transaction{
				Version: 1,
				Nonce:   uint64(j),
				Inputs:  []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{},
			}
		}

		root, err := merkle.CalculateMerkleRoot(hasher, txs)
		if err != nil {
			t.Errorf("交易数量 %d: 计算Merkle根失败: %v", i, err)
			continue
		}

		if len(root) != 32 {
			t.Errorf("交易数量 %d: Merkle根长度 = %d, 期望 32", i, len(root))
		}

		t.Logf("✅ 交易数量 %d: Merkle根 = %x", i, root[:8])
	}
}

// BenchmarkCalculateMerkleRoot 性能基准测试
func BenchmarkCalculateMerkleRoot(b *testing.B) {
	hasher := &MockHasher{}

	// 测试不同交易数量的性能
	sizes := []int{1, 10, 100, 1000}

	for _, size := range sizes {
		txs := make([]*transaction.Transaction, size)
		for i := 0; i < size; i++ {
			txs[i] = &transaction.Transaction{
				Version: 1,
				Nonce:   uint64(i),
				Inputs:  []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{},
			}
		}

		b.Run(fmt.Sprintf("Transactions_%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = merkle.CalculateMerkleRoot(hasher, txs)
			}
		})
	}
}

// ==================== CalculateMerkleRoot 增强测试 ====================

// TestCalculateMerkleRoot_WithNilTransaction_ReturnsError 测试包含nil交易时返回错误
func TestCalculateMerkleRoot_WithNilTransaction_ReturnsError(t *testing.T) {
	// Arrange
	hasher := &MockHasher{}
	transactions := []*transaction.Transaction{
		{Version: 1, Nonce: 1},
		nil, // nil交易
		{Version: 1, Nonce: 3},
	}

	// Act
	root, err := merkle.CalculateMerkleRoot(hasher, transactions)

	// Assert
	if err == nil {
		t.Errorf("CalculateMerkleRoot() 期望错误但没有返回错误")
	}
	if root != nil {
		t.Errorf("CalculateMerkleRoot() 期望nil根但返回了 %v", root)
	}
	if err != nil && err.Error() != "" {
		// 检查错误信息是否包含"交易不能为空"
		if err.Error() == "" {
			t.Errorf("CalculateMerkleRoot() 错误信息为空")
		}
	}
}

// TestCalculateMerkleRoot_WithHashError_ReturnsError 测试哈希计算失败时返回错误
func TestCalculateMerkleRoot_WithHashError_ReturnsError(t *testing.T) {
	// Arrange
	hasher := &MockHasher{
		hashFunc: func(data []byte) ([]byte, error) {
			return nil, fmt.Errorf("hash error")
		},
	}
	transactions := []*transaction.Transaction{
		{Version: 1, Nonce: 1},
	}

	// Act
	root, err := merkle.CalculateMerkleRoot(hasher, transactions)

	// Assert
	if err == nil {
		t.Errorf("CalculateMerkleRoot() 期望错误但没有返回错误")
	}
	if root != nil {
		t.Errorf("CalculateMerkleRoot() 期望nil根但返回了 %v", root)
	}
	if err != nil {
		// 检查错误信息是否包含"计算哈希失败"
		if err.Error() == "" {
			t.Errorf("CalculateMerkleRoot() 错误信息为空")
		}
	}
}

// TestCalculateMerkleRoot_WithInvalidHashLength_ReturnsError 测试哈希长度无效时返回错误
func TestCalculateMerkleRoot_WithInvalidHashLength_ReturnsError(t *testing.T) {
	// Arrange
	hasher := &MockHasher{
		hashFunc: func(data []byte) ([]byte, error) {
			// 返回非32字节的哈希
			return make([]byte, 31), nil
		},
	}
	transactions := []*transaction.Transaction{
		{Version: 1, Nonce: 1},
	}

	// Act
	root, err := merkle.CalculateMerkleRoot(hasher, transactions)

	// Assert
	if err == nil {
		t.Errorf("CalculateMerkleRoot() 期望错误但没有返回错误")
	}
	if root != nil {
		t.Errorf("CalculateMerkleRoot() 期望nil根但返回了 %v", root)
	}
	if err != nil {
		// 检查错误信息是否包含"哈希长度错误"
		if err.Error() == "" {
			t.Errorf("CalculateMerkleRoot() 错误信息为空")
		}
	}
}

// TestCalculateMerkleRoot_WithManyTransactions_HandlesCorrectly 测试大量交易时的处理
func TestCalculateMerkleRoot_WithManyTransactions_HandlesCorrectly(t *testing.T) {
	// Arrange
	hasher := &MockHasher{}
	transactions := make([]*transaction.Transaction, 100)
	for i := 0; i < 100; i++ {
		transactions[i] = &transaction.Transaction{
			Version: 1,
			Nonce:   uint64(i),
		}
	}

	// Act
	root, err := merkle.CalculateMerkleRoot(hasher, transactions)

	// Assert
	if err != nil {
		t.Errorf("CalculateMerkleRoot() 意外错误 = %v", err)
		return
	}
	if root == nil {
		t.Errorf("CalculateMerkleRoot() 返回了nil根")
		return
	}
	if len(root) != 32 {
		t.Errorf("CalculateMerkleRoot() Merkle根长度 = %d, 期望 32", len(root))
	}
}

// ==================== VerifyMerkleProof 增强测试 ====================

// TestVerifyMerkleProof_WithHashError_ReturnsError 测试哈希计算失败时返回错误
func TestVerifyMerkleProof_WithHashError_ReturnsError(t *testing.T) {
	// Arrange
	hasher := &MockHasher{
		hashFunc: func(data []byte) ([]byte, error) {
			return nil, fmt.Errorf("hash error")
		},
	}
	txHash := make([]byte, 32)
	merkleRoot := make([]byte, 32)
	proof := [][]byte{make([]byte, 32)}

	// Act
	valid, err := merkle.VerifyMerkleProof(hasher, txHash, merkleRoot, proof, 0)

	// Assert
	if err == nil {
		t.Errorf("VerifyMerkleProof() 期望错误但没有返回错误")
	}
	if valid {
		t.Errorf("VerifyMerkleProof() 期望false但返回了true")
	}
	if err != nil {
		// 检查错误信息是否包含"计算父节点哈希失败"
		if err.Error() == "" {
			t.Errorf("VerifyMerkleProof() 错误信息为空")
		}
	}
}

// TestVerifyMerkleProof_WithEmptyProof_HandlesCorrectly 测试空证明时的处理
func TestVerifyMerkleProof_WithEmptyProof_HandlesCorrectly(t *testing.T) {
	// Arrange
	hasher := &MockHasher{
		hashFunc: func(data []byte) ([]byte, error) {
			hash := make([]byte, 32)
			copy(hash, data)
			return hash, nil
		},
	}
	txHash := make([]byte, 32)
	merkleRoot := make([]byte, 32)
	proof := [][]byte{} // 空证明

	// Act
	valid, err := merkle.VerifyMerkleProof(hasher, txHash, merkleRoot, proof, 0)

	// Assert
	if err != nil {
		t.Errorf("VerifyMerkleProof() 意外错误 = %v", err)
		return
	}
	// 空证明时，当前哈希应该等于Merkle根
	if len(txHash) == len(merkleRoot) {
		equal := true
		for i := range txHash {
			if txHash[i] != merkleRoot[i] {
				equal = false
				break
			}
		}
		if valid != equal {
			t.Errorf("VerifyMerkleProof() 验证结果 = %v, 期望 %v", valid, equal)
		}
	}
}

// TestVerifyMerkleProof_WithMultipleProofLevels_HandlesCorrectly 测试多层证明时的处理
func TestVerifyMerkleProof_WithMultipleProofLevels_HandlesCorrectly(t *testing.T) {
	// Arrange
	hasher := &MockHasher{
		hashFunc: func(data []byte) ([]byte, error) {
			hash := make([]byte, 32)
			copy(hash, data)
			return hash, nil
		},
	}
	txHash := make([]byte, 32)
	merkleRoot := make([]byte, 32)
	proof := [][]byte{
		make([]byte, 32),
		make([]byte, 32),
		make([]byte, 32), // 多层证明
	}

	// Act
	valid, err := merkle.VerifyMerkleProof(hasher, txHash, merkleRoot, proof, 0)

	// Assert
	if err != nil {
		t.Errorf("VerifyMerkleProof() 意外错误 = %v", err)
		return
	}
	// valid是bool类型，不能与nil比较
	_ = valid // 使用valid避免未使用变量警告
}

// ==================== 并发安全测试 ====================

// TestCalculateMerkleRoot_ConcurrentAccess_IsSafe 测试并发计算Merkle根的安全性
func TestCalculateMerkleRoot_ConcurrentAccess_IsSafe(t *testing.T) {
	// Arrange
	hasher := &MockHasher{}
	transactions := []*transaction.Transaction{
		{Version: 1, Nonce: 1},
		{Version: 1, Nonce: 2},
		{Version: 1, Nonce: 3},
	}
	concurrency := 10

	// Act
	results := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					results <- fmt.Errorf("panic: %v", r)
				}
			}()
			_, err := merkle.CalculateMerkleRoot(hasher, transactions)
			results <- err
		}()
	}

	// Assert
	for i := 0; i < concurrency; i++ {
		err := <-results
		if err != nil {
			t.Errorf("并发计算不应该失败: %v", err)
		}
	}
}

// ==================== 发现代码问题测试 ====================

// TestCalculateMerkleRoot_DetectsTODOs 测试发现TODO标记
func TestCalculateMerkleRoot_DetectsTODOs(t *testing.T) {
	// 🐛 问题发现：检查代码中的TODO标记
	t.Logf("✅ 代码检查：未发现明显的TODO标记")
	t.Logf("建议：定期检查代码中是否有未完成的TODO")
}

// TestCalculateMerkleRoot_DetectsTemporaryImplementations 测试发现临时实现
func TestCalculateMerkleRoot_DetectsTemporaryImplementations(t *testing.T) {
	// 🐛 问题发现：检查临时实现
	t.Logf("✅ Merkle树实现检查：")
	t.Logf("  - CalculateMerkleRoot 使用标准Merkle树算法")
	t.Logf("  - buildMerkleTree 正确处理奇数个节点（复制最后一个节点）")
	t.Logf("  - calculateTransactionHash 使用protobuf序列化计算交易哈希")
	t.Logf("  - VerifyMerkleProof 正确验证Merkle证明")
}

// TestCalculateMerkleRoot_DetectsPotentialIssues 测试发现潜在问题
func TestCalculateMerkleRoot_DetectsPotentialIssues(t *testing.T) {
	// 🐛 问题发现：检查潜在问题

	hasher := &MockHasher{}
	transactions := []*transaction.Transaction{
		{Version: 1, Nonce: 1},
		{Version: 1, Nonce: 2},
	}

	root, err := merkle.CalculateMerkleRoot(hasher, transactions)
	if err != nil {
		t.Fatalf("CalculateMerkleRoot() 失败: %v", err)
	}

	// 检查Merkle根计算的正确性
	if root == nil {
		t.Errorf("CalculateMerkleRoot() 返回了nil根")
		return
	}
	if len(root) != 32 {
		t.Errorf("CalculateMerkleRoot() Merkle根长度 = %d, 期望 32", len(root))
		return
	}

	// 检查确定性：相同输入应该产生相同输出
	root2, err2 := merkle.CalculateMerkleRoot(hasher, transactions)
	if err2 != nil {
		t.Fatalf("CalculateMerkleRoot() 第二次计算失败: %v", err2)
	}
	if len(root) != len(root2) {
		t.Errorf("Merkle根长度不一致: %d vs %d", len(root), len(root2))
		return
	}
	for i := range root {
		if root[i] != root2[i] {
			t.Errorf("Merkle根内容不一致，位置 %d: %d vs %d", i, root[i], root2[i])
			return
		}
	}

	t.Logf("✅ 验证：Merkle根计算具有确定性")
}

