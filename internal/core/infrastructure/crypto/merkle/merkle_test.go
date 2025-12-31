package merkle

import (
	"testing"
)

func TestNewMerkleTree(t *testing.T) {
	data := [][]byte{
		[]byte("数据1"),
		[]byte("数据2"),
		[]byte("数据3"),
		[]byte("数据4"),
	}

	tree, err := NewMerkleTree(data)

	if err != nil {
		t.Fatalf("创建Merkle树失败: %v", err)
	}

	if tree.Root == nil {
		t.Fatal("Merkle树根节点为空")
	}

	if len(tree.Leaves) != 4 {
		t.Fatalf("叶子节点数量错误，期望4，实际%d", len(tree.Leaves))
	}
}

func TestMerkleTreeVerify(t *testing.T) {
	data := [][]byte{
		[]byte("数据1"),
		[]byte("数据2"),
		[]byte("数据3"),
		[]byte("数据4"),
	}

	tree, err := NewMerkleTree(data)

	if err != nil {
		t.Fatalf("创建Merkle树失败: %v", err)
	}

	// 验证存在的数据
	for _, d := range data {
		if !tree.Verify(d) {
			t.Errorf("应该验证通过的数据验证失败: %s", string(d))
		}
	}

	// 验证不存在的数据
	if tree.Verify([]byte("不存在的数据")) {
		t.Error("不存在的数据验证通过了")
	}
}

func TestMerkleTreeProof(t *testing.T) {
	data := [][]byte{
		[]byte("数据1"),
		[]byte("数据2"),
		[]byte("数据3"),
		[]byte("数据4"),
	}

	tree, err := NewMerkleTree(data)

	if err != nil {
		t.Fatalf("创建Merkle树失败: %v", err)
	}

	// 获取证明
	for _, d := range data {
		proof, err := tree.GetProof(d)

		if err != nil {
			t.Errorf("获取证明失败: %v", err)
			continue
		}

		// 验证证明
		if !tree.VerifyProof(d, proof, tree.Root.Hash) {
			t.Errorf("证明验证失败: %s", string(d))
		}
	}

	// 测试不存在数据的证明
	_, err = tree.GetProof([]byte("不存在的数据"))
	if err == nil {
		t.Error("对不存在的数据生成证明应该失败但成功了")
	}
}

func TestMerkleTreeOddLeaves(t *testing.T) {
	// 测试奇数个叶子节点的情况
	data := [][]byte{
		[]byte("数据1"),
		[]byte("数据2"),
		[]byte("数据3"),
	}

	tree, err := NewMerkleTree(data)

	if err != nil {
		t.Fatalf("创建Merkle树失败: %v", err)
	}

	if len(tree.Leaves) != 4 { // 应该有4个叶子，因为最后一个被复制了
		t.Fatalf("叶子节点数量错误，期望4，实际%d", len(tree.Leaves))
	}

	// 验证所有原始数据
	for _, d := range data {
		if !tree.Verify(d) {
			t.Errorf("应该验证通过的数据验证失败: %s", string(d))
		}
	}
}
