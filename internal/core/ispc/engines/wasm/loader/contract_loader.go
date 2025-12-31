package loader

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/weisyn/v1/internal/core/ispc/engines/wasm/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// ContractLoader 合约加载器
//
// 🎯 **核心职责**：从确定性路径加载WASM合约字节码（纯执行层）
//
// 负责将合约地址解析为内容哈希，然后通过确定性路径构建
// 从文件系统获取对应的WASM字节码，并进行基础的格式验证。
//
// 📋 **设计原则**：
// - 确定性路径构建：fileStoreRootPath + hash[:2] + hash
// - 内容寻址优先：路径由配置和内容哈希决定，无歧义
// - 安全验证：基础的WASM格式和安全性检查
// - 错误分类：详细的错误信息，便于问题定位
//
// ⚠️ **架构边界**：
// - ✅ engines层只负责"加载字节码 → 执行 → 返回结果"
// - ❌ 不关心合约是否在区块链上（这是TX层的职责）
// - ❌ 不验证UTXO状态（这是TX层的职责）
//
// 🔗 **依赖关系**：
// - log.Logger：日志记录
// - fileStoreRootPath：文件存储根路径（从配置注入）
type ContractLoader struct {
	logger            log.Logger
	fileStoreRootPath string // 文件存储根路径（从配置读取）

	// 预留：合约缓存优化（根据性能需求决定是否实现）
	// contractCache map[string]*types.WASMContract
	// cacheMutex    sync.RWMutex
}

// 确保ContractLoader实现interfaces.ContractLoader接口
var _ interfaces.ContractLoader = (*ContractLoader)(nil)

// NewContractLoader 创建合约加载器
//
// 🎯 **构造器模式**：通过依赖注入创建加载器实例
//
// 📋 **参数说明**：
//   - logger: 日志记录器
//   - fileStoreRootPath: 文件存储根路径（从配置读取）
//
// ⚠️ **架构边界**：
//   - engines层专注于"字节码加载和执行"
//   - 路径构建完全基于配置 + 内容哈希，无需资源管理器
//   - 区块链UTXO验证由TX层（call.go）负责
func NewContractLoader(
	logger log.Logger,
	fileStoreRootPath string,
) *ContractLoader {
	return &ContractLoader{
		logger:            logger,
		fileStoreRootPath: fileStoreRootPath,
	}
}

// LoadContract 根据合约ID（contentHash）加载字节码
//
// 🎯 **核心加载流程**：
//  1. 解析合约ID（64位十六进制字符串，表示32字节SHA-256哈希）
//  2. 从resourceManager查询资源信息（本地索引）
//  3. 从文件系统读取WASM字节码文件
//  4. 验证字节码格式和完整性
//  5. 构造WASMContract对象返回
//
// 📋 **合约ID格式**：
//   - 仅支持：64位十六进制字符串（32字节contentHash，严格不带0x前缀）
//   - 示例：d2ef233ef664052a09f1ca6e90b8319ab9f2b0e15d6b069069a8062619390a1b
//
// ⚠️ **架构边界**：
//   - 区块链UTXO验证由TX层（call.go）负责
//   - 此处专注于字节码加载，不关心合约是否在链上
//
// 🔧 **参数说明**：
//   - ctx: 调用上下文，用于超时控制和取消操作
//   - contractAddress: 合约ID（64位hex contentHash）
//
// 🔧 **返回值**：
//   - *types.WASMContract: 加载的WASM合约对象
//   - error: 加载过程中的错误信息
func (l *ContractLoader) LoadContract(ctx context.Context, contractAddress string) (*types.WASMContract, error) {
	if l.logger != nil {
		l.logger.Debug("开始加载WASM合约")
	}

	// 1. 解析合约标识符：仅支持内容哈希（64位十六进制字符串，严格不允许0x前缀）
	contentHash, err := l.parseContractAddress(contractAddress)
	if err != nil {
		return nil, fmt.Errorf("解析合约地址失败: %w", err)
	}

	// 2. 读取WASM字节码（直接从确定性路径）
	wasmBytes, err := l.readBytecodeFromStorage(contentHash)
	if err != nil {
		return nil, fmt.Errorf("读取WASM字节码失败，合约地址: %s, 错误: %v", contractAddress, err)
	}

	// 4. 验证WASM格式
	if err := l.validateWASMFormat(wasmBytes); err != nil {
		return nil, fmt.Errorf("WASM格式验证失败: %w", err)
	}

	// 5. 构造WASM合约对象
	contract := &types.WASMContract{
		Address:  contractAddress,
		Bytecode: wasmBytes,
	}

	if l.logger != nil {
		l.logger.Debugf("合约加载成功: %s (%d bytes)", contractAddress, len(wasmBytes))
	}

	return contract, nil
}

// parseContractAddress 解析合约地址为内容哈希
//
// 🎯 **标准化地址解析**：
// 严格要求64位十六进制字符串（32字节哈希），不允许0x前缀
//
// 参数：
//   - address: 合约地址字符串
//
// 返回：
//   - []byte: 解析后的内容哈希（32字节）
//   - error: 解析错误
func (l *ContractLoader) parseContractAddress(address string) ([]byte, error) {
	// 移除可能的空白字符
	address = strings.TrimSpace(address)

	// 严格拒绝 0x 前缀
	if strings.HasPrefix(address, "0x") || strings.HasPrefix(address, "0X") {
		return nil, fmt.Errorf("合约地址不允许0x前缀，请使用纯十六进制字符串: %s", address)
	}

	// 验证长度（32字节 = 64个十六进制字符）
	if len(address) != 64 {
		return nil, fmt.Errorf("合约地址长度必须为64位十六进制字符，实际长度: %d", len(address))
	}

	// 解析为字节数组
	contentHash, err := hex.DecodeString(address)
	if err != nil {
		return nil, fmt.Errorf("合约地址必须是有效的十六进制字符串: %w", err)
	}

	// 再次验证解析后的长度
	if len(contentHash) != 32 {
		return nil, fmt.Errorf("解析后的哈希长度必须为32字节，实际: %d", len(contentHash))
	}

	return contentHash, nil
}

// readBytecodeFromStorage 从文件系统读取WASM字节码
//
// 🎯 **统一路径构建**（使用公共函数）
//
// 核心原则：
// - 使用 utils.BuildContentAddressedPath() 统一路径构建
// - 确保系统中所有模块使用一致的路径策略
// - 简单、明确、唯一来源
//
// 路径构建公式（由 pkg/utils/path.go 统一定义）：
//
//	hashHex = hex.Encode(ContentHash)
//	relativePath = utils.BuildContentAddressedPath(hashHex)
//	fullPath = filepath.Join(fileStoreRootPath, relativePath)
//
// 示例：
//
//	ContentHash = [0xd2, 0xef, ...]
//	hashHex     = "d2ef233ef664052a09f1ca6e90b8319ab9f2b0e15d6b069069a8062619390a1b"
//	relativePath = "resources/d2/ef/d2ef233ef664052a09f1ca6e90b8319ab9f2b0e15d6b069069a8062619390a1b"
//	fullPath    = "data/files/resources/d2/ef/d2ef233ef664052a09f1ca6e90b8319ab9f2b0e15d6b069069a8062619390a1b"
//
// ✅ **架构优势**：
// - 唯一来源：所有路径构建调用同一个公共函数
// - 易维护：路径策略变更只需修改一处
// - 类型安全：统一的函数签名和返回值
//
// 📋 **路径来源**：
// - fileStoreRootPath: 从配置读取的存储根路径
// - ContentHash: 资源的内容哈希
func (l *ContractLoader) readBytecodeFromStorage(contentHash []byte) ([]byte, error) {
	// 🎯 **统一路径构建**（调用公共函数）
	//
	// 核心原则：
	// - 使用 utils.BuildContentAddressedPath() 确保唯一性
	// - 与存储层（tx/resource/file_storage.go）使用相同的路径构建逻辑
	// - 符合"唯一来源"原则
	//
	// 路径构建流程：
	// 1. 将 ContentHash 转换为十六进制字符串
	// 2. 调用 utils.BuildContentAddressedPath() 构建相对路径
	// 3. 与 fileStoreRootPath 组合得到完整物理路径

	// ⚠️ 修复：使用三级目录结构（与URES CAS一致）
	// URES CAS使用：{hash[0:2]}/{hash[2:4]}/{fullHash}
	hashHex := hex.EncodeToString(contentHash)
	dir1 := hashHex[0:2]  // "18"
	dir2 := hashHex[2:4]  // "c1"
	relativePath := filepath.Join(dir1, dir2, hashHex)
	storagePath := filepath.Join(l.fileStoreRootPath, relativePath)

	// 从文件系统读取WASM字节码
	wasmBytes, err := os.ReadFile(storagePath)
	if err != nil {
		// 详细的错误信息，便于调试
		return nil, fmt.Errorf("读取WASM文件失败\n"+
			"   ContentHash: %x\n"+
			"   ContentHashHex: %s\n"+
			"   Storage Path: %s\n"+
			"   FileStore Root: %s\n"+
			"   错误: %v\n"+
			"   建议操作：\n"+
			"   1. 检查文件是否存在：ls -la %s\n"+
			"   2. 检查配置根路径是否正确\n"+
			"   3. 如果文件不存在，请重新部署合约",
			contentHash,
			hashHex,
			storagePath,
			l.fileStoreRootPath,
			err,
			storagePath)
	}

	if l.logger != nil {
		l.logger.Debugf("从文件系统读取WASM字节码成功: %s (%d bytes)", storagePath, len(wasmBytes))
	}

	return wasmBytes, nil
}

// validateWASMFormat 验证WASM字节码格式
//
// 🎯 **基础格式验证**：
// 检查WASM字节码的基本格式，确保是有效的WebAssembly模块
//
// 参数：
//   - bytecode: WASM字节码数据
//
// 返回：
//   - error: 验证错误，如果格式正确则返回nil
func (l *ContractLoader) validateWASMFormat(bytecode []byte) error {
	// 检查最小长度
	if len(bytecode) < 8 {
		return fmt.Errorf("WASM字节码长度不足，至少需要8字节，实际: %d字节", len(bytecode))
	}

	// 检查WASM魔数: \0asm
	magic := bytecode[:4]
	expectedMagic := []byte{0x00, 0x61, 0x73, 0x6D}
	for i := 0; i < 4; i++ {
		if magic[i] != expectedMagic[i] {
			return fmt.Errorf("无效的WASM魔数: %x, 期望: %x", magic, expectedMagic)
		}
	}

	// 检查版本号
	version := bytecode[4:8]
	// WebAssembly 1.0: 版本号为 0x01 0x00 0x00 0x00
	expectedVersion := []byte{0x01, 0x00, 0x00, 0x00}
	for i := 0; i < 4; i++ {
		if version[i] != expectedVersion[i] {
			if l.logger != nil {
				l.logger.Warnf("WASM版本号不是1.0: %x, 期望: %x", version, expectedVersion)
			}
		}
	}

	return nil
}
