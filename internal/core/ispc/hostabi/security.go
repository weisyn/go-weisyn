package hostabi

import (
	"context"
	"fmt"
	"sync"
	"time"

	publicispc "github.com/weisyn/v1/pkg/interfaces/ispc"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbresource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
)

// PrimitiveOperationType 原语操作类型
type PrimitiveOperationType string

const (
	OperationTypeReadOnly  PrimitiveOperationType = "read_only"  // 只读操作
	OperationTypeWriteOnly PrimitiveOperationType = "write_only" // 写操作
	OperationTypeTrace     PrimitiveOperationType = "trace"      // 追踪操作
)

// PrimitiveSecurityConfig 原语安全配置
type PrimitiveSecurityConfig struct {
	// 操作类型
	OperationType PrimitiveOperationType
	// 最大调用频率（每秒调用次数，0表示无限制）
	MaxCallRatePerSecond uint64
	// 是否需要权限检查
	RequirePermissionCheck bool
	// 参数验证规则
	ParamValidationRules map[string]interface{}
}

// RateLimiter 调用频率限制器
type RateLimiter struct {
	// 每个原语的调用时间窗口（滑动窗口）
	callWindows map[string][]time.Time
	// 每个原语的最大调用频率
	maxRates map[string]uint64
	mutex    sync.RWMutex
}

// NewRateLimiter 创建调用频率限制器
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		callWindows: make(map[string][]time.Time),
		maxRates:    make(map[string]uint64),
	}
}

// SetMaxRate 设置原语的最大调用频率
func (r *RateLimiter) SetMaxRate(primitiveName string, maxRatePerSecond uint64) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.maxRates[primitiveName] = maxRatePerSecond
}

// CheckRateLimit 检查是否超过调用频率限制
//
// 🎯 **滑动窗口算法**：
// - 维护每个原语的调用时间窗口（最近1秒）
// - 如果窗口内的调用次数超过限制，返回错误
//
// 📋 **参数**：
//   - primitiveName: 原语名称
//
// 🔧 **返回值**：
//   - error: 如果超过限制，返回错误；否则返回nil
func (r *RateLimiter) CheckRateLimit(primitiveName string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	maxRate, exists := r.maxRates[primitiveName]
	if !exists || maxRate == 0 {
		// 没有限制或未配置，允许调用
		return nil
	}

	now := time.Now()
	windowStart := now.Add(-1 * time.Second)

	// 获取或创建调用时间窗口
	window, exists := r.callWindows[primitiveName]
	if !exists {
		window = []time.Time{}
	}

	// 清理窗口外的调用记录（超过1秒的记录）
	validWindow := []time.Time{}
	for _, callTime := range window {
		if callTime.After(windowStart) {
			validWindow = append(validWindow, callTime)
		}
	}

	// 检查是否超过限制
	if uint64(len(validWindow)) >= maxRate {
		return fmt.Errorf("原语 %s 调用频率超过限制: %d次/秒 (限制: %d次/秒)", primitiveName, len(validWindow), maxRate)
	}

	// 添加当前调用时间
	validWindow = append(validWindow, now)
	r.callWindows[primitiveName] = validWindow

	return nil
}

// Reset 重置所有调用频率限制
func (r *RateLimiter) Reset() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.callWindows = make(map[string][]time.Time)
}

// ParameterValidator 参数验证器
type ParameterValidator struct {
	// 验证规则
	validationRules map[string]func(interface{}) error
}

// NewParameterValidator 创建参数验证器
func NewParameterValidator() *ParameterValidator {
	validator := &ParameterValidator{
		validationRules: make(map[string]func(interface{}) error),
	}

	// 注册默认验证规则
	validator.registerDefaultRules()

	return validator
}

// registerDefaultRules 注册默认验证规则
func (v *ParameterValidator) registerDefaultRules() {
	// 地址验证规则（20字节）
	v.validationRules["address_20"] = func(value interface{}) error {
		addr, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("地址必须是[]byte类型")
		}
		if len(addr) != 20 {
			return fmt.Errorf("地址长度必须是20字节，实际长度: %d", len(addr))
		}
		return nil
	}

	// 哈希验证规则（32字节）
	v.validationRules["hash_32"] = func(value interface{}) error {
		hash, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("哈希必须是[]byte类型")
		}
		if len(hash) != 32 {
			return fmt.Errorf("哈希长度必须是32字节，实际长度: %d", len(hash))
		}
		return nil
	}

	// 非空验证规则
	v.validationRules["non_empty"] = func(value interface{}) error {
		if value == nil {
			return fmt.Errorf("参数不能为nil")
		}
		switch v := value.(type) {
		case []byte:
			if len(v) == 0 {
				return fmt.Errorf("参数不能为空")
			}
		case string:
			if v == "" {
				return fmt.Errorf("参数不能为空")
			}
		}
		return nil
	}
}

// ValidateParameter 验证参数
func (v *ParameterValidator) ValidateParameter(paramName string, value interface{}, ruleName string) error {
	rule, exists := v.validationRules[ruleName]
	if !exists {
		return fmt.Errorf("验证规则 %s 不存在", ruleName)
	}

	if err := rule(value); err != nil {
		return fmt.Errorf("参数 %s 验证失败: %w", paramName, err)
	}

	return nil
}

// PermissionChecker 权限检查器
type PermissionChecker struct {
	// 只读原语列表
	readOnlyPrimitives map[string]bool
	// 写操作原语列表
	writeOnlyPrimitives map[string]bool
}

// NewPermissionChecker 创建权限检查器
func NewPermissionChecker() *PermissionChecker {
	checker := &PermissionChecker{
		readOnlyPrimitives:  make(map[string]bool),
		writeOnlyPrimitives: make(map[string]bool),
	}

	// 初始化只读原语列表
	readOnlyPrimitives := []string{
		"GetBlockHeight",
		"GetBlockTimestamp",
		"GetBlockHash",
		"GetChainID",
		"GetCaller",
		"GetContractAddress",
		"GetTransactionID",
		"UTXOLookup",
		"UTXOExists",
		"ResourceLookup",
		"ResourceExists",
	}

	// 初始化写操作原语列表
	writeOnlyPrimitives := []string{
		"TxAddInput",
		"TxAddAssetOutput",
		"TxAddResourceOutput",
		"TxAddStateOutput",
	}

	for _, name := range readOnlyPrimitives {
		checker.readOnlyPrimitives[name] = true
	}

	for _, name := range writeOnlyPrimitives {
		checker.writeOnlyPrimitives[name] = true
	}

	return checker
}

// CheckPermission 检查权限
//
// 🎯 **权限检查**：
// - 只读操作：所有合约都可以调用
// - 写操作：需要验证调用者权限（未来可以扩展）
//
// 📋 **参数**：
//   - primitiveName: 原语名称
//   - callerAddress: 调用者地址
//
// 🔧 **返回值**：
//   - error: 如果权限不足，返回错误；否则返回nil
func (p *PermissionChecker) CheckPermission(primitiveName string, callerAddress []byte) error {
	// 只读操作：所有合约都可以调用
	if p.readOnlyPrimitives[primitiveName] {
		return nil
	}

	// 写操作：当前允许所有调用者（基本权限检查）
	// 📋 **当前实现**：
	// - 所有合约都可以执行写操作（AppendAssetOutput、AppendStateOutput等）
	// - 这是基本的安全策略，确保合约可以正常执行
	//
	// 🔮 **未来增强方向**：
	// - 可以实现更细粒度的权限检查，例如：
	//   1. 检查调用者是否有权限执行特定类型的写操作
	//   2. 基于合约地址或调用者身份进行权限验证
	//   3. 实现基于角色的访问控制（RBAC）
	//   4. 支持权限委托和撤销机制
	// - 这些增强功能需要更复杂的权限管理系统，当前基本实现已满足需求
	if p.writeOnlyPrimitives[primitiveName] {
		// 当前实现：允许所有调用者执行写操作
		// 未来可以扩展为更细粒度的权限检查
		return nil
	}

	// 追踪操作（EmitEvent、LogDebug）：所有合约都可以调用
	if primitiveName == "EmitEvent" || primitiveName == "LogDebug" {
		return nil
	}

	// 未知原语：拒绝访问
	return fmt.Errorf("未知原语: %s", primitiveName)
}

// SecurityEnforcer 安全执行器
//
// 🎯 **安全增强**：
// - 参数验证
// - 权限检查
// - 调用频率限制
type SecurityEnforcer struct {
	rateLimiter      *RateLimiter
	paramValidator   *ParameterValidator
	permissionChecker *PermissionChecker
}

// NewSecurityEnforcer 创建安全执行器
func NewSecurityEnforcer() *SecurityEnforcer {
	return &SecurityEnforcer{
		rateLimiter:       NewRateLimiter(),
		paramValidator:    NewParameterValidator(),
		permissionChecker: NewPermissionChecker(),
	}
}

// SetMaxRate 设置原语的最大调用频率
func (s *SecurityEnforcer) SetMaxRate(primitiveName string, maxRatePerSecond uint64) {
	s.rateLimiter.SetMaxRate(primitiveName, maxRatePerSecond)
}

// EnforceSecurity 执行安全检查
//
// 🎯 **安全检查流程**：
// 1. 检查调用频率限制
// 2. 检查权限
// 3. 验证参数（如果需要）
//
// 📋 **参数**：
//   - primitiveName: 原语名称
//   - callerAddress: 调用者地址
//   - params: 参数列表（用于参数验证）
//
// 🔧 **返回值**：
//   - error: 如果安全检查失败，返回错误；否则返回nil
func (s *SecurityEnforcer) EnforceSecurity(primitiveName string, callerAddress []byte, params map[string]interface{}) error {
	// 1. 检查调用频率限制
	if err := s.rateLimiter.CheckRateLimit(primitiveName); err != nil {
		return fmt.Errorf("调用频率限制: %w", err)
	}

	// 2. 检查权限
	if err := s.permissionChecker.CheckPermission(primitiveName, callerAddress); err != nil {
		return fmt.Errorf("权限检查失败: %w", err)
	}

	// 3. 验证参数（如果需要）
	if params != nil {
		if err := s.validatePrimitiveParams(primitiveName, params); err != nil {
			return fmt.Errorf("参数验证失败: %w", err)
		}
	}

	return nil
}

// validatePrimitiveParams 验证原语参数
func (s *SecurityEnforcer) validatePrimitiveParams(primitiveName string, params map[string]interface{}) error {
	switch primitiveName {
	case "TxAddAssetOutput":
		// 验证owner地址
		if owner, ok := params["owner"].([]byte); ok {
			if err := s.paramValidator.ValidateParameter("owner", owner, "address_20"); err != nil {
				return err
			}
		}
	case "TxAddResourceOutput":
		// 验证contentHash
		if contentHash, ok := params["contentHash"].([]byte); ok {
			if err := s.paramValidator.ValidateParameter("contentHash", contentHash, "hash_32"); err != nil {
				return err
			}
		}
		// 验证owner地址
		if owner, ok := params["owner"].([]byte); ok {
			if err := s.paramValidator.ValidateParameter("owner", owner, "address_20"); err != nil {
				return err
			}
		}
	case "TxAddStateOutput":
		// 验证executionResultHash
		if executionResultHash, ok := params["executionResultHash"].([]byte); ok {
			if err := s.paramValidator.ValidateParameter("executionResultHash", executionResultHash, "hash_32"); err != nil {
				return err
			}
		}
	case "UTXOLookup", "UTXOExists":
		// 验证outpoint
		if outpoint, ok := params["outpoint"].(*pb.OutPoint); ok {
			if outpoint == nil {
				return fmt.Errorf("outpoint不能为nil")
			}
		}
	case "ResourceLookup", "ResourceExists":
		// 验证contentHash
		if contentHash, ok := params["contentHash"].([]byte); ok {
			if err := s.paramValidator.ValidateParameter("contentHash", contentHash, "hash_32"); err != nil {
				return err
			}
		}
	}

	return nil
}

// HostRuntimePortsWithSecurity 带安全增强的HostABI实现包装器
type HostRuntimePortsWithSecurity struct {
	publicispc.HostABI
	securityEnforcer *SecurityEnforcer
	callerAddress    []byte
}

// NewHostRuntimePortsWithSecurity 创建带安全增强的HostABI包装器
func NewHostRuntimePortsWithSecurity(hostABI publicispc.HostABI, callerAddress []byte) *HostRuntimePortsWithSecurity {
	return &HostRuntimePortsWithSecurity{
		HostABI:          hostABI,
		securityEnforcer: NewSecurityEnforcer(),
		callerAddress:    callerAddress,
	}
}

// SetMaxRate 设置原语的最大调用频率
func (w *HostRuntimePortsWithSecurity) SetMaxRate(primitiveName string, maxRatePerSecond uint64) {
	w.securityEnforcer.SetMaxRate(primitiveName, maxRatePerSecond)
}

// 包装所有17个原语方法，添加安全检查

// 类别 A：确定性区块视图（4个）- 只读原语
func (w *HostRuntimePortsWithSecurity) GetBlockHeight(ctx context.Context) (uint64, error) {
	params := map[string]interface{}{}
	if err := w.securityEnforcer.EnforceSecurity("GetBlockHeight", w.callerAddress, params); err != nil {
		return 0, err
	}
	return w.HostABI.GetBlockHeight(ctx)
}

func (w *HostRuntimePortsWithSecurity) GetBlockTimestamp(ctx context.Context) (uint64, error) {
	params := map[string]interface{}{}
	if err := w.securityEnforcer.EnforceSecurity("GetBlockTimestamp", w.callerAddress, params); err != nil {
		return 0, err
	}
	return w.HostABI.GetBlockTimestamp(ctx)
}

func (w *HostRuntimePortsWithSecurity) GetBlockHash(ctx context.Context, height uint64) ([]byte, error) {
	params := map[string]interface{}{
		"height": height,
	}
	if err := w.securityEnforcer.EnforceSecurity("GetBlockHash", w.callerAddress, params); err != nil {
		return nil, err
	}
	return w.HostABI.GetBlockHash(ctx, height)
}

func (w *HostRuntimePortsWithSecurity) GetChainID(ctx context.Context) ([]byte, error) {
	params := map[string]interface{}{}
	if err := w.securityEnforcer.EnforceSecurity("GetChainID", w.callerAddress, params); err != nil {
		return nil, err
	}
	return w.HostABI.GetChainID(ctx)
}

// 类别 B：执行上下文（3个）- 只读原语
func (w *HostRuntimePortsWithSecurity) GetCaller(ctx context.Context) ([]byte, error) {
	params := map[string]interface{}{}
	if err := w.securityEnforcer.EnforceSecurity("GetCaller", w.callerAddress, params); err != nil {
		return nil, err
	}
	return w.HostABI.GetCaller(ctx)
}

func (w *HostRuntimePortsWithSecurity) GetContractAddress(ctx context.Context) ([]byte, error) {
	params := map[string]interface{}{}
	if err := w.securityEnforcer.EnforceSecurity("GetContractAddress", w.callerAddress, params); err != nil {
		return nil, err
	}
	return w.HostABI.GetContractAddress(ctx)
}

func (w *HostRuntimePortsWithSecurity) GetTransactionID(ctx context.Context) ([]byte, error) {
	params := map[string]interface{}{}
	if err := w.securityEnforcer.EnforceSecurity("GetTransactionID", w.callerAddress, params); err != nil {
		return nil, err
	}
	return w.HostABI.GetTransactionID(ctx)
}

// 类别 C：UTXO查询（2个）- 只读原语
func (w *HostRuntimePortsWithSecurity) UTXOLookup(ctx context.Context, outpoint *pb.OutPoint) (*pb.TxOutput, error) {
	params := map[string]interface{}{
		"outpoint": outpoint,
	}
	if err := w.securityEnforcer.EnforceSecurity("UTXOLookup", w.callerAddress, params); err != nil {
		return nil, err
	}
	return w.HostABI.UTXOLookup(ctx, outpoint)
}

func (w *HostRuntimePortsWithSecurity) UTXOExists(ctx context.Context, outpoint *pb.OutPoint) (bool, error) {
	params := map[string]interface{}{
		"outpoint": outpoint,
	}
	if err := w.securityEnforcer.EnforceSecurity("UTXOExists", w.callerAddress, params); err != nil {
		return false, err
	}
	return w.HostABI.UTXOExists(ctx, outpoint)
}

// 类别 D：资源查询（2个）- 只读原语
func (w *HostRuntimePortsWithSecurity) ResourceLookup(ctx context.Context, contentHash []byte) (*pbresource.Resource, error) {
	params := map[string]interface{}{
		"contentHash": contentHash,
	}
	if err := w.securityEnforcer.EnforceSecurity("ResourceLookup", w.callerAddress, params); err != nil {
		return nil, err
	}
	return w.HostABI.ResourceLookup(ctx, contentHash)
}

func (w *HostRuntimePortsWithSecurity) ResourceExists(ctx context.Context, contentHash []byte) (bool, error) {
	params := map[string]interface{}{
		"contentHash": contentHash,
	}
	if err := w.securityEnforcer.EnforceSecurity("ResourceExists", w.callerAddress, params); err != nil {
		return false, err
	}
	return w.HostABI.ResourceExists(ctx, contentHash)
}

// 类别 E：交易草稿构建（4个）- 写操作原语
func (w *HostRuntimePortsWithSecurity) TxAddInput(ctx context.Context, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) {
	params := map[string]interface{}{
		"outpoint": outpoint,
	}
	if err := w.securityEnforcer.EnforceSecurity("TxAddInput", w.callerAddress, params); err != nil {
		return 0, err
	}
	return w.HostABI.TxAddInput(ctx, outpoint, isReferenceOnly, unlockingProof)
}

func (w *HostRuntimePortsWithSecurity) TxAddAssetOutput(
	ctx context.Context,
	owner []byte,
	amount uint64,
	tokenID []byte,
	lockingConditions []*pb.LockingCondition,
) (uint32, error) {
	params := map[string]interface{}{
		"owner": owner,
	}
	if err := w.securityEnforcer.EnforceSecurity("TxAddAssetOutput", w.callerAddress, params); err != nil {
		return 0, err
	}
	return w.HostABI.TxAddAssetOutput(ctx, owner, amount, tokenID, lockingConditions)
}

func (w *HostRuntimePortsWithSecurity) TxAddResourceOutput(
	ctx context.Context,
	contentHash []byte,
	category string,
	owner []byte,
	lockingConditions []*pb.LockingCondition,
	metadata []byte,
) (uint32, error) {
	params := map[string]interface{}{
		"contentHash": contentHash,
		"owner":       owner,
	}
	if err := w.securityEnforcer.EnforceSecurity("TxAddResourceOutput", w.callerAddress, params); err != nil {
		return 0, err
	}
	return w.HostABI.TxAddResourceOutput(ctx, contentHash, category, owner, lockingConditions, metadata)
}

func (w *HostRuntimePortsWithSecurity) TxAddStateOutput(
	ctx context.Context,
	stateID []byte,
	stateVersion uint64,
	executionResultHash []byte,
	publicInputs []byte,
	parentStateHash []byte,
) (uint32, error) {
	params := map[string]interface{}{
		"executionResultHash": executionResultHash,
	}
	if err := w.securityEnforcer.EnforceSecurity("TxAddStateOutput", w.callerAddress, params); err != nil {
		return 0, err
	}
	return w.HostABI.TxAddStateOutput(ctx, stateID, stateVersion, executionResultHash, publicInputs, parentStateHash)
}

// 类别 G：执行追踪（2个）- 追踪原语
func (w *HostRuntimePortsWithSecurity) EmitEvent(ctx context.Context, eventType string, eventData []byte) error {
	params := map[string]interface{}{
		"eventType": eventType,
	}
	if err := w.securityEnforcer.EnforceSecurity("EmitEvent", w.callerAddress, params); err != nil {
		return err
	}
	return w.HostABI.EmitEvent(ctx, eventType, eventData)
}

func (w *HostRuntimePortsWithSecurity) LogDebug(ctx context.Context, message string) error {
	params := map[string]interface{}{
		"message": message,
	}
	if err := w.securityEnforcer.EnforceSecurity("LogDebug", w.callerAddress, params); err != nil {
		return err
	}
	return w.HostABI.LogDebug(ctx, message)
}

// 确保实现接口
var _ publicispc.HostABI = (*HostRuntimePortsWithSecurity)(nil)

