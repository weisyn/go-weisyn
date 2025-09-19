package interfaces

import types "github.com/weisyn/v1/pkg/types"

// InternalABIService 执行层内部使用的 ABI 服务接口
// 说明：比公共接口多了统计/报告等能力，供协调器/监控等内部组件使用
type InternalABIService interface {
	RegisterABI(contractID string, abi *types.ContractABI) error
	EncodeParameters(contractID, method string, args []interface{}) ([]byte, error)
	DecodeResult(contractID, method string, data []byte) ([]interface{}, error)
	// 可选：内部统计
	GetABIStats() *ABIStats
}

// ABIStats 内部统计快照（精简）
type ABIStats struct {
	TotalABIs          uint64
	EncodingOperations uint64
	DecodingOperations uint64
}

// ------------------------------------------------------------
// 编解码能力（可替换）

type Encoder interface {
	EncodeFunctionCall(fn *types.ContractFunction, args []interface{}) ([]byte, error)
	EncodeParameters(params []types.ABIParam, args []interface{}) ([]byte, error)
	EncodeValue(paramType string, value interface{}) ([]byte, error)
}

type Decoder interface {
	DecodeFunctionResult(fn *types.ContractFunction, data []byte) ([]interface{}, error)
	DecodeParameters(params []types.ABIParam, data []byte) ([]interface{}, error)
	DecodeValue(paramType string, data []byte) (interface{}, error)
}

// ------------------------------------------------------------
// 兼容性能力（可替换）

type VersionComparator interface {
	Compare(version1, version2 string) int
	IsCompatible(current, required string) bool
	ParseVersion(version string) (*SemanticVersion, error)
}

type MigrationExecutor interface {
	ExecuteMigration(fromVersion, toVersion string, data interface{}, rules []MigrationRule) (interface{}, error)
	ValidateMigration(fromVersion, toVersion string, rules []MigrationRule) error
}

type SemanticVersion struct {
	Major, Minor, Patch int
	Pre, Build          string
}

type CompatibilityReport struct {
	Compatible        bool
	BreakingChanges   []string
	AddedFunctions    []string
	RemovedFunctions  []string
	ModifiedFunctions []string
	Recommendations   []string
}

type MigrationRule struct{}

type CompatibilityService interface {
	IsCompatible(oldABI, newABI *types.ContractABI) bool
	GenerateCompatibilityReport(oldABI, newABI *types.ContractABI) *CompatibilityReport
}

// ------------------------------------------------------------
// 验证能力（可扩展）

type ValidationSeverity string

const (
	ValidationSeverityError   ValidationSeverity = "error"
	ValidationSeverityWarning ValidationSeverity = "warning"
	ValidationSeverityInfo    ValidationSeverity = "info"
)

type ValidationError struct {
	RuleName    string
	Severity    ValidationSeverity
	Message     string
	Location    string
	Suggestions []string
}

type ValidationRule interface {
	Validate(abi *types.ContractABI) []ValidationError
	GetRuleName() string
	GetSeverity() ValidationSeverity
}
