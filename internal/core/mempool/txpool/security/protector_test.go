package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTxPoolProtector_CheckTransaction(t *testing.T) {
	tpp := NewTxPoolProtector(100, 1000)

	// 测试正常交易
	err := tpp.CheckTransaction("peer1")
	assert.NoError(t, err)

	// 测试单节点交易数限制
	for i := 0; i < 100; i++ {
		err = tpp.AddTransaction("peer1")
		assert.NoError(t, err)
	}

	// 测试单节点交易数限制
	err = tpp.AddTransaction("peer1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "单节点交易数已达上限")
}

func TestTxPoolProtector_RemoveTransaction(t *testing.T) {
	tpp := NewTxPoolProtector(100, 1000)

	// 添加交易
	for i := 0; i < 10; i++ {
		tpp.AddTransaction("peer1")
	}
	assert.Equal(t, 10, tpp.GetTransactionCount("peer1"))

	// 移除交易
	for i := 0; i < 5; i++ {
		tpp.RemoveTransaction("peer1")
	}
	assert.Equal(t, 5, tpp.GetTransactionCount("peer1"))

	// 移除不存在的交易
	tpp.RemoveTransaction("peer2")
	assert.Equal(t, 0, tpp.GetTransactionCount("peer2"))
}

func TestTxPoolProtector_GetUsageRate(t *testing.T) {
	tpp := NewTxPoolProtector(100, 1000)

	// 测试空交易池
	rate := tpp.GetUsageRate()
	assert.Equal(t, 0.0, rate)

	// 添加交易
	for i := 0; i < 500; i++ {
		peerID := "peer" + string(rune(i/100+1))
		tpp.AddTransaction(peerID)
	}

	// 测试使用率
	rate = tpp.GetUsageRate()
	assert.Equal(t, 0.5, rate)
}

func TestTxPoolProtector_Reset(t *testing.T) {
	tpp := NewTxPoolProtector(100, 1000)

	// 添加交易
	for i := 0; i < 10; i++ {
		tpp.AddTransaction("peer1")
	}
	assert.Equal(t, 10, tpp.GetTransactionCount("peer1"))

	// 重置
	tpp.Reset("peer1")
	assert.Equal(t, 0, tpp.GetTransactionCount("peer1"))

	// 重置后可以继续添加交易
	err := tpp.AddTransaction("peer1")
	assert.NoError(t, err)
}

func TestTxPoolProtector_ResetAll(t *testing.T) {
	tpp := NewTxPoolProtector(100, 1000)

	// 添加交易
	tpp.AddTransaction("peer1")
	tpp.AddTransaction("peer2")
	tpp.AddTransaction("peer3")

	// 重置所有
	tpp.ResetAll()

	// 验证所有交易计数被清空
	assert.Equal(t, 0, tpp.GetTransactionCount("peer1"))
	assert.Equal(t, 0, tpp.GetTransactionCount("peer2"))
	assert.Equal(t, 0, tpp.GetTransactionCount("peer3"))
}
