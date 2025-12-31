// Package testutil 辅助函数
package testutil

// 注意：testutil 包不提供 NewTestService 函数，以避免导入循环
// 测试文件应该直接调用 cas.NewService，使用 testutil 中的 Mock 对象
//
// 示例：
//   fileStore := testutil.NewMockFileStore()
//   hasher := &testutil.MockHashManager{}
//   logger := &testutil.MockLogger{}
//   service, err := cas.NewService(fileStore, hasher, logger)

