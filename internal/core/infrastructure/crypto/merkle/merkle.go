// Package merkle 提供Merkle树相关功能
package merkle

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/weisyn/v1/internal/core/infrastructure/crypto/hash"
	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

// MerkleTree 表示一个Merkle树
type MerkleTree struct {
	Root       *MerkleNode       // 树的根节点
	Leaves     []*MerkleNode     // 叶子节点切片
	hashFunc   *hash.HashService // 使用的哈希函数
	merkleRoot []byte            // 缓存的根哈希
}

// 确保MerkleTree实现了cryptointf.MerkleTree接口
var _ cryptointf.MerkleTree = (*MerkleTree)(nil)

// MerkleNode 表示Merkle树中的一个节点
type MerkleNode struct {
	Tree   *MerkleTree // 指向树的引用
	Parent *MerkleNode // 父节点
	Left   *MerkleNode // 左子节点
	Right  *MerkleNode // 右子节点
	Hash   []byte      // 节点哈希值
	Data   []byte      // 原始数据（仅叶子节点有）
	dup    bool        // 是否为复制节点（处理奇数叶子）
}

// GetRoot 获取树的根节点哈希
func (m *MerkleTree) GetRoot() []byte {
	if m.Root == nil {
		return nil
	}
	return m.Root.Hash
}

// GetLeaves 获取所有叶子节点哈希
func (m *MerkleTree) GetLeaves() [][]byte {
	result := make([][]byte, len(m.Leaves))
	for i, leaf := range m.Leaves {
		result[i] = leaf.Hash
	}
	return result
}

// NewMerkleTree 创建一个新的Merkle树
// 参数:
//   - data: 用于构建树的数据切片
//
// 返回:
//   - *MerkleTree: 创建的Merkle树
//   - error: 错误信息
func NewMerkleTree(data [][]byte) (*MerkleTree, error) {
	if len(data) == 0 {
		return nil, errors.New("数据不能为空")
	}

	hashService := hash.NewHashService()

	mt := &MerkleTree{
		hashFunc: hashService,
	}

	// 创建叶子节点
	var leaves []*MerkleNode
	for i, datum := range data {
		node := &MerkleNode{
			Tree: mt,
			Hash: hashService.SHA256(datum),
			Data: datum,
		}
		leaves = append(leaves, node)

		// 如果是最后一个节点且总数为奇数，复制它
		if i == len(data)-1 && len(data)%2 != 0 {
			dupNode := &MerkleNode{
				Tree: mt,
				Hash: node.Hash,
				Data: datum,
				dup:  true,
			}
			leaves = append(leaves, dupNode)
		}
	}

	mt.Leaves = leaves

	// 构建树
	root, err := mt.buildTree(leaves)
	if err != nil {
		return nil, err
	}

	mt.Root = root
	mt.merkleRoot = root.Hash

	return mt, nil
}

// buildTree 构建Merkle树
func (m *MerkleTree) buildTree(nodes []*MerkleNode) (*MerkleNode, error) {
	if len(nodes) == 0 {
		return nil, errors.New("节点列表不能为空")
	}

	if len(nodes) == 1 {
		return nodes[0], nil
	}

	var nextLevel []*MerkleNode

	// 处理每一对节点
	for i := 0; i < len(nodes); i += 2 {
		left := nodes[i]
		var right *MerkleNode

		if i+1 < len(nodes) {
			right = nodes[i+1]
		} else {
			// 奇数个节点，复制最后一个
			right = &MerkleNode{
				Tree: m,
				Hash: left.Hash,
				Data: left.Data,
				dup:  true,
			}
		}

		// 创建父节点
		combined := append(left.Hash, right.Hash...)
		parentHash := m.hashFunc.SHA256(combined)

		parent := &MerkleNode{
			Tree:  m,
			Left:  left,
			Right: right,
			Hash:  parentHash,
		}

		// 设置子节点的父节点引用
		left.Parent = parent
		right.Parent = parent

		nextLevel = append(nextLevel, parent)
	}

	// 递归构建上一层
	return m.buildTree(nextLevel)
}

// Verify 验证数据是否在Merkle树中
func (m *MerkleTree) Verify(data []byte) bool {
	hash := m.hashFunc.SHA256(data)

	for _, leaf := range m.Leaves {
		if bytes.Equal(leaf.Hash, hash) && !leaf.dup {
			return true
		}
	}

	return false
}

// GetProof 生成Merkle证明
func (m *MerkleTree) GetProof(data []byte) ([][]byte, error) {
	hash := m.hashFunc.SHA256(data)

	// 找到对应的叶子节点
	var targetLeaf *MerkleNode
	for _, leaf := range m.Leaves {
		if bytes.Equal(leaf.Hash, hash) && !leaf.dup {
			targetLeaf = leaf
			break
		}
	}

	if targetLeaf == nil {
		return nil, errors.New("数据不在Merkle树中")
	}

	var proof [][]byte
	current := targetLeaf

	// 从叶子节点向上遍历到根节点
	for current.Parent != nil {
		parent := current.Parent

		if parent.Left == current {
			// 当前节点是左子节点，添加右兄弟的哈希
			if parent.Right != nil {
				proof = append(proof, parent.Right.Hash)
			}
		} else {
			// 当前节点是右子节点，添加左兄弟的哈希
			if parent.Left != nil {
				proof = append(proof, parent.Left.Hash)
			}
		}

		current = parent
	}

	return proof, nil
}

// VerifyProof 验证Merkle证明
func (m *MerkleTree) VerifyProof(data []byte, proof [][]byte, rootHash []byte) bool {
	hash := m.hashFunc.SHA256(data)

	// 找到数据在叶子节点中的位置
	leafIndex := -1
	for i, leaf := range m.Leaves {
		if bytes.Equal(leaf.Hash, hash) && !leaf.dup {
			leafIndex = i
			break
		}
	}

	if leafIndex == -1 {
		return false
	}

	// 使用证明重新计算根哈希
	currentHash := hash
	currentIndex := leafIndex

	for _, proofHash := range proof {
		var combined []byte

		// 根据索引确定哈希顺序
		if currentIndex%2 == 0 {
			// 当前是左子节点
			combined = append(currentHash, proofHash...)
		} else {
			// 当前是右子节点
			combined = append(proofHash, currentHash...)
		}

		currentHash = m.hashFunc.SHA256(combined)
		currentIndex = currentIndex / 2
	}

	return bytes.Equal(currentHash, rootHash)
}

// GetMerklePath 获取Merkle路径和索引信息（兼容性方法）
func (m *MerkleTree) GetMerklePath(data []byte) ([][]byte, []int64, error) {
	proof, err := m.GetProof(data)
	if err != nil {
		return nil, nil, err
	}

	hash := m.hashFunc.SHA256(data)
	leafIndex := -1
	for i, leaf := range m.Leaves {
		if bytes.Equal(leaf.Hash, hash) && !leaf.dup {
			leafIndex = i
			break
		}
	}

	if leafIndex == -1 {
		return nil, nil, errors.New("数据不在Merkle树中")
	}

	// 计算路径索引
	var indices []int64
	currentIndex := leafIndex

	for i := 0; i < len(proof); i++ {
		indices = append(indices, int64(currentIndex%2))
		currentIndex = currentIndex / 2
	}

	return proof, indices, nil
}

// String 返回树的字符串表示
func (m *MerkleTree) String() string {
	if m.Root == nil {
		return "Empty Merkle Tree"
	}

	return fmt.Sprintf("Merkle Tree - Root: %x, Leaves: %d", m.Root.Hash, len(m.Leaves))
}

// RebuildTree 重建树（保持现有叶子数据）
func (m *MerkleTree) RebuildTree() error {
	if len(m.Leaves) == 0 {
		return errors.New("没有叶子节点可以重建")
	}

	var data [][]byte
	for _, leaf := range m.Leaves {
		if !leaf.dup && leaf.Data != nil {
			data = append(data, leaf.Data)
		}
	}

	newTree, err := NewMerkleTree(data)
	if err != nil {
		return err
	}

	m.Root = newTree.Root
	m.Leaves = newTree.Leaves
	m.merkleRoot = newTree.merkleRoot

	return nil
}

// VerifyTree 验证整个树的完整性
func (m *MerkleTree) VerifyTree() (bool, error) {
	if m.Root == nil {
		return false, errors.New("树为空")
	}

	return m.verifyNode(m.Root), nil
}

// verifyNode 递归验证节点
func (m *MerkleTree) verifyNode(node *MerkleNode) bool {
	if node == nil {
		return true
	}

	// 如果是叶子节点
	if node.Left == nil && node.Right == nil {
		return node.Data != nil && bytes.Equal(node.Hash, m.hashFunc.SHA256(node.Data))
	}

	// 如果是内部节点
	if node.Left == nil || node.Right == nil {
		return false
	}

	// 验证子节点
	if !m.verifyNode(node.Left) || !m.verifyNode(node.Right) {
		return false
	}

	// 验证当前节点的哈希
	combined := append(node.Left.Hash, node.Right.Hash...)
	expectedHash := m.hashFunc.SHA256(combined)

	return bytes.Equal(node.Hash, expectedHash)
}

// MerkleService 实现MerkleTreeManager接口
type MerkleService struct {
	hashService *hash.HashService
}

// 确保MerkleService实现了cryptointf.MerkleTreeManager接口
var _ cryptointf.MerkleTreeManager = (*MerkleService)(nil)

// NewMerkleService 创建一个新的Merkle服务
func NewMerkleService() *MerkleService {
	return &MerkleService{
		hashService: hash.NewHashService(),
	}
}

// NewMerkleTree 创建一个新的Merkle树
func (s *MerkleService) NewMerkleTree(data [][]byte) (cryptointf.MerkleTree, error) {
	return NewMerkleTree(data)
}

// Verify 验证数据是否在Merkle树中
func (s *MerkleService) Verify(tree cryptointf.MerkleTree, data []byte) bool {
	return tree.Verify(data)
}

// VerifyProof 验证Merkle证明
func (s *MerkleService) VerifyProof(tree cryptointf.MerkleTree, data []byte, proof [][]byte, rootHash []byte) bool {
	return tree.VerifyProof(data, proof, rootHash)
}

// GetProof 生成Merkle证明
func (s *MerkleService) GetProof(tree cryptointf.MerkleTree, data []byte) ([][]byte, error) {
	return tree.GetProof(data)
}
