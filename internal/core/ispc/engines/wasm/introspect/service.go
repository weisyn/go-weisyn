// Package introspect 提供WASM字节码解析和分析工具
//
// 🎯 **设计理念**: 统一的WASM解析服务,避免客户端与服务端重复实现
// 📋 **核心职责**: 提供WASM模块的静态分析能力,如导出函数提取、ABI解析等
package introspect

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tetratelabs/wazero"
	corelog "github.com/weisyn/v1/internal/core/infrastructure/log"
)

// IntrospectionService WASM模块分析服务
//
// 🎯 **设计目标**:
//   - 提供统一的WASM解析能力,服务端/客户端/工具链共享
//   - 基于wazero实现,零依赖,高性能
//   - 线程安全,支持并发调用
//
// 📋 **使用场景**:
//   - 合约部署时自动提取导出函数列表
//   - 合约调用前验证函数存在性
//   - 开发工具展示合约接口信息
//   - ABI生成与校验
type IntrospectionService struct {
	// 可扩展字段,如缓存、配置等
}

// NewIntrospectionService 创建WASM分析服务实例
func NewIntrospectionService() *IntrospectionService {
	return &IntrospectionService{}
}

// ExtractExportedFunctions 从WASM字节码中提取导出函数列表
//
// 🎯 **核心功能**: 解析WASM模块的导出表,提取业务函数名称
//
// 📋 **参数说明**:
//   - wasmBytes: WASM字节码(通常从.wasm文件读取)
//
// 🔧 **返回值**:
//   - []string: 导出的函数名称列表(已过滤内部函数)
//   - error: 解析错误或WASM格式无效
//
// 📋 **过滤规则**:
//   - 过滤内存管理函数: malloc, calloc, realloc, free
//   - 过滤标准启动函数: _start, _initialize
//   - 过滤私有函数: 以下划线开头的函数名(除了导出的公开函数)
//
// 💡 **使用示例**:
//
//	service := NewIntrospectionService()
//	wasmBytes, _ := os.ReadFile("contract.wasm")
//	functions, err := service.ExtractExportedFunctions(wasmBytes)
//	// functions: ["Transfer", "GetBalance", "Mint", ...]
func (s *IntrospectionService) ExtractExportedFunctions(wasmBytes []byte) ([]string, error) {
	if len(wasmBytes) == 0 {
		return nil, fmt.Errorf("WASM字节码为空")
	}

	// 调试日志：记录字节码基本信息（长度）
	// 使用 Info 级别，确保在默认日志级别下也能看到
	corelog.Infof("[Introspect] 开始解析 WASM 导出函数, bytes_len=%d", len(wasmBytes))

	// ===== 解析导出函数名称（直接解析WASM Export Section，避免依赖运行时差异） =====
	rawNames, err := parseExportedFunctionNames(wasmBytes)

	// 如果手写解析失败，或者解析出的名称都为空字符串，尝试使用 wazero 作为备用方案
	useWazeroFallback := false
	if err != nil {
		corelog.Warnf("[Introspect] 手写解析失败，尝试使用 wazero 备用解析: %v", err)
		useWazeroFallback = true
	} else if len(rawNames) > 0 {
		// 检查是否有非空名称
		hasNonEmptyName := false
		for _, name := range rawNames {
			if name != "" {
				hasNonEmptyName = true
				break
			}
		}
		if !hasNonEmptyName {
			corelog.Warnf("[Introspect] 手写解析成功但所有导出函数名称为空，尝试使用 wazero 备用解析")
			useWazeroFallback = true
		}
	}

	if useWazeroFallback {
		rawNames, err = parseExportedFunctionNamesWithWazero(wasmBytes)
		if err != nil {
			return nil, fmt.Errorf("解析WASM导出函数名称失败（手写解析和wazero备用解析均失败）: %w", err)
		}
		corelog.Infof("[Introspect] 使用 wazero 备用解析成功，找到 %d 个导出函数", len(rawNames))
	}

	if len(rawNames) == 0 {
		corelog.Info("[Introspect] WASM 模块未导出任何函数 (Export Section 为空或无函数导出)")
	} else {
		corelog.Infof("[Introspect] WASM 模块原始导出函数总数: %d", len(rawNames))
		for _, name := range rawNames {
			corelog.Infof("[Introspect] WASM 原始导出函数: name=%s", name)
		}
	}

	// 定义需要过滤的内部函数(TinyGo/WASI标准函数)
	internalFunctions := map[string]bool{
		"malloc":      true,
		"calloc":      true,
		"realloc":     true,
		"free":        true,
		"_start":      true,
		"_initialize": true,
	}

	// 提取导出的函数名称
	var exports []string
	for _, funcName := range rawNames {
		// 过滤掉内部函数和以_开头的私有函数
		if funcName != "" && !internalFunctions[funcName] && !strings.HasPrefix(funcName, "_") {
			exports = append(exports, funcName)
		}
	}

	// 调试日志：打印过滤后的业务导出函数列表
	if len(exports) > 0 {
		corelog.Infof("[Introspect] 业务导出函数列表（过滤后）: %v", exports)
	} else {
		corelog.Info("[Introspect] 业务导出函数列表为空（过滤后无非内部导出函数）")
	}

	if len(exports) == 0 {
		// 错误日志：在返回错误前，记录详细信息，辅助排查
		corelog.Errorf("[Introspect] 未找到业务导出函数，可能原因：WASM 未正确导出业务函数或仅包含内部导出函数")
		return nil, fmt.Errorf("未找到业务导出函数(WASM文件可能未使用//export标记导出函数)")
	}

	return exports, nil
}

// ===========================
//  WASM Export Section 解析
// ===========================

// parseExportedFunctionNames 从WASM字节码中解析导出的“函数名称”列表
// 说明：
//   - 仅解析标准 Export Section (section id = 7)
//   - 只返回 kind = func(0x00) 的导出名称
//   - 不依赖运行时（wazero）的 ExportedFunctions 视图，避免不同运行时的实现差异
func parseExportedFunctionNames(wasm []byte) ([]string, error) {
	reader := bytes.NewReader(wasm)

	// 1. 校验魔数和版本
	var magic uint32
	if err := binary.Read(reader, binary.LittleEndian, &magic); err != nil {
		return nil, fmt.Errorf("读取WASM魔数失败: %w", err)
	}
	if magic != 0x6d736100 { // "\0asm" 小端
		return nil, fmt.Errorf("无效的WASM魔数: 0x%x", magic)
	}

	var version uint32
	if err := binary.Read(reader, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("读取WASM版本失败: %w", err)
	}
	// 当前不强制校验版本号（通常为 1）
	_ = version

	var exportedNames []string

	// 2. 遍历各个 section，找到 Export Section (id = 7)
	for {
		sectionID, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取section id失败: %w", err)
		}

		sectionSize, err := readVarUint32(reader)
		if err != nil {
			return nil, fmt.Errorf("读取section size失败: %w", err)
		}

		// 只关心 Export Section，其余跳过
		if sectionID != 7 {
			if _, err := reader.Seek(int64(sectionSize), io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("跳过section失败(id=%d): %w", sectionID, err)
			}
			continue
		}

		// 3. 解析 Export Section
		limitReader := io.LimitReader(reader, int64(sectionSize))
		buf := &bytes.Buffer{}
		if _, err := io.Copy(buf, limitReader); err != nil {
			return nil, fmt.Errorf("读取Export Section失败: %w", err)
		}

		secReader := bytes.NewReader(buf.Bytes())

		// 导出条目数量
		count, err := readVarUint32(secReader)
		if err != nil {
			return nil, fmt.Errorf("读取导出条目数量失败: %w", err)
		}

		for i := uint32(0); i < count; i++ {
			// 记录当前读取位置（用于调试）
			posBeforeNameLen, _ := secReader.Seek(0, io.SeekCurrent)

			// 名称长度 + 名称
			nameLen, err := readVarUint32(secReader)
			if err != nil {
				return nil, fmt.Errorf("读取导出名称长度失败: %w", err)
			}

			// 诊断日志：输出名称长度和原始字节
			posAfterNameLen, _ := secReader.Seek(0, io.SeekCurrent)
			corelog.Infof("[Introspect] 导出条目[%d]: name_len=%d, 位置偏移_before=%d, 位置偏移_after=%d", i, nameLen, posBeforeNameLen, posAfterNameLen)

			nameBytes := make([]byte, nameLen)
			if nameLen > 0 {
				if _, err := io.ReadFull(secReader, nameBytes); err != nil {
					return nil, fmt.Errorf("读取导出名称失败: name_len=%d, error=%w", nameLen, err)
				}
				// 诊断日志：输出名称的原始字节（十六进制）和 UTF-8 解码结果
				corelog.Infof("[Introspect] 导出条目[%d]: name_bytes_hex=%x, name_utf8=%q", i, nameBytes, string(nameBytes))
			} else {
				// 名称长度为 0，记录警告
				corelog.Warnf("[Introspect] 导出条目[%d]: 名称长度为 0，可能是解析错误", i)
			}
			name := string(nameBytes)

			// kind: 0x00=func, 0x01=table, 0x02=mem, 0x03=global
			kind, err := secReader.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("读取导出kind失败: %w", err)
			}

			// index (varuint32)，当前未使用，但需要跳过
			index, err := readVarUint32(secReader)
			if err != nil {
				return nil, fmt.Errorf("读取导出index失败: %w", err)
			}

			// 诊断日志：输出完整的导出条目信息
			corelog.Infof("[Introspect] 导出条目[%d]: name=%q, kind=0x%02x, index=%d", i, name, kind, index)

			if kind == 0x00 { // func
				exportedNames = append(exportedNames, name)
			}
		}

		// 已经解析完 Export Section，可以退出
		break
	}

	return exportedNames, nil
}

// readVarUint32 读取 WASM 中使用的 LEB128 编码的无符号32位整数
func readVarUint32(r *bytes.Reader) (uint32, error) {
	var result uint32
	var shift uint

	for {
		if shift >= 32 {
			return 0, fmt.Errorf("varuint32 过长")
		}

		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}

		result |= uint32(b&0x7F) << shift

		if (b & 0x80) == 0 {
			break
		}

		shift += 7
	}

	return result, nil
}

// parseExportedFunctionNamesWithWazero 使用 wazero 库解析导出函数名称（备用方案）
// 当手写解析失败时，使用此方法作为备用
func parseExportedFunctionNamesWithWazero(wasmBytes []byte) ([]string, error) {
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	// 编译WASM模块（不实例化，只解析）
	compiled, err := runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("wazero编译WASM模块失败: %w", err)
	}
	defer compiled.Close(ctx)

	var exportedNames []string
	for _, export := range compiled.ExportedFunctions() {
		funcName := export.Name()
		if funcName != "" {
			exportedNames = append(exportedNames, funcName)
		}
	}

	return exportedNames, nil
}

// ExtractExportedFunctionsFromFile 从WASM文件路径提取导出函数列表
//
// 🎯 **便捷方法**: 封装文件读取 + 解析的完整流程
//
// 📋 **参数说明**:
//   - wasmPath: WASM文件的完整路径
//
// 🔧 **返回值**:
//   - []string: 导出的函数名称列表
//   - error: 文件读取或解析错误
//
// 💡 **使用示例**:
//
//	service := NewIntrospectionService()
//	functions, err := service.ExtractExportedFunctionsFromFile("./hello_world.wasm")
//	// functions: ["SayHello", "GetGreeting", "SetMessage", ...]
func (s *IntrospectionService) ExtractExportedFunctionsFromFile(wasmPath string) ([]string, error) {
	// 读取WASM文件
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("读取WASM文件失败: %w", err)
	}

	// 调用字节码解析方法
	return s.ExtractExportedFunctions(wasmBytes)
}

// ValidateFunctionExists 验证WASM模块是否导出了指定函数
//
// 🎯 **校验功能**: 合约调用前验证函数存在性,提前发现错误
//
// 📋 **参数说明**:
//   - wasmBytes: WASM字节码
//   - functionName: 要验证的函数名称
//
// 🔧 **返回值**:
//   - bool: true表示函数存在,false表示不存在
//   - error: 解析错误
//
// 💡 **使用场景**:
//   - API/CLI调用合约前的参数校验
//   - 开发工具的智能提示与补全
//   - 测试框架的自动化验证
func (s *IntrospectionService) ValidateFunctionExists(wasmBytes []byte, functionName string) (bool, error) {
	functions, err := s.ExtractExportedFunctions(wasmBytes)
	if err != nil {
		return false, err
	}

	for _, fn := range functions {
		if fn == functionName {
			return true, nil
		}
	}

	return false, nil
}

// GetModuleInfo 获取WASM模块的完整信息(预留扩展)
//
// 🎯 **预留接口**: 未来可扩展为返回更多元信息
//
// 📋 **可能返回的信息**:
//   - 导出函数列表及签名
//   - 导入的宿主函数
//   - 内存配置
//   - 全局变量
//   - 自定义段(Custom Sections)
//
// 🔧 **当前实现**: 仅返回导出函数列表,后续根据需求扩展
type ModuleInfo struct {
	// 导出的函数列表
	ExportedFunctions []string `json:"exported_functions"`

	// 预留字段: 导入的宿主函数列表
	// ImportedFunctions []string `json:"imported_functions,omitempty"`

	// 预留字段: 内存配置
	// MemoryPages int `json:"memory_pages,omitempty"`

	// 预留字段: 自定义段
	// CustomSections map[string][]byte `json:"custom_sections,omitempty"`
}

// GetModuleInfo 获取WASM模块信息
func (s *IntrospectionService) GetModuleInfo(wasmBytes []byte) (*ModuleInfo, error) {
	functions, err := s.ExtractExportedFunctions(wasmBytes)
	if err != nil {
		return nil, err
	}

	return &ModuleInfo{
		ExportedFunctions: functions,
	}, nil
}

// ============================================================================
//                          包级别便捷函数
// ============================================================================

// ExtractExportedFunctions 从WASM文件提取导出函数 (包级别便捷函数)
//
// 🎯 **便捷封装**: 提供无需实例化服务的快捷调用方式
//
// 📋 **参数说明**:
//   - wasmPath: WASM文件路径
//
// 🔧 **返回值**:
//   - []string: 导出的函数名称列表
//   - error: 文件读取或解析错误
//
// 📝 **使用示例**:
//
//	functions, err := introspect.ExtractExportedFunctions("./contract.wasm")
//	if err != nil {
//	    log.Fatalf("解析失败: %v", err)
//	}
//	fmt.Printf("导出函数: %v\n", functions)
func ExtractExportedFunctions(wasmPath string) ([]string, error) {
	svc := NewIntrospectionService()
	return svc.ExtractExportedFunctionsFromFile(wasmPath)
}
