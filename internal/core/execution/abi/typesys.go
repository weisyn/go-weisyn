package abi

import (
	"fmt"
)

// TypeSystem ABI类型系统，负责管理和处理合约接口中的各种数据类型。
//
// 类型系统提供完整的类型定义、验证、转换和序列化支持，确保ABI操作的类型安全性。
// 支持基本类型、复合类型和自定义类型，以及灵活的类型转换规则。
//
// 核心功能：
//   - 类型注册和管理：支持基本类型和复合类型的注册
//   - 类型验证：确保值符合类型定义和约束
//   - 类型转换：提供安全的类型转换机制
//   - 序列化支持：为每种类型提供序列化和反序列化能力
//
// 设计特点：
//   - 可扩展性：支持自定义类型和转换规则
//   - 类型安全：严格的类型检查和验证
//   - 性能优化：缓存类型定义，减少查找开销
type TypeSystem struct {
	// basicTypes 基本类型定义映射表。
	// 存储如uint8、string、bool等基础数据类型的定义信息。
	basicTypes map[string]*TypeDefinition

	// compositeTypes 复合类型定义映射表。
	// 存储如数组、结构体、枚举等复杂数据类型的定义信息。
	compositeTypes map[string]*TypeDefinition

	// conversionRules 类型转换规则映射表。
	// 定义不同类型间的转换规则和转换器，支持安全的类型转换。
	// 结构：源类型 -> 目标类型 -> 转换规则
	conversionRules map[string]map[string]ConversionRule

	// config 类型系统配置，控制类型检查的严格程度和转换行为。
	config *TypeSystemConfig
}

// TypeSystemConfig 类型系统的配置结构。
//
// 提供类型系统行为的细粒度控制，支持不同场景下的类型处理需求。
type TypeSystemConfig struct {
	// EnableStrictTyping 严格类型检查开关。
	// 启用时要求严格的类型匹配，禁用时允许一定程度的类型兼容。
	EnableStrictTyping bool

	// EnableImplicitConversion 隐式类型转换开关。
	// 启用时允许安全的隐式类型转换，如int8到int32。
	EnableImplicitConversion bool

	// MaxNestedDepth 最大嵌套深度限制。
	// 防止复杂类型的无限嵌套导致的性能问题和栈溢出。
	MaxNestedDepth int

	// EnableCustomTypes 自定义类型支持开关。
	// 启用时允许注册和使用自定义类型定义。
	EnableCustomTypes bool
}

// TypeDefinition 类型定义结构，描述一个数据类型的完整信息。
//
// 包含类型的元数据、约束条件和序列化信息，为类型操作提供完整支持。
type TypeDefinition struct {
	// Name 类型名称，如"uint256"、"address"等。
	Name string

	// Category 类型分类，用于快速识别类型的基本特征。
	Category TypeCategory

	// Size 类型的字节大小，用于内存分配和序列化计算。
	Size int

	// Alignment 内存对齐要求，用于优化内存布局。
	Alignment int

	// Constraints 类型约束条件，定义值的有效范围和格式要求。
	Constraints TypeConstraints

	// Serializer 类型序列化器，负责将值转换为字节序列。
	Serializer TypeSerializer

	// Deserializer 类型反序列化器，负责从字节序列恢复值。
	Deserializer TypeDeserializer
}

// TypeCategory 类型分类枚举，用于对类型进行基本分类。
//
// 帮助类型系统快速识别类型特征，优化处理逻辑。
type TypeCategory string

const (
	// TypeCategoryBasic 基本类型分类
	// 包括整数、浮点数、布尔值、字符串等基础类型
	TypeCategoryBasic TypeCategory = "basic"

	// TypeCategoryArray 数组类型分类
	// 包括固定长度数组和动态数组
	TypeCategoryArray TypeCategory = "array"

	// TypeCategoryStruct 结构体类型分类
	// 包括复合数据结构和记录类型
	TypeCategoryStruct TypeCategory = "struct"

	// TypeCategoryEnum 枚举类型分类
	// 包括有限值集合的类型定义
	TypeCategoryEnum TypeCategory = "enum"

	// TypeCategoryPointer 指针类型分类
	// 包括引用类型和间接访问类型
	TypeCategoryPointer TypeCategory = "pointer"

	// TypeCategoryFunction 函数类型分类
	// 包括函数签名和回调类型
	TypeCategoryFunction TypeCategory = "function"
)

// TypeConstraints 类型约束结构，定义类型值的有效性规则。
//
// 提供细粒度的值验证，确保数据的合法性和一致性。
type TypeConstraints struct {
	// MinValue 最小值约束，适用于数值类型。
	// nil表示无最小值限制。
	MinValue *int64

	// MaxValue 最大值约束，适用于数值类型。
	// nil表示无最大值限制。
	MaxValue *int64

	// MinLength 最小长度约束，适用于字符串、数组等。
	// nil表示无最小长度限制。
	MinLength *int

	// MaxLength 最大长度约束，适用于字符串、数组等。
	// nil表示无最大长度限制。
	MaxLength *int

	// Pattern 正则表达式模式，适用于字符串格式验证。
	// 空字符串表示无格式要求。
	Pattern string

	// AllowedValues 允许值列表，用于枚举类型的值限制。
	// nil表示无值限制。
	AllowedValues []interface{}
}

// TypeSerializer 类型序列化器接口。
//
// 定义将类型值转换为字节序列的标准方法，用于数据存储和传输。
type TypeSerializer interface {
	// Serialize 将值序列化为字节序列。
	//
	// 参数：
	//   - value: 待序列化的值
	//
	// 返回值：
	//   - []byte: 序列化后的字节序列
	//   - error: 序列化过程中的错误
	Serialize(value interface{}) ([]byte, error)
}

// TypeDeserializer 类型反序列化器接口。
//
// 定义从字节序列恢复类型值的标准方法，用于数据读取和解析。
type TypeDeserializer interface {
	// Deserialize 从字节序列反序列化值。
	//
	// 参数：
	//   - data: 待反序列化的字节序列
	//
	// 返回值：
	//   - interface{}: 反序列化后的值
	//   - error: 反序列化过程中的错误
	Deserialize(data []byte) (interface{}, error)
}

// TypeConverter 类型转换器接口。
//
// 定义类型间转换的标准方法，支持安全的类型转换操作。
type TypeConverter interface {
	// Convert 执行类型转换。
	//
	// 参数：
	//   - value: 待转换的值
	//
	// 返回值：
	//   - interface{}: 转换后的值
	//   - error: 转换过程中的错误
	Convert(value interface{}) (interface{}, error)

	// CanConvert 检查是否可以转换指定值。
	//
	// 参数：
	//   - value: 待检查的值
	//
	// 返回值：
	//   - bool: true表示可以转换，false表示不能转换
	CanConvert(value interface{}) bool
}

// NewTypeSystem 创建新的类型系统实例。
//
// 初始化类型系统，注册基本类型，为类型操作做准备。
//
// 参数：
//   - cfg: 类型系统配置，控制类型检查和转换行为
//
// 返回值：
//   - *TypeSystem: 初始化完成的类型系统实例
func NewTypeSystem(cfg *TypeSystemConfig) *TypeSystem {
	ts := &TypeSystem{
		basicTypes:      make(map[string]*TypeDefinition),
		compositeTypes:  make(map[string]*TypeDefinition),
		conversionRules: make(map[string]map[string]ConversionRule),
		config:          cfg,
	}
	ts.initializeBasicTypes()
	return ts
}

// DefaultTypeSystemConfig 创建默认的类型系统配置。
//
// 返回一个平衡性能和安全性的默认配置，适用于大多数使用场景。
//
// 默认配置特点：
//   - 关闭严格类型检查，提高兼容性
//   - 启用隐式转换，增强易用性
//   - 设置合理的嵌套深度限制（10层）
//   - 启用自定义类型支持
//
// 返回值：
//   - *TypeSystemConfig: 包含默认设置的配置实例
func DefaultTypeSystemConfig() *TypeSystemConfig {
	return &TypeSystemConfig{
		EnableStrictTyping:       false,
		EnableImplicitConversion: true,
		MaxNestedDepth:           10,
		EnableCustomTypes:        true,
	}
}

// ConversionRule 类型转换规则结构，定义两种类型间的转换方法。
//
// 描述转换的源类型、目标类型、转换器和转换特性，支持灵活的类型转换配置。
type ConversionRule struct {
	// FromType 源类型名称。
	FromType string

	// ToType 目标类型名称。
	ToType string

	// Converter 类型转换器，执行实际的转换操作。
	Converter TypeConverter

	// IsLossy 是否为有损转换。
	// true表示转换可能丢失精度或信息，false表示无损转换。
	IsLossy bool

	// Requirements 转换要求，描述转换的前置条件和参数。
	// 键值对形式存储转换器特定的配置信息。
	Requirements map[string]interface{}
}

// ValidateType 验证值是否符合指定类型的要求。
//
// 当前实现为简化版本，只进行基本的空值检查。
// 生产环境中可扩展为完整的类型验证逻辑。
//
// 参数：
//   - typeName: 类型名称（当前实现中未使用，保留接口兼容性）
//   - value: 待验证的值
//
// 返回值：
//   - error: 验证错误，nil表示验证通过
func (ts *TypeSystem) ValidateType(_ string, value interface{}) error {
	if value == nil {
		return fmt.Errorf("nil value not allowed")
	}
	return nil
}

// ConvertType 执行类型转换操作。
//
// 当前实现为简化版本，直接返回原值。
// 生产环境中可扩展为完整的类型转换逻辑。
//
// 参数：
//   - targetType: 目标类型名称（当前实现中未使用，保留接口兼容性）
//   - value: 待转换的值
//
// 返回值：
//   - interface{}: 转换后的值
//   - error: 转换过程中的错误
func (ts *TypeSystem) ConvertType(_ string, value interface{}) (interface{}, error) {
	return value, nil
}

// initializeBasicTypes 初始化基本类型定义。
//
// 注册常用的基本数据类型，为类型系统提供基础支持。
// 包括各种整数类型、布尔类型、字符串类型和字节类型。
func (ts *TypeSystem) initializeBasicTypes() {
	basicTypeNames := []string{
		"uint8", "uint16", "uint32", "uint64",
		"int8", "int16", "int32", "int64",
		"bool", "string", "bytes",
	}

	for _, typeName := range basicTypeNames {
		ts.basicTypes[typeName] = &TypeDefinition{
			Name:     typeName,
			Category: TypeCategoryBasic,
			Size:     ts.calculateTypeSize(typeName),
		}
	}
}

// calculateTypeSize 计算类型的字节大小。
//
// 根据类型名称返回对应的字节大小，用于内存分配和序列化。
//
// 参数：
//   - typeName: 类型名称
//
// 返回值：
//   - int: 类型的字节大小
func (ts *TypeSystem) calculateTypeSize(typeName string) int {
	switch typeName {
	case "uint8", "int8":
		return 1
	case "uint16", "int16":
		return 2
	case "uint32", "int32":
		return 4
	case "uint64", "int64":
		return 8
	case "bool":
		return 1
	case "string", "bytes":
		return -1 // 变长类型
	default:
		return 0 // 未知类型
	}
}

// GetTypeDefinition 获取指定类型的定义信息。
//
// 从基本类型或复合类型中查找类型定义，支持类型信息的查询和验证。
//
// 参数：
//   - typeName: 类型名称
//
// 返回值：
//   - *TypeDefinition: 类型定义，nil表示类型不存在
//   - bool: 是否找到类型定义
func (ts *TypeSystem) GetTypeDefinition(typeName string) (*TypeDefinition, bool) {
	// 先查找基本类型
	if def, exists := ts.basicTypes[typeName]; exists {
		return def, true
	}

	// 再查找复合类型
	if def, exists := ts.compositeTypes[typeName]; exists {
		return def, true
	}

	return nil, false
}

// RegisterCustomType 注册自定义类型定义。
//
// 允许用户扩展类型系统，添加业务特定的数据类型。
//
// 参数：
//   - definition: 自定义类型定义
//
// 返回值：
//   - error: 注册过程中的错误
func (ts *TypeSystem) RegisterCustomType(definition *TypeDefinition) error {
	if !ts.config.EnableCustomTypes {
		return fmt.Errorf("custom types are disabled")
	}

	if definition.Name == "" {
		return fmt.Errorf("type name cannot be empty")
	}

	// 检查类型名是否已存在
	if _, exists := ts.GetTypeDefinition(definition.Name); exists {
		return fmt.Errorf("type %s already exists", definition.Name)
	}

	// 根据类型分类注册到相应的映射表
	if definition.Category == TypeCategoryBasic {
		ts.basicTypes[definition.Name] = definition
	} else {
		ts.compositeTypes[definition.Name] = definition
	}

	return nil
}
