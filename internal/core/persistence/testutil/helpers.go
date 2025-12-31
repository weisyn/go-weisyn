// Package testutil 提供测试辅助函数
package testutil

import (
	"github.com/weisyn/v1/internal/core/eutxo/testutil"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ==================== 辅助函数 ====================

// NewTestLogger 创建测试用的 Logger
func NewTestLogger() log.Logger {
	return testutil.NewTestLogger()
}

// NewTestBehavioralLogger 创建行为测试用的 Logger
func NewTestBehavioralLogger() *BehavioralMockLogger {
	return testutil.NewTestBehavioralLogger()
}

// NewTestBadgerStore 创建测试用的 BadgerStore
func NewTestBadgerStore() storage.BadgerStore {
	return testutil.NewTestBadgerStore()
}

// NewTestFileStore 创建测试用的 FileStore
func NewTestFileStore() storage.FileStore {
	return NewMockFileStore()
}

// NewTestHashManager 创建测试用的 HashManager
func NewTestHashManager() interface{} {
	return testutil.NewTestHashManager()
}

// NewTestBlockHashClient 创建测试用的 BlockHashServiceClient
func NewTestBlockHashClient() core.BlockHashServiceClient {
	return &MockBlockHashServiceClient{}
}

// NewTestTransactionHashClient 创建测试用的 TransactionHashServiceClient
func NewTestTransactionHashClient() transaction.TransactionHashServiceClient {
	return &MockTransactionHashServiceClient{}
}

