package block

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/weisyn/v1/pb/blockchain/block/transaction"
	"google.golang.org/protobuf/proto"
)

func TestBlockHashCalculation(t *testing.T) {
	tests := []struct {
		name        string
		block       *Block
		expectError bool
	}{
		{
			name: "valid_genesis_block",
			block: &Block{
				Header: &BlockHeader{
					Version:      1,
					PreviousHash: make([]byte, 32), // Genesis block has zero previous hash
					Height:       0,
					Timestamp:    uint64(time.Now().Unix()),
					MerkleRoot:   make([]byte, 32),
					Nonce:        []byte{0, 0, 0, 0, 0, 0, 0, 0},
					Difficulty:   1000000,
				},
				Body: &BlockBody{
					Transactions: []*transaction.Transaction{}, // Empty for genesis
				},
			},
			expectError: false,
		},
		{
			name: "valid_regular_block",
			block: &Block{
				Header: &BlockHeader{
					Version:      1,
					PreviousHash: []byte("prev_block_hash_32_bytes_long___"), // 32 bytes
					Height:       1,
					Timestamp:    uint64(time.Now().Unix()),
					MerkleRoot:   []byte("merkle_root_32_bytes_long_here___"), // 32 bytes
					Nonce:        []byte{1, 2, 3, 4, 5, 6, 7, 8},
					Difficulty:   1000000,
				},
				Body: &BlockBody{
					Transactions: []*transaction.Transaction{
						{
							Version:           1,
							Nonce:             12345,
							CreationTimestamp: uint64(time.Now().Unix()),
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "nil_block_header",
			block: &Block{
				Header: nil,
				Body: &BlockBody{
					Transactions: []*transaction.Transaction{},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 测试区块哈希计算
			hash, err := ComputeBlockHash(tt.block)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test case %s, but got none", tt.name)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for test case %s: %v", tt.name, err)
				return
			}

			// 验证哈希长度
			if len(hash) != 32 {
				t.Errorf("Expected hash length 32, got %d", len(hash))
			}

			// 验证哈希的一致性 - 相同输入应该产生相同哈希
			hash2, err := ComputeBlockHash(tt.block)
			if err != nil {
				t.Errorf("Error computing hash second time: %v", err)
				return
			}

			if string(hash) != string(hash2) {
				t.Errorf("Hash calculation is not deterministic")
			}
		})
	}
}

func TestBlockHashDeterminism(t *testing.T) {
	// 创建两个相同的区块
	block1 := &Block{
		Header: &BlockHeader{
			Version:      1,
			PreviousHash: []byte("same_prev_hash_32_bytes_long____"), // 32 bytes
			Height:       100,
			Timestamp:    1234567890,
			MerkleRoot:   []byte("same_merkle_root_32_bytes_long__"), // 32 bytes
			Nonce:        []byte{1, 2, 3, 4, 5, 6, 7, 8},
			Difficulty:   1000000,
		},
		Body: &BlockBody{
			Transactions: []*transaction.Transaction{},
		},
	}

	block2 := &Block{
		Header: &BlockHeader{
			Version:      1,
			PreviousHash: []byte("same_prev_hash_32_bytes_long____"), // 32 bytes
			Height:       100,
			Timestamp:    1234567890,
			MerkleRoot:   []byte("same_merkle_root_32_bytes_long__"), // 32 bytes
			Nonce:        []byte{1, 2, 3, 4, 5, 6, 7, 8},
			Difficulty:   1000000,
		},
		Body: &BlockBody{
			Transactions: []*transaction.Transaction{},
		},
	}

	hash1, err1 := ComputeBlockHash(block1)
	hash2, err2 := ComputeBlockHash(block2)

	if err1 != nil || err2 != nil {
		t.Fatalf("Hash computation failed: err1=%v, err2=%v", err1, err2)
	}

	if string(hash1) != string(hash2) {
		t.Errorf("Identical blocks should produce identical hashes")
	}
}

func TestBlockHashUniqueness(t *testing.T) {
	baseBlock := &Block{
		Header: &BlockHeader{
			Version:      1,
			PreviousHash: []byte("base_prev_hash_32_bytes_long____"), // 32 bytes
			Height:       100,
			Timestamp:    1234567890,
			MerkleRoot:   []byte("base_merkle_root_32_bytes_long__"), // 32 bytes
			Nonce:        []byte{1, 2, 3, 4, 5, 6, 7, 8},
			Difficulty:   1000000,
		},
		Body: &BlockBody{
			Transactions: []*transaction.Transaction{},
		},
	}

	// 创建修改版本 - 不同的高度
	modifiedBlock := &Block{
		Header: &BlockHeader{
			Version:      1,
			PreviousHash: []byte("base_prev_hash_32_bytes_long____"), // 32 bytes
			Height:       101,                                        // 不同的高度
			Timestamp:    1234567890,
			MerkleRoot:   []byte("base_merkle_root_32_bytes_long__"), // 32 bytes
			Nonce:        []byte{1, 2, 3, 4, 5, 6, 7, 8},
			Difficulty:   1000000,
		},
		Body: &BlockBody{
			Transactions: []*transaction.Transaction{},
		},
	}

	hash1, err1 := ComputeBlockHash(baseBlock)
	hash2, err2 := ComputeBlockHash(modifiedBlock)

	if err1 != nil || err2 != nil {
		t.Fatalf("Hash computation failed: err1=%v, err2=%v", err1, err2)
	}

	if string(hash1) == string(hash2) {
		t.Errorf("Different blocks should produce different hashes")
	}
}

// ComputeBlockHash 计算区块哈希的简单实现
// 在实际系统中，这应该在专门的哈希服务中实现
func ComputeBlockHash(block *Block) ([]byte, error) {
	if block == nil {
		return nil, ErrNilBlock
	}

	if block.Header == nil {
		return nil, ErrNilBlockHeader
	}

	// 只对区块头进行哈希计算，确保确定性
	headerBytes, err := proto.Marshal(block.Header)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(headerBytes)
	return hash[:], nil
}

// 定义测试用的错误类型
var (
	ErrNilBlock       = &BlockHashError{Code: "NIL_BLOCK", Message: "block cannot be nil"}
	ErrNilBlockHeader = &BlockHashError{Code: "NIL_BLOCK_HEADER", Message: "block header cannot be nil"}
)

type BlockHashError struct {
	Code    string
	Message string
}

func (e *BlockHashError) Error() string {
	return e.Message
}

func BenchmarkBlockHashCalculation(t *testing.B) {
	block := &Block{
		Header: &BlockHeader{
			Version:      1,
			PreviousHash: []byte("benchmark_hash_32_bytes_long____"), // 32 bytes
			Height:       1000,
			Timestamp:    1234567890,
			MerkleRoot:   []byte("benchmark_merkle_32_bytes_long__"), // 32 bytes
			Nonce:        []byte{1, 2, 3, 4, 5, 6, 7, 8},
			Difficulty:   1000000,
		},
		Body: &BlockBody{
			Transactions: []*transaction.Transaction{
				{
					Version:           1,
					Nonce:             12345,
					CreationTimestamp: 1234567890,
				},
			},
		},
	}

	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		_, err := ComputeBlockHash(block)
		if err != nil {
			t.Fatalf("Hash computation failed: %v", err)
		}
	}
}
