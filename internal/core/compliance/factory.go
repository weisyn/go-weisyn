// Package compliance 提供合规服务工厂实现
package compliance

import (
	"github.com/weisyn/v1/internal/config/compliance"
	"github.com/weisyn/v1/internal/core/compliance/geoip"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// CreateCompliancePolicy 创建合规策略服务
//
// 🏭 **工厂函数**：
// 将服务创建逻辑从module.go中分离，保持module.go的薄实现。
// 这个函数负责创建合规策略服务的完整逻辑。
//
// 参数：
//   - config: 合规配置选项
//   - logger: 日志记录器（可选）
//
// 返回：
//   - complianceIfaces.Policy: 合规策略服务实例
//   - error: 创建过程中的错误
func CreateCompliancePolicy(config *compliance.ComplianceOptions, logger log.Logger) (complianceIfaces.Policy, error) {
	// 创建GeoIP服务（唯一依赖）
	geoipService, err := geoip.NewDBIPService(config, logger)
	if err != nil {
		return nil, err
	}

	// 创建合规策略服务（纯基于地理位置）
	policy := NewCompliancePolicyService(config, logger, geoipService)

	return policy, nil
}
