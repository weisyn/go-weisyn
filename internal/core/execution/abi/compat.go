package abi

import (
	"fmt"
	"strconv"
	"strings"

	iface "github.com/weisyn/v1/internal/core/execution/interfaces"
	typespkg "github.com/weisyn/v1/pkg/types"
)

// defaultVersionComparator 默认版本比较器的非导出实现。
//
// 实现 interfaces.VersionComparator 接口，提供语义化版本的比较功能。
// 支持标准的语义化版本格式（如 "1.2.3"），用于ABI版本兼容性检查。
type defaultVersionComparator struct{}

// newDefaultVersionComparator 创建默认版本比较器实例。
//
// 返回值：
//   - iface.VersionComparator: 版本比较器接口实例
func newDefaultVersionComparator() iface.VersionComparator {
	return &defaultVersionComparator{}
}

// Compare 比较两个版本号的大小关系。
//
// 参数：
//   - v1: 第一个版本号
//   - v2: 第二个版本号
//
// 返回值：
//   - int: -1表示v1<v2，0表示v1==v2，1表示v1>v2
func (vc *defaultVersionComparator) Compare(v1, v2 string) int {
	if v1 == v2 {
		return 0
	}

	// 解析版本号
	ver1, err1 := vc.ParseVersion(v1)
	ver2, err2 := vc.ParseVersion(v2)

	// 如果解析失败，回退到字符串比较
	if err1 != nil || err2 != nil {
		if v1 < v2 {
			return -1
		}
		return 1
	}

	// 比较主版本号
	if ver1.Major != ver2.Major {
		if ver1.Major < ver2.Major {
			return -1
		}
		return 1
	}

	// 比较次版本号
	if ver1.Minor != ver2.Minor {
		if ver1.Minor < ver2.Minor {
			return -1
		}
		return 1
	}

	// 比较修订版本号
	if ver1.Patch != ver2.Patch {
		if ver1.Patch < ver2.Patch {
			return -1
		}
		return 1
	}

	return 0
}

// IsCompatible 检查当前版本是否与所需版本兼容。
//
// 兼容性规则：
//   - 主版本号相同时，当前版本的次版本号和修订版本号可以大于等于所需版本
//   - 主版本号不同时，不兼容（遵循语义化版本规范）
//
// 参数：
//   - current: 当前版本号
//   - required: 所需版本号
//
// 返回值：
//   - bool: true表示兼容，false表示不兼容
func (vc *defaultVersionComparator) IsCompatible(current, required string) bool {
	if current == required {
		return true
	}

	currVer, err1 := vc.ParseVersion(current)
	reqVer, err2 := vc.ParseVersion(required)

	if err1 != nil || err2 != nil {
		// 解析失败时要求严格相等
		return current == required
	}

	// 主版本号必须相同（语义化版本规范）
	if currVer.Major != reqVer.Major {
		return false
	}

	// 次版本号和修订版本号可以更高
	return currVer.Minor > reqVer.Minor ||
		(currVer.Minor == reqVer.Minor && currVer.Patch >= reqVer.Patch)
}

// ParseVersion 解析版本字符串为语义化版本结构。
//
// 支持标准的语义化版本格式："主版本号.次版本号.修订版本号"
//
// 参数：
//   - version: 版本字符串（如 "1.2.3"）
//
// 返回值：
//   - *iface.SemanticVersion: 解析后的版本结构
//   - error: 解析错误信息
func (vc *defaultVersionComparator) ParseVersion(version string) (*iface.SemanticVersion, error) {
	// 移除可能的前缀（如 "v1.2.3" -> "1.2.3"）
	version = strings.TrimPrefix(version, "v")

	// 分割版本号
	parts := strings.Split(version, ".")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid version format: %s", version)
	}

	// 解析主版本号
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", parts[0])
	}

	// 解析次版本号
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	// 解析修订版本号
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version: %s", parts[2])
	}

	return &iface.SemanticVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
		Pre:   "", // 预发布版本标识暂不支持
		Build: "", // 构建元数据暂不支持
	}, nil
}

// defaultMigrationExecutor 默认迁移执行器的非导出实现。
//
// 实现 interfaces.MigrationExecutor 接口，提供ABI版本间的迁移功能。
// 当前为简化实现，实际使用中可根据业务需求扩展迁移规则。
type defaultMigrationExecutor struct{}

// newDefaultMigrationExecutor 创建默认迁移执行器实例。
//
// 返回值：
//   - iface.MigrationExecutor: 迁移执行器接口实例
func newDefaultMigrationExecutor() iface.MigrationExecutor {
	return &defaultMigrationExecutor{}
}

// ExecuteMigration 执行版本迁移操作。
//
// 当前实现为透传模式，实际业务中可根据迁移规则进行数据转换。
//
// 参数：
//   - fromVersion: 源版本号
//   - toVersion: 目标版本号
//   - data: 待迁移的数据
//   - rules: 迁移规则集合
//
// 返回值：
//   - interface{}: 迁移后的数据
//   - error: 迁移过程中的错误
func (me *defaultMigrationExecutor) ExecuteMigration(fromVersion, toVersion string, data interface{}, rules []iface.MigrationRule) (interface{}, error) {
	// 当前为简化实现，直接返回原数据
	// 实际使用中可根据fromVersion和toVersion的差异，应用相应的迁移规则
	return data, nil
}

// ValidateMigration 验证迁移规则的有效性。
//
// 参数：
//   - fromVersion: 源版本号
//   - toVersion: 目标版本号
//   - rules: 迁移规则集合
//
// 返回值：
//   - error: 验证过程中的错误，nil表示验证通过
func (me *defaultMigrationExecutor) ValidateMigration(fromVersion, toVersion string, rules []iface.MigrationRule) error {
	// 当前为简化实现，总是返回成功
	// 实际使用中应验证迁移路径的有效性和规则的完整性
	return nil
}

// defaultCompatibilityService 默认兼容性服务的非导出实现。
//
// 实现 interfaces.CompatibilityService 接口，提供ABI版本间的兼容性检查功能。
// 结合版本比较器，对ABI结构进行详细的兼容性分析。
type defaultCompatibilityService struct {
	// comparator 版本比较器，用于版本号的比较操作
	comparator iface.VersionComparator
}

// newDefaultCompatibilityService 创建默认兼容性服务实例。
//
// 返回值：
//   - iface.CompatibilityService: 兼容性服务接口实例
func newDefaultCompatibilityService() iface.CompatibilityService {
	return &defaultCompatibilityService{
		comparator: newDefaultVersionComparator(),
	}
}

// IsCompatible 检查两个ABI版本是否兼容。
//
// 兼容性检查包括：
//   - 版本号兼容性检查
//   - 函数签名兼容性检查
//   - 参数类型兼容性检查
//
// 参数：
//   - oldABI: 旧版本的ABI定义
//   - newABI: 新版本的ABI定义
//
// 返回值：
//   - bool: true表示兼容，false表示不兼容
func (cs *defaultCompatibilityService) IsCompatible(oldABI, newABI *typespkg.ContractABI) bool {
	if oldABI == nil || newABI == nil {
		return false
	}

	// 版本兼容性检查：新版本应该向后兼容旧版本
	// 即检查当前新版本是否可以替代所需的旧版本
	if !cs.comparator.IsCompatible(newABI.Version, oldABI.Version) {
		return false
	}

	// 函数兼容性检查：新版本必须包含所有旧版本的函数
	oldFunctions := make(map[string]typespkg.ContractFunction)
	for _, fn := range oldABI.Functions {
		oldFunctions[fn.Name] = fn
	}

	newFunctions := make(map[string]typespkg.ContractFunction)
	for _, fn := range newABI.Functions {
		newFunctions[fn.Name] = fn
	}

	// 检查旧版本的所有函数在新版本中是否存在且兼容
	for name, oldFn := range oldFunctions {
		newFn, exists := newFunctions[name]
		if !exists {
			return false // 缺少函数，不兼容
		}

		// 检查函数签名兼容性
		if !cs.isFunctionCompatible(oldFn, newFn) {
			return false
		}
	}

	return true
}

// GenerateCompatibilityReport 生成详细的兼容性报告。
//
// 分析两个ABI版本间的差异，生成包含兼容性状态、变更详情和建议的报告。
//
// 参数：
//   - oldABI: 旧版本的ABI定义
//   - newABI: 新版本的ABI定义
//
// 返回值：
//   - *iface.CompatibilityReport: 详细的兼容性分析报告
func (cs *defaultCompatibilityService) GenerateCompatibilityReport(oldABI, newABI *typespkg.ContractABI) *iface.CompatibilityReport {
	report := &iface.CompatibilityReport{
		Compatible:        true,
		BreakingChanges:   []string{},
		AddedFunctions:    []string{},
		RemovedFunctions:  []string{},
		ModifiedFunctions: []string{},
		Recommendations:   []string{},
	}

	if oldABI == nil || newABI == nil {
		report.Compatible = false
		report.BreakingChanges = append(report.BreakingChanges, "One or both ABI definitions are nil")
		return report
	}

	// 构建函数映射
	oldFunctions := make(map[string]typespkg.ContractFunction)
	for _, fn := range oldABI.Functions {
		oldFunctions[fn.Name] = fn
	}

	newFunctions := make(map[string]typespkg.ContractFunction)
	for _, fn := range newABI.Functions {
		newFunctions[fn.Name] = fn
	}

	// 检查删除的函数
	for name := range oldFunctions {
		if _, exists := newFunctions[name]; !exists {
			report.RemovedFunctions = append(report.RemovedFunctions, name)
			report.BreakingChanges = append(report.BreakingChanges, "Function removed: "+name)
			report.Compatible = false
		}
	}

	// 检查新增的函数
	for name := range newFunctions {
		if _, exists := oldFunctions[name]; !exists {
			report.AddedFunctions = append(report.AddedFunctions, name)
		}
	}

	// 检查修改的函数
	for name, oldFn := range oldFunctions {
		if newFn, exists := newFunctions[name]; exists {
			if !cs.isFunctionCompatible(oldFn, newFn) {
				report.ModifiedFunctions = append(report.ModifiedFunctions, name)
				report.BreakingChanges = append(report.BreakingChanges, "Function signature changed: "+name)
				report.Compatible = false
			}
		}
	}

	// 生成建议
	if len(report.BreakingChanges) > 0 {
		report.Recommendations = append(report.Recommendations, "Consider incrementing major version due to breaking changes")
	}
	if len(report.AddedFunctions) > 0 {
		report.Recommendations = append(report.Recommendations, "Consider incrementing minor version due to new functions")
	}

	return report
}

// isFunctionCompatible 检查两个函数定义是否兼容。
//
// 比较函数的参数数量、类型和返回值类型，确保签名兼容性。
//
// 参数：
//   - oldFn: 旧版本的函数定义
//   - newFn: 新版本的函数定义
//
// 返回值：
//   - bool: true表示兼容，false表示不兼容
func (cs *defaultCompatibilityService) isFunctionCompatible(oldFn, newFn typespkg.ContractFunction) bool {
	// 检查参数数量
	if len(oldFn.Params) != len(newFn.Params) {
		return false
	}

	// 检查参数类型
	for i, oldParam := range oldFn.Params {
		newParam := newFn.Params[i]
		if oldParam.Type != newParam.Type {
			return false
		}
	}

	// 检查返回值数量
	if len(oldFn.Returns) != len(newFn.Returns) {
		return false
	}

	// 检查返回值类型
	for i, oldReturn := range oldFn.Returns {
		newReturn := newFn.Returns[i]
		if oldReturn.Type != newReturn.Type {
			return false
		}
	}

	return true
}
