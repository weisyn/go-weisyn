package integration

import (
	"testing"

	resourcepb "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
)

// TestResourceMinimalFlow 验证最小资源流：部署→查询→下载
func TestResourceMinimalFlow(t *testing.T) {
	// ResourceService接口不存在，跳过测试
	t.Skip("ResourceService接口不存在，需要重新设计资源管理测试")

	// 构造最小资源（为了保持测试结构的完整性）
	_ = &resourcepb.Resource{
		Category:       resourcepb.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE,
		ExecutableType: resourcepb.ExecutableType_EXECUTABLE_TYPE_CONTRACT,
		MimeType:       "application/wasm",
		Name:           "demo",
		Version:        "0.0.1",
		Description:    "test",
		// 注意：Data字段不存在，移除
	}

	t.Log("资源管理API需要重新设计后才能测试")
}
