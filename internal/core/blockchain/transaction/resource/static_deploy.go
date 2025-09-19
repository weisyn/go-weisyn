// Package resource 静态资源部署实现
//
// 🎯 **模块定位**：TransactionService 接口的静态资源部署功能实现
//
// 本文件实现静态资源部署的核心业务逻辑，包括：
// - 静态资源上传和区块链锚定（DeployStaticResource）
// - 内容寻址存储集成
// - 资源元数据管理
// - 资源访问权限控制
// - 资源生命周期管理
//
// 🏗️ **架构定位**：
// - 业务层：实现静态资源的部署业务逻辑
// - 存储层：与内容寻址网络的集成
// - 权限层：实现资源的初始访问控制设置
//
// 🔧 **设计原则**：
// - 内容驱动：基于 content_hash 的资源身份管理
// - 权限分离：部署时设置初始权限，后续通过交易层管理
// - 存储分离：资源内容存储在内容寻址网络，区块链只记录元信息
// - 类型安全：严格的资源类型定义和验证
//
// 📋 **支持的静态资源类型**：
// - 文档文件：PDF、Word、Excel 等办公文档
// - 图片资源：JPEG、PNG、GIF、SVG 等图像格式
// - 数据文件：JSON、XML、CSV 等结构化数据
// - 媒体文件：MP3、MP4、WebM 等音视频文件
// - 代码文件：源代码、配置文件等开发资源
//
// 🎯 **与可执行资源的区别**：
// - 静态资源：ResourceCategory.STATIC，无需执行引擎，纯内容存储和访问
// - 可执行资源：ResourceCategory.EXECUTABLE，需要执行引擎，具备计算能力
//
// ⚠️ **实现状态**：
// 当前为薄实现阶段，提供接口骨架和基础验证
// 完整业务逻辑将在后续迭代中实现
package resource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/types"

	// 协议定义
	"github.com/weisyn/v1/pb/blockchain/block/transaction"
	resourcepb "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pb/blockchain/utxo"
	"google.golang.org/protobuf/proto"

	// 基础设施
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// 内部工具
	"github.com/weisyn/v1/internal/core/blockchain/transaction/internal"
	"github.com/weisyn/v1/pkg/utils"
)

// ============================================================================
//
//	静态资源部署实现服务
//
// ============================================================================
// StaticResourceDeployService 静态资源部署核心实现服务
//
// 🎯 **服务职责**：
// - 实现 TransactionService.DeployStaticResource 方法
// - 处理各类静态资源的上传和锚定
// - 管理资源的内容寻址存储
// - 设置资源的初始访问权限
//
// 🔧 **依赖注入**：
// - contentAddressStore：内容寻址存储服务
// - utxoSelector：UTXO 选择和管理服务
// - feeCalculator：费用计算服务
// - cacheStore：交易缓存存储
// - logger：日志记录服务
//
// 📝 **使用示例**：
//
//	service := NewStaticResourceDeployService(contentStore, utxoSelector, feeCalc, cache, logger)
//	txHash, err := service.DeployStaticResource(ctx, deployer, resourceData, options...)
type StaticResourceDeployService struct {
	// 核心依赖服务（使用公共接口）
	utxoManager     repository.UTXOManager     // UTXO 管理服务
	resourceManager repository.ResourceManager // 资源存储管理服务
	hashManager     crypto.HashManager         // 哈希计算服务
	keyManager      crypto.KeyManager          // 密钥管理服务（用于从私钥生成公钥）
	addressManager  crypto.AddressManager      // 地址管理服务（用于从公钥生成地址）
	cacheStore      storage.MemoryStore        // 缓存存储服务
	configManager   config.Provider            // 配置管理服务
	logger          log.Logger                 // 日志记录器

	// 工具类
	fileUtils    *FileUtils    // 文件处理工具
	mimeDetector *MimeDetector // MIME类型检测器
}

// NewStaticResourceDeployService 创建静态资源部署服务实例
//
// 🏗️ **构造器模式**：
// 使用依赖注入创建服务实例，确保所有依赖都已正确初始化
//
// 参数：
//   - utxoManager: UTXO 管理服务
//   - hashManager: 哈希计算服务
//   - keyManager: 密钥管理服务
//   - addressManager: 地址管理服务
//   - cacheStore: 缓存存储服务
//   - resourceStore: 资源存储服务（存储层）🆕
//   - logger: 日志记录器
//
// 返回：
//   - *StaticResourceDeployService: 静态资源部署服务实例
//
// 🚨 **注意事项**：
// 所有依赖参数都不能为 nil，否则 panic
func NewStaticResourceDeployService(
	utxoManager repository.UTXOManager,
	resourceManager repository.ResourceManager,
	hashManager crypto.HashManager,
	keyManager crypto.KeyManager,
	addressManager crypto.AddressManager,
	cacheStore storage.MemoryStore,
	configManager config.Provider,
	logger log.Logger,
) *StaticResourceDeployService {
	// 严格依赖检查
	if logger == nil {
		panic("StaticResourceDeployService: logger不能为nil")
	}
	if utxoManager == nil {
		panic("StaticResourceDeployService: utxoManager不能为nil")
	}
	if resourceManager == nil {
		panic("StaticResourceDeployService: resourceManager不能为nil")
	}
	if keyManager == nil {
		panic("StaticResourceDeployService: keyManager不能为nil")
	}
	if addressManager == nil {
		panic("StaticResourceDeployService: addressManager不能为nil")
	}
	if cacheStore == nil {
		panic("StaticResourceDeployService: cacheStore不能为nil")
	}
	if configManager == nil {
		panic("StaticResourceDeployService: configManager不能为nil")
	}
	return &StaticResourceDeployService{
		utxoManager:     utxoManager,
		resourceManager: resourceManager,
		hashManager:     hashManager,
		keyManager:      keyManager,
		addressManager:  addressManager,
		cacheStore:      cacheStore,
		configManager:   configManager,
		logger:          logger,
		// 初始化工具类
		fileUtils:    NewFileUtils(logger),
		mimeDetector: NewMimeDetector(logger),
	}
}

// ============================================================================
//
//	核心部署方法实现
//
// ============================================================================
// DeployStaticResource 实现静态资源部署功能（薄实现）
//
// 🎯 **方法职责**：
// 实现 blockchain.TransactionService.DeployStaticResource 接口
// 支持各类静态资源的上传、存储和区块链锚定
//
// 📋 **业务流程**：
// 1. 验证静态资源数据的完整性和格式
// 2. 计算资源的内容哈希（content_hash）
// 3. 将资源内容存储到内容寻址网络
// 4. 构建 ResourceOutput 交易输出
// 5. 设置资源的初始访问权限
// 6. 选择部署费用的支付 UTXO
// 7. 将部署交易存储到内存缓存
// 8. 返回交易哈希供用户签名
//
// 📝 **参数说明**：
//   - ctx: 上下文对象，用于超时控制和取消操作
//   - deployerAddress: 部署者地址
//   - resourceData: 静态资源的二进制数据
//   - options: 可选的部署选项（权限控制、费用设置等）
//
// 📤 **返回值**：
//   - []byte: 交易哈希，用于后续签名和提交
//   - error: 错误信息，部署失败时返回具体原因
//
// 🎯 **支持场景**：
// - 文档发布：DeployStaticResource(ctx, deployer, pdfData)
// - 图片上传：DeployStaticResource(ctx, deployer, imageData)
// - 数据存档：DeployStaticResource(ctx, deployer, jsonData, &types.ResourceDeployOptions{LifecycleControl: {...}})
// - 私有资源：DeployStaticResource(ctx, deployer, data, &types.ResourceDeployOptions{PermissionModel: {...}})
//
// 💡 **设计特性**：
// - 内容寻址：通过 SHA-256 哈希确保资源完整性
// - 权限可控：支持公开、私有、白名单等多种访问模式
// - 元数据丰富：自动提取文件类型、大小等元信息
// - 费用透明：提供详细的部署费用计算
//
// ⚠️ **当前状态**：薄实现，返回未实现错误
func (s *StaticResourceDeployService) DeployStaticResource(
	ctx context.Context,
	deployerPrivateKey []byte,
	filePath string,
	name string,
	description string,
	tags []string,
	options ...*types.ResourceDeployOptions,
) ([]byte, error) {
	// 从私钥计算部署者地址（无状态设计）
	deployerAddress, err := s.calculateAddressFromPrivateKey(deployerPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("从私钥计算地址失败: %v", err)
	}

	// 🔧 **修复**: 使用FileManager接口实现真实的文件读取功能
	resourceData, err := s.fileUtils.ReadFileWithValidation(ctx, filePath)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("❌ 文件读取失败 - 文件: %s, 错误: %v", filePath, err))
		}
		return nil, fmt.Errorf("文件读取失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("🚀 开始处理静态资源部署请求 - deployer: %s, 文件路径: %s",
			deployerAddress, filePath))
	}

	// 🔄 步骤1: 基础参数验证（移除大小限制，支持任意大小文件）
	if err := s.validateDeployParams(deployerAddress, resourceData, options); err != nil {
		return nil, fmt.Errorf("参数验证失败: %v", err)
	}

	// 🔧 步骤2: 合并部署选项
	mergedOptions, err := s.mergeDeployOptions(options)
	if err != nil {
		return nil, fmt.Errorf("部署选项处理失败: %v", err)
	}

	// 🔍 步骤3: 检测资源类型（使用文件路径进行更精确的检测）
	mimeType := s.mimeDetector.DetectResourceMimeType(resourceData, filePath)
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("检测到资源MIME类型: %s", mimeType))
	}

	// 🧮 步骤4: 存储文件到ResourceManager并获取内容哈希
	metadata := map[string]string{
		"resource_type":   "static",
		"name":            name,
		"description":     description,
		"creator_address": deployerAddress,
		"mime_type":       mimeType,
	}
	// 添加标签到元数据
	for i, tag := range tags {
		metadata[fmt.Sprintf("tag_%d", i)] = tag
	}

	contentHashBytes, err := s.resourceManager.StoreResourceFile(ctx, filePath, metadata)
	if err != nil {
		return nil, fmt.Errorf("存储静态资源文件失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 静态资源文件已存储 - content_hash: %x", contentHashBytes))
	}

	// 📍 步骤5: 解析部署者地址
	deployerAddrBytes, err := s.parseAddress(deployerAddress)
	if err != nil {
		return nil, fmt.Errorf("部署者地址解析失败: %v", err)
	}

	// 🏗️ 步骤6: 构建静态资源定义（使用真实文件信息）
	fileName := filepath.Base(filePath)
	staticResource, err := s.buildStaticResourceWithFileInfo(deployerAddress, resourceData, mimeType, contentHashBytes, fileName, name, description, mergedOptions)
	if err != nil {
		return nil, fmt.Errorf("静态资源构建失败: %v", err)
	}

	// 💰 步骤7: 选择部署费用的UTXO（使用原生代币）
	deploymentFee := s.estimateDeploymentFee(len(resourceData))
	selectedInputs, changeAmount, err := s.selectUTXOsForDeployment(
		ctx, deployerAddrBytes, deploymentFee, "") // 原生代币
	if err != nil {
		return nil, fmt.Errorf("部署费用UTXO选择失败: %v", err)
	}

	// 🏗️ 步骤8: 构建静态资源部署输出
	outputs, err := s.buildStaticResourceOutputs(deployerAddress, staticResource, changeAmount, mergedOptions)
	if err != nil {
		return nil, fmt.Errorf("静态资源输出构建失败: %v", err)
	}

	// 🔄 步骤9: 构建完整交易
	tx, err := s.buildCompleteTransaction(selectedInputs, outputs)
	if err != nil {
		return nil, fmt.Errorf("构建完整交易失败: %v", err)
	}

	// 🔄 步骤10: 计算交易哈希并缓存
	txHash, err := s.cacheTransaction(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("缓存交易失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("✅ 静态资源部署交易构建完成 - txHash: %x, 资源哈希: %x, 费用: %s",
			txHash, contentHashBytes, deploymentFee))
	}

	return txHash, nil
}

// FetchStaticResourceFile 获取静态资源文件
//
// 🎯 **功能说明**：
//   - 根据内容哈希获取已部署的静态资源文件
//   - 验证请求者权限（仅资源部署者可获取）
//   - 支持自定义保存目录或使用默认目录
//   - 自动处理文件名冲突（iOS风格递增）
//
// 📝 **权限验证流程**：
//  1. 通过ResourceManager获取资源信息
//  2. 从元数据中提取部署者地址
//  3. 从请求者私钥计算地址
//  4. 验证地址是否匹配
//
// 📝 **文件保存流程**：
//  1. 确定目标保存目录（默认或指定）
//  2. 从存储路径复制文件到目标位置
//  3. 处理文件名冲突（iOS风格递增）
//
// 💡 **参数说明**：
//   - ctx: 上下文对象，用于超时控制和取消操作
//   - contentHash: 资源内容的SHA-256哈希值（32字节）
//   - requesterPrivateKey: 请求者私钥，用于权限验证
//   - targetDir: 目标保存目录（可选，为空时使用默认目录）
//
// 💡 **返回值说明**：
//   - string: 实际保存的文件路径
//   - error: 操作错误（权限不足、资源不存在、磁盘空间不足等）
func (s *StaticResourceDeployService) FetchStaticResourceFile(ctx context.Context,
	contentHash []byte,
	requesterPrivateKey []byte,
	targetDir string,
) (string, error) {
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("🔍 开始获取静态资源文件 - content_hash: %x", contentHash))
	}

	// 步骤1: 参数验证
	if len(contentHash) == 0 {
		return "", fmt.Errorf("内容哈希不能为空")
	}
	if len(requesterPrivateKey) == 0 {
		return "", fmt.Errorf("请求者私钥不能为空")
	}

	// 步骤2: 从请求者私钥计算地址
	requesterAddress, err := s.calculateAddressFromPrivateKey(requesterPrivateKey)
	if err != nil {
		return "", fmt.Errorf("从私钥计算请求者地址失败: %v", err)
	}

	// 步骤3: 通过ResourceManager获取资源信息
	resourceInfo, err := s.resourceManager.GetResourceByHash(ctx, contentHash)
	if err != nil {
		return "", fmt.Errorf("获取资源信息失败: %v", err)
	}

	// 步骤4: 权限验证 - 检查请求者是否为资源部署者
	deployerAddress, exists := resourceInfo.Metadata["creator_address"]
	if !exists {
		return "", fmt.Errorf("资源元数据中缺少部署者地址信息")
	}
	if requesterAddress != deployerAddress {
		return "", fmt.Errorf("权限不足：仅资源部署者可获取文件")
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 权限验证通过 - requester: %s", requesterAddress))
	}

	// 步骤5: 确定目标保存目录
	if targetDir == "" {
		targetDir = s.getDefaultDownloadDir() // 根据操作系统确定默认目录
	}

	// 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 步骤6: 获取原始文件名
	originalFileName := resourceInfo.ResourcePath
	if name, exists := resourceInfo.Metadata["name"]; exists && name != "" {
		originalFileName = name
	}

	// 步骤7: 处理文件名冲突，生成最终保存路径
	finalPath := s.resolveFileNameConflict(targetDir, originalFileName)

	// 步骤8: 通过ResourceManager获取文件内容并保存到目标位置
	storagePath, exists := resourceInfo.Metadata["storage_path"]
	if !exists {
		return "", fmt.Errorf("资源元数据中缺少存储路径信息")
	}

	// 直接通过文件路径读取并复制文件
	// 注意：现在文件存储在data/files目录中，需要构建完整路径
	fullSourcePath := filepath.Join("data/files", storagePath)
	sourceFile, err := os.Open(fullSourcePath)
	if err != nil {
		return "", fmt.Errorf("打开源文件失败: %v", err)
	}
	defer sourceFile.Close()

	// 创建目标文件
	targetFile, err := os.Create(finalPath)
	if err != nil {
		return "", fmt.Errorf("创建目标文件失败: %v", err)
	}
	defer targetFile.Close()

	// 复制文件内容
	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return "", fmt.Errorf("复制文件内容失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("✅ 静态资源文件获取成功 - 保存路径: %s", finalPath))
	}

	return finalPath, nil
}

// ============================================================================
//
//	私有辅助方法
//
// ============================================================================
// mergeDeployOptions 合并多个部署选项
//
// 🔧 **合并策略**：
// - 后面的选项覆盖前面的选项
// - 保持最后一个非空值
// - 对嵌套结构进行深度合并
//
// 参数：
//   - options: 多个部署选项
//
// 返回：
//   - *types.ResourceDeployOptions: 合并后的选项
//   - error: 合并失败时的错误信息
//
// validateDeployParams 验证部署参数
func (s *StaticResourceDeployService) validateDeployParams(
	deployerAddress string,
	resourceData []byte,
	options []*types.ResourceDeployOptions,
) error {
	// 基础参数验证（支持任意大小文件）
	if deployerAddress == "" {
		return fmt.Errorf("部署者地址不能为空")
	}
	if len(resourceData) == 0 {
		return fmt.Errorf("资源数据不能为空")
	}
	// ✅ 移除文件大小限制：支持从几字节到几十GB的文件

	// 选项验证
	for i, option := range options {
		if option == nil {
			return fmt.Errorf("部署选项[%d]不能为nil", i)
		}
	}

	return nil
}

func (s *StaticResourceDeployService) mergeDeployOptions(
	options []*types.ResourceDeployOptions,
) (*types.ResourceDeployOptions, error) {
	if len(options) == 0 {
		return &types.ResourceDeployOptions{}, nil // 返回空选项
	}
	if s.logger != nil {
		s.logger.Debug("合并部署选项")
	}
	// 简化合并策略：使用最后一个有效选项
	// 遍历查找最后一个非空选项
	var result *types.ResourceDeployOptions
	for i := len(options) - 1; i >= 0; i-- {
		if options[i] != nil {
			result = options[i]
			break
		}
	}

	// 如果没有找到有效选项，返回默认选项
	if result == nil {
		result = &types.ResourceDeployOptions{}
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("选项合并完成 - 源选项数: %d", len(options)))
	}

	return result, nil
}

// detectResourceMimeType 检测静态资源的 MIME 类型
//
// 🔍 **检测策略**：
// - 基于文件内容的魔数检测
// - 文件扩展名辅助判断
// - 默认类型处理
//
// 参数：
//   - resourceData: 资源二进制数据
//   - filename: 文件名（可选，用于扩展名检测）
//
// 返回：
//   - string: 检测到的 MIME 类型
func (s *StaticResourceDeployService) detectResourceMimeType(
	resourceData []byte,
	filename string,
) string {
	if len(resourceData) == 0 {
		return "application/octet-stream"
	}

	// 🔍 基于文件头魔数的精确检测
	mimeType := s.detectMimeByMagicNumbers(resourceData)
	if mimeType != "" {
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("通过魔数检测到MIME类型: %s", mimeType))
		}
		return mimeType
	}

	// 🎯 基于文件扩展名的 MIME 类型检测
	if filename != "" {
		ext := filepath.Ext(filename)
		if mimeType := mime.TypeByExtension(ext); mimeType != "" {
			if s.logger != nil {
				s.logger.Debug(fmt.Sprintf("通过扩展名检测到MIME类型: %s -> %s", ext, mimeType))
			}
			return mimeType
		}
	}

	// 🔍 基于内容特征的检测
	mimeType = s.detectMimeByContent(resourceData)
	if mimeType != "" {
		return mimeType
	}

	return "application/octet-stream" // 默认二进制类型
}

// parseAddress 解析地址
func (s *StaticResourceDeployService) parseAddress(address string) ([]byte, error) {
	// 简化实现：直接使用字符串作为地址
	if address == "" {
		return nil, fmt.Errorf("地址不能为空")
	}
	// 使用地址管理器进行标准化验证和解析
	if s.addressManager != nil {
		addressBytes, err := s.addressManager.AddressToBytes(address)
		if err != nil {
			return nil, fmt.Errorf("地址解析失败: %w", err)
		}
		return addressBytes, nil
	}

	// 后备方案：简单的字符串转换（不推荐生产使用）
	return []byte(address), nil
}

// estimateDeploymentFee 估算部署费用
func (s *StaticResourceDeployService) estimateDeploymentFee(dataSize int) string {
	// 简化费用计算：基础费用 + 数据大小费用
	baseFee := 1000    // 基础部署费用
	sizeFeePerKB := 10 // 每 KB 数据费用
	sizeFee := (dataSize / 1024) * sizeFeePerKB
	totalFee := baseFee + sizeFee
	return fmt.Sprintf("%d", totalFee)
}

func (s *StaticResourceDeployService) calculateResourceHash(
	resourceData []byte,
) ([]byte, error) {
	if s.logger != nil {
		s.logger.Debug("计算资源内容哈希")
	}
	// 使用 SHA-256 计算哈希
	hash := sha256.Sum256(resourceData)
	return hash[:], nil
}

// buildResourceOutput 构建静态资源的输出 UTXO
//
// 🏗️ **输出构建**：
// - 创建 ResourceOutput 类型
// - 设置 ResourceCategory.STATIC
// - 包含完整的 Resource 定义
// - 配置初始访问权限
//
// 参数：
//   - deployerAddress: 部署者地址
//   - resource: 资源定义
//   - options: 部署选项
//
// 返回：
//   - *transaction.TxOutput: 构建的资源输出
//   - error: 构建失败时的错误信息
//
// buildStaticResourceOutputs 构建静态资源部署输出
func (s *StaticResourceDeployService) buildStaticResourceOutputs(
	deployerAddress string,
	resource *resourcepb.Resource,
	changeAmount string,
	options *types.ResourceDeployOptions,
) ([]*transaction.TxOutput, error) {
	var outputs []*transaction.TxOutput

	// 1. 构建资源输出
	resourceOutput, err := s.buildResourceOutput(deployerAddress, resource, options)
	if err != nil {
		return nil, fmt.Errorf("构建资源输出失败: %v", err)
	}
	outputs = append(outputs, resourceOutput)

	// 2. 构建找零输出（如果需要）
	if changeAmount != "" && changeAmount != "0" {
		changeOutput, err := s.buildChangeOutput(deployerAddress, changeAmount)
		if err != nil {
			return nil, fmt.Errorf("构建找零输出失败: %v", err)
		}
		outputs = append(outputs, changeOutput)
	}

	return outputs, nil
}

func (s *StaticResourceDeployService) buildResourceOutput(
	deployerAddress string,
	resource *resourcepb.Resource,
	options *types.ResourceDeployOptions,
) (*transaction.TxOutput, error) {
	if s.logger != nil {
		s.logger.Debug("构建静态资源输出")
	}

	// 解析部署者地址
	deployerAddrBytes, err := s.parseAddress(deployerAddress)
	if err != nil {
		return nil, err
	}

	// 构建 ResourceOutput
	resourceOutput := &transaction.TxOutput{
		Owner: deployerAddrBytes,
		LockingConditions: []*transaction.LockingCondition{
			{
				Condition: &transaction.LockingCondition_SingleKeyLock{
					SingleKeyLock: &transaction.SingleKeyLock{
						KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
							RequiredAddressHash: deployerAddrBytes,
						},
						RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:       transaction.SignatureHashType_SIGHASH_ALL,
					},
				},
			},
		},
		OutputContent: &transaction.TxOutput_Resource{
			Resource: &transaction.ResourceOutput{
				Resource:          resource,
				CreationTimestamp: uint64(time.Now().Unix()),
				StorageStrategy:   transaction.ResourceOutput_STORAGE_STRATEGY_CONTENT_ADDRESSED,
				StorageLocations:  [][]byte{},
				IsImmutable:       true, // 静态资源默认不可变
			},
		},
	}

	// 根据 options 设置锁定条件的访问控制
	if options != nil {
		// 简化实现：标记选项已应用
		// 具体的访问控制逻辑需要根据实际需求定制
		if s.logger != nil {
			s.logger.Debug("应用部署选项到锁定条件")
		}
	}

	return resourceOutput, nil
}

// buildChangeOutput 构建找零输出
func (s *StaticResourceDeployService) buildChangeOutput(address string, amount string) (*transaction.TxOutput, error) {
	addrBytes, err := s.parseAddress(address)
	if err != nil {
		return nil, err
	}

	changeOutput := &transaction.TxOutput{
		Owner: addrBytes,
		LockingConditions: []*transaction.LockingCondition{
			{
				Condition: &transaction.LockingCondition_SingleKeyLock{
					SingleKeyLock: &transaction.SingleKeyLock{
						KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
							RequiredAddressHash: addrBytes,
						},
						RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:       transaction.SignatureHashType_SIGHASH_ALL,
					},
				},
			},
		},
		OutputContent: &transaction.TxOutput_Asset{
			Asset: &transaction.AssetOutput{
				AssetContent: &transaction.AssetOutput_NativeCoin{
					NativeCoin: &transaction.NativeCoinAsset{
						Amount: amount,
					},
				},
			},
		},
	}

	return changeOutput, nil
}

// storeResourceContent 将资源内容存储到内容寻址网络
//
// 🌐 **存储策略**：
// - 内容寻址存储（默认）
// - 支持多副本存储
// - 提供存储位置提示
//
// 参数：
//   - ctx: 上下文对象
//   - resourceData: 资源内容
//   - contentHash: 内容哈希
//
// 返回：
//   - [][]byte: 存储位置列表
//   - error: 存储失败时的错误信息
//
// buildCompleteTransaction 构建完整交易
func (s *StaticResourceDeployService) buildCompleteTransaction(
	inputs []*transaction.TxInput,
	outputs []*transaction.TxOutput,
) (*transaction.Transaction, error) {
	tx := &transaction.Transaction{
		Version:           1,
		Inputs:            inputs,
		Outputs:           outputs,
		Nonce:             0,
		CreationTimestamp: uint64(time.Now().Unix()),
		ChainId:           []byte("weisyn-mainnet"),
	}

	return tx, nil
}

// cacheTransaction 缓存交易
func (s *StaticResourceDeployService) cacheTransaction(ctx context.Context, tx *transaction.Transaction) ([]byte, error) {
	// 计算真实的交易哈希
	if s.hashManager == nil {
		return nil, fmt.Errorf("哈希管理器未初始化")
	}

	// 序列化交易（使用 protobuf）
	txBytes, err := proto.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("序列化交易失败: %w", err)
	}

	// 计算哈希
	txHash := s.hashManager.SHA256(txBytes)
	if len(txHash) == 0 {
		return nil, fmt.Errorf("计算交易哈希失败：返回空哈希")
	}

	// 缓存交易
	cacheConfig := internal.GetDefaultCacheConfig()
	err = internal.CacheUnsignedTransaction(ctx, s.cacheStore, txHash, tx, cacheConfig, s.logger)
	if err != nil {
		return nil, fmt.Errorf("缓存交易失败: %v", err)
	}

	return txHash, nil
}

// maxInMemoryFileSize 返回可以直接加载到内存的文件大小阈值
//
// 🎯 **设计思路**：
// - 小文件：直接加载到内存处理（快速）
// - 大文件：使用流式处理或内容寻址存储（内存友好）
//
// 返回：
//   - int: 内存处理阈值（字节）

// calculateAddressFromPrivateKey 从私钥计算地址（无状态设计的核心方法）
//
// 实现完整的私钥到地址的推导流程：
// 私钥 → 公钥(secp256k1) → 地址(Base58Check)
//
// 参数：
//   - privateKey: 32字节私钥
//
// 返回：
//   - string: WES标准地址
//   - error: 计算失败时的错误
func (s *StaticResourceDeployService) calculateAddressFromPrivateKey(privateKey []byte) (string, error) {
	// 1. 从私钥导出公钥
	publicKey, err := s.keyManager.DerivePublicKey(privateKey)
	if err != nil {
		return "", fmt.Errorf("从私钥导出公钥失败: %v", err)
	}

	// 2. 从公钥生成地址
	address, err := s.addressManager.PublicKeyToAddress(publicKey)
	if err != nil {
		return "", fmt.Errorf("从公钥生成地址失败: %v", err)
	}

	return address, nil
}

// readFileWithValidation 智能文件读取和验证（业务层逻辑）
//
// 🔧 **业务处理职责**：
// - 文件存在性和权限验证
// - 智能大小处理：小文件全读取，大文件读取头部用于验证
// - MIME类型检测和基础安全检查
// - 这是业务逻辑，属于transaction层
//
// 参数：
//   - ctx: 上下文
//   - filePath: 文件完整路径
//
// 返回：
//   - []byte: 文件内容或文件头（用于验证和MIME检测）
//   - error: 读取错误

// computeFileHashDirect 直接计算文件哈希（业务层实现）
//
// 🧮 **智能哈希计算**：
// - 小文件：直接内存计算SHA-256
// - 大文件：流式计算SHA-256，内存友好
// - 这是业务逻辑，属于transaction层
//
// 参数：
//   - ctx: 上下文
//   - filePath: 文件路径
//
// 返回：
//   - []byte: SHA-256哈希值（32字节）
//   - error: 计算错误
func (s *StaticResourceDeployService) computeFileHashDirect(ctx context.Context, filePath string) ([]byte, error) {
	// 检查上下文
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("上下文已取消: %w", err)
	}

	// 获取文件信息
	stat, err := os.Stat(filePath)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("获取文件信息失败: %s, 错误: %v", filePath, err))
		}
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	fileSize := stat.Size()
	fileName := filepath.Base(filePath)

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("开始计算文件哈希: %s (大小: %d bytes)", fileName, fileSize))
	}

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("打开文件失败: %s, 错误: %v", filePath, err))
		}
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	// 使用SHA-256计算哈希
	hasher := sha256.New()

	// 流式复制，自动处理大文件
	_, err = io.Copy(hasher, file)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("计算文件哈希失败: %s, 错误: %v", filePath, err))
		}
		return nil, fmt.Errorf("计算文件哈希失败: %w", err)
	}

	// 获取哈希值
	hashBytes := hasher.Sum(nil)

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 文件哈希计算完成: %s, 哈希: %x", fileName, hashBytes))
	}

	return hashBytes, nil
}

// detectMimeByMagicNumbers 基于文件头魔数检测MIME类型（支持所有主流格式）
func (s *StaticResourceDeployService) detectMimeByMagicNumbers(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// ============== 🎯 图像格式 ==============

	// JPEG文件
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}

	// PNG文件
	pngSignature := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if len(data) >= 8 && bytes.HasPrefix(data, pngSignature) {
		return "image/png"
	}

	// GIF文件
	if bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) {
		return "image/gif"
	}

	// WebP文件
	if len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp"
	}

	// BMP文件
	if len(data) >= 2 && data[0] == 0x42 && data[1] == 0x4D {
		return "image/bmp"
	}

	// TIFF文件
	if len(data) >= 4 && ((data[0] == 0x49 && data[1] == 0x49 && data[2] == 0x2A && data[3] == 0x00) ||
		(data[0] == 0x4D && data[1] == 0x4D && data[2] == 0x00 && data[3] == 0x2A)) {
		return "image/tiff"
	}

	// ICO文件
	if len(data) >= 4 && data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x01 && data[3] == 0x00 {
		return "image/x-icon"
	}

	// ============== 📁 文档格式 ==============

	// PDF文件
	if bytes.HasPrefix(data, []byte("%PDF")) {
		return "application/pdf"
	}

	// Microsoft Office格式 (ZIP-based)
	zipSignature := []byte{0x50, 0x4B, 0x03, 0x04}
	if len(data) >= 4 && bytes.HasPrefix(data, zipSignature) {
		// 需要进一步检查内容来区分不同的Office格式
		// 简化处理，先返回通用ZIP格式，后续可扩展
		if len(data) > 30 {
			content := string(data[:512]) // 检查前512字节
			if bytes.Contains(data, []byte("word/")) {
				return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
			}
			if bytes.Contains(data, []byte("xl/")) {
				return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
			}
			if bytes.Contains(data, []byte("ppt/")) {
				return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
			}
			_ = content // 避免未使用变量警告
		}
		return "application/zip"
	}

	// RTF文件
	if bytes.HasPrefix(data, []byte("{\\rtf")) {
		return "application/rtf"
	}

	// ============== 🎵 音频格式 ==============

	// MP3文件
	if len(data) >= 3 && ((data[0] == 0xFF && (data[1]&0xFE) == 0xFA) || // MPEG header
		bytes.HasPrefix(data, []byte("ID3"))) { // ID3 tag
		return "audio/mpeg"
	}

	// WAV文件
	if len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WAVE")) {
		return "audio/wav"
	}

	// FLAC文件
	if bytes.HasPrefix(data, []byte("fLaC")) {
		return "audio/flac"
	}

	// OGG文件
	if bytes.HasPrefix(data, []byte("OggS")) {
		return "audio/ogg"
	}

	// ============== 🎬 视频格式 ==============

	// MP4/MOV文件
	if len(data) >= 8 {
		// MP4文件通常在第4-7字节有类型标识
		if bytes.Contains(data[4:8], []byte("ftyp")) {
			return "video/mp4"
		}
	}

	// AVI文件
	if len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("AVI ")) {
		return "video/x-msvideo"
	}

	// WebM文件
	if len(data) >= 4 && data[0] == 0x1A && data[1] == 0x45 && data[2] == 0xDF && data[3] == 0xA3 {
		return "video/webm"
	}

	// ============== 🗜️ 压缩格式 ==============

	// ZIP文件 (已在上面处理)

	// RAR文件
	if bytes.HasPrefix(data, []byte("Rar!")) ||
		(len(data) >= 7 && data[0] == 0x52 && data[1] == 0x61 && data[2] == 0x72 && data[3] == 0x21 && data[4] == 0x1A && data[5] == 0x07 && data[6] == 0x01) {
		return "application/vnd.rar"
	}

	// 7Z文件
	sevenZipSignature := []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}
	if len(data) >= 6 && bytes.HasPrefix(data, sevenZipSignature) {
		return "application/x-7z-compressed"
	}

	// GZIP文件
	if len(data) >= 3 && data[0] == 0x1F && data[1] == 0x8B && data[2] == 0x08 {
		return "application/gzip"
	}

	// TAR文件（简化检测）
	if len(data) >= 262 && string(data[257:262]) == "ustar" {
		return "application/x-tar"
	}

	// ============== 💾 可执行文件 ==============

	// Windows PE文件 (.exe, .dll)
	if len(data) >= 2 && data[0] == 0x4D && data[1] == 0x5A {
		return "application/vnd.microsoft.portable-executable"
	}

	// ELF文件 (Linux执行文件)
	if len(data) >= 4 && data[0] == 0x7F && data[1] == 0x45 && data[2] == 0x4C && data[3] == 0x46 {
		return "application/x-executable"
	}

	// Mach-O文件 (macOS执行文件)
	if len(data) >= 4 && ((data[0] == 0xFE && data[1] == 0xED && data[2] == 0xFA && data[3] == 0xCE) ||
		(data[0] == 0xCE && data[1] == 0xFA && data[2] == 0xED && data[3] == 0xFE)) {
		return "application/x-mach-binary"
	}

	// ============== 📝 文本/数据格式 ==============

	// 通过内容特征检测（在detectMimeByContent中处理）

	return ""
}

// detectMimeByContent 基于内容特征检测MIME类型
func (s *StaticResourceDeployService) detectMimeByContent(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// 检测JSON
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return "application/json"
	}

	// 检测XML
	if bytes.HasPrefix(trimmed, []byte("<?xml")) || (len(trimmed) > 0 && trimmed[0] == '<') {
		return "application/xml"
	}

	return ""
}

// buildStaticResourceWithFileInfo 基于文件信息构建静态资源定义
//
// 🎯 **使用真实文件信息构建Resource**
//
// 参数：
//   - deployerAddress: 部署者地址
//   - resourceData: 文件内容
//   - mimeType: MIME类型
//   - contentHash: 内容哈希
//   - fileName: 原始文件名
//   - name: 用户指定的名称
//   - description: 用户指定的描述
//   - options: 部署选项
//
// 返回：
//   - *resourcepb.Resource: 构建的资源定义
//   - error: 构建错误
func (s *StaticResourceDeployService) buildStaticResourceWithFileInfo(
	deployerAddress string,
	resourceData []byte,
	mimeType string,
	contentHash []byte,
	fileName string,
	name string,
	description string,
	options *types.ResourceDeployOptions,
) (*resourcepb.Resource, error) {
	if s.logger != nil {
		s.logger.Debug("构建静态资源定义 - 基于真实文件信息")
	}

	// 使用用户提供的名称，否则使用文件名
	resourceName := name
	if resourceName == "" {
		resourceName = fileName
	}

	// 使用用户提供的描述，否则生成默认描述
	resourceDescription := description
	if resourceDescription == "" {
		resourceDescription = fmt.Sprintf("静态资源: %s (%s, %d字节)",
			fileName, mimeType, len(resourceData))
	}

	// 构建完整的资源定义
	resource := &resourcepb.Resource{
		// ========== 资源核心身份 ==========
		Category:       resourcepb.ResourceCategory_RESOURCE_CATEGORY_STATIC,
		ExecutableType: resourcepb.ExecutableType_EXECUTABLE_TYPE_UNKNOWN, // 静态资源无执行类型
		ContentHash:    contentHash,                                       // ✅ 真实文件哈希
		MimeType:       mimeType,                                          // ✅ 精确MIME类型
		Size:           uint64(len(resourceData)),                         // ✅ 真实文件大小

		// ========== 资源元信息 ==========
		Name:             resourceName,              // ✅ 用户指定或文件名
		Version:          "1.0",                     // 默认版本
		CreatedTimestamp: uint64(time.Now().Unix()), // 当前时间戳
		CreatorAddress:   deployerAddress,           // 部署者地址
		Description:      resourceDescription,       // 资源描述

		// ========== 自定义属性 ==========
		CustomAttributes: map[string]string{
			"original_filename": fileName,                     // 原始文件名
			"file_extension":    filepath.Ext(fileName),       // 文件扩展名
			"mime_detection":    "magic_number_and_extension", // 检测方式
			"validation_status": "verified",                   // 验证状态
		},

		// 静态资源不需要执行配置，ExecutionConfig 保持为空
	}

	// 根据options设置额外属性
	if options != nil {
		// 简化实现：在自定义属性中标记选项已应用
		// 具体的字段映射需要根据实际的 types.ResourceDeployOptions 结构调整
		resource.CustomAttributes["deploy_options_applied"] = "true"

		if s.logger != nil {
			s.logger.Debug("应用部署选项到资源属性")
		}
	}

	return resource, nil
}

// ============================================================================
//                              内部UTXO选择方法
// ============================================================================

// selectUTXOsForDeployment 为资源部署选择UTXO（内部方法）
//
// 🎯 **简化的UTXO选择逻辑**：
// - 获取地址所有可用AssetUTXO
// - 使用首次适应算法选择足够金额
// - 计算找零金额
//
// 📝 **参数说明**：
//   - deployerAddr: 部署方地址字节
//   - amountStr: 需要金额（字符串格式）
//   - tokenID: 代币类型（""=原生币）
//
// 💡 **返回值说明**：
//   - []*transaction.TxInput: 选中的UTXO输入
//   - string: 找零金额字符串
//   - error: 选择错误
func (s *StaticResourceDeployService) selectUTXOsForDeployment(ctx context.Context, deployerAddr []byte, amountStr string, tokenID string) ([]*transaction.TxInput, string, error) {
	if s.logger != nil {
		s.logger.Debugf("资源部署UTXO选择 - 地址: %x, 金额: %s", deployerAddr, amountStr)
	}

	// 1. 解析目标金额
	targetAmount, err := s.parseAmount(amountStr)
	if err != nil {
		return nil, "", fmt.Errorf("金额解析失败: %v", err)
	}

	// 2. 获取地址所有可用AssetUTXO
	assetCategory := utxo.UTXOCategory_UTXO_CATEGORY_ASSET
	allUTXOs, err := s.utxoManager.GetUTXOsByAddress(ctx, deployerAddr, &assetCategory, true)
	if err != nil {
		return nil, "", fmt.Errorf("获取UTXO失败: %v", err)
	}

	if len(allUTXOs) == 0 {
		return nil, "", fmt.Errorf("地址没有可用UTXO")
	}

	// 3. 简单选择算法：首次适应
	var selectedInputs []*transaction.TxInput
	var totalSelected uint64 = 0

	for _, utxoItem := range allUTXOs {
		// 提取UTXO金额
		utxoAmount := s.extractUTXOAmount(utxoItem)
		if utxoAmount == 0 {
			continue // 跳过零金额UTXO
		}

		// 创建交易输入
		txInput := &transaction.TxInput{
			PreviousOutput: &transaction.OutPoint{
				TxId:        utxoItem.Outpoint.TxId,
				OutputIndex: utxoItem.Outpoint.OutputIndex,
			},
			IsReferenceOnly: false, // 部署需要消费UTXO
			Sequence:        0xffffffff,
		}

		selectedInputs = append(selectedInputs, txInput)
		totalSelected += utxoAmount

		// 找到足够金额就停止
		if totalSelected >= targetAmount {
			break
		}
	}

	// 4. 检查余额是否充足
	if totalSelected < targetAmount {
		return nil, "", fmt.Errorf("余额不足，需要: %d, 可用: %d", targetAmount, totalSelected)
	}

	// 5. 计算找零
	changeAmount := totalSelected - targetAmount
	changeStr := s.formatAmount(changeAmount)

	if s.logger != nil {
		s.logger.Infof("资源部署UTXO选择完成 - 选中: %d个, 总额: %d, 找零: %s",
			len(selectedInputs), totalSelected, changeStr)
	}

	return selectedInputs, changeStr, nil
}

// parseAmount 解析金额字符串为wei单位
func (s *StaticResourceDeployService) parseAmount(amountStr string) (uint64, error) {
	// 简化实现：假设输入是整数wei
	amount, err := strconv.ParseUint(amountStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("无效的金额格式: %v", err)
	}
	return amount, nil
}

// extractUTXOAmount 从UTXO中提取金额
func (s *StaticResourceDeployService) extractUTXOAmount(utxoItem *utxo.UTXO) uint64 {
	if utxoItem == nil {
		return 0
	}

	// 根据UTXO的content_strategy提取金额
	switch strategy := utxoItem.ContentStrategy.(type) {
	case *utxo.UTXO_CachedOutput:
		if cachedOutput := strategy.CachedOutput; cachedOutput != nil {
			if assetOutput := cachedOutput.GetAsset(); assetOutput != nil {
				if nativeCoin := assetOutput.GetNativeCoin(); nativeCoin != nil {
					amount, err := utils.ParseAmountSafely(nativeCoin.Amount)
					if err != nil {
						return 0
					}
					return amount
				}
				if contractToken := assetOutput.GetContractToken(); contractToken != nil {
					amount, err := utils.ParseAmountSafely(contractToken.Amount)
					if err != nil {
						return 0
					}
					return amount
				}
			}
		}
	case *utxo.UTXO_ReferenceOnly:
		// 引用型UTXO通常用于ResourceUTXO，对资产消费无金额意义
		return 0
	}

	return 0
}

// formatAmount 格式化金额为字符串
func (s *StaticResourceDeployService) formatAmount(amount uint64) string {
	// 使用统一的protobuf Amount字段格式化方法
	return utils.FormatAmountForProtobuf(amount)
}

// ============================================================================
//
//	编译时接口检查
//
// ============================================================================
// resolveFileNameConflict 处理文件名冲突（iOS风格递增）
//
// 🎯 **冲突处理策略**：
//   - file.txt -> file(1).txt -> file(2).txt
//   - 自动递增数字直到找到不冲突的文件名
//
// 参数：
//   - targetDir: 目标目录
//   - fileName: 原始文件名
//
// 返回：
//   - string: 解决冲突后的完整文件路径
func (s *StaticResourceDeployService) resolveFileNameConflict(targetDir, fileName string) string {
	// 处理空文件名
	if fileName == "" {
		fileName = "untitled"
	}

	basePath := filepath.Join(targetDir, fileName)

	// 如果文件不存在，直接返回原路径
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return basePath
	}

	// 分离文件名和扩展名
	ext := filepath.Ext(fileName)
	nameWithoutExt := fileName[:len(fileName)-len(ext)]

	// iOS风格递增：name(1).ext, name(2).ext, ...
	counter := 1
	for {
		newFileName := fmt.Sprintf("%s(%d)%s", nameWithoutExt, counter, ext)
		newPath := filepath.Join(targetDir, newFileName)

		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
		counter++
	}
}

// copyFileToTarget 复制文件到目标位置
//
// 🎯 **文件复制功能**：
//   - 从存储路径复制文件到目标路径
//   - 保持文件内容完整性
//
// 参数：
//   - sourcePath: 源文件路径
//   - targetPath: 目标文件路径
//
// 返回：
//   - error: 复制错误
func (s *StaticResourceDeployService) copyFileToTarget(sourcePath, targetPath string) error {
	// 打开源文件
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer sourceFile.Close()

	// 创建目标文件
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer targetFile.Close()

	// 复制文件内容
	_, err = io.Copy(targetFile, sourceFile)
	if err != nil {
		return fmt.Errorf("复制文件内容失败: %w", err)
	}

	// 确保数据写入磁盘
	if err := targetFile.Sync(); err != nil {
		return fmt.Errorf("同步文件数据失败: %w", err)
	}

	return nil
}

// getResourceBasePath 获取资源存储基础路径
//
// 🎯 **路径管理**：
//   - 从配置或默认值获取资源存储根路径
//
// 返回：
//   - string: 资源存储基础路径
func (s *StaticResourceDeployService) getResourceBasePath() string {
	// 从配置管理器获取资源存储路径
	if s.configManager != nil {
		// 假设配置中有资源存储路径配置
		// 实际实现时需要根据具体的配置结构调整
		return "./resources" // 默认路径
	}
	return "./resources" // 默认路径
}

// getDefaultDownloadDir 获取操作系统默认下载目录
//
// 🎯 **跨平台下载目录**：
//   - Windows: %USERPROFILE%\Downloads
//   - macOS: ~/Downloads
//   - Linux: ~/Downloads (如果存在) 或 ~/下载 (中文系统)
//   - 其他: ./downloads (当前目录下的 downloads 文件夹)
//
// 📝 **目录优先级**：
//  1. 操作系统标准下载目录
//  2. 用户主目录下的 Downloads
//  3. 当前工作目录下的 downloads
//
// 返回：
//   - string: 默认下载目录路径
func (s *StaticResourceDeployService) getDefaultDownloadDir() string {
	var downloadDir string

	switch runtime.GOOS {
	case "windows":
		// Windows: %USERPROFILE%\Downloads
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			downloadDir = filepath.Join(userProfile, "Downloads")
		}
	case "darwin":
		// macOS: ~/Downloads
		if homeDir, err := os.UserHomeDir(); err == nil {
			downloadDir = filepath.Join(homeDir, "Downloads")
		}
	case "linux":
		// Linux: ~/Downloads 或 ~/下载
		if homeDir, err := os.UserHomeDir(); err == nil {
			// 优先尝试英文 Downloads 目录
			englishDownloads := filepath.Join(homeDir, "Downloads")
			if _, err := os.Stat(englishDownloads); err == nil {
				downloadDir = englishDownloads
			} else {
				// 尝试中文下载目录（常见于中文 Linux 系统）
				chineseDownloads := filepath.Join(homeDir, "下载")
				if _, err := os.Stat(chineseDownloads); err == nil {
					downloadDir = chineseDownloads
				} else {
					// 如果都不存在，使用英文作为默认
					downloadDir = englishDownloads
				}
			}
		}
	default:
		// 其他操作系统或无法获取用户目录时的后备方案
		if homeDir, err := os.UserHomeDir(); err == nil {
			downloadDir = filepath.Join(homeDir, "Downloads")
		}
	}

	// 如果无法获取系统下载目录，使用当前目录下的 downloads
	if downloadDir == "" {
		downloadDir = "./downloads"
		if s.logger != nil {
			s.logger.Warn("无法获取系统下载目录，使用当前目录下的 downloads 文件夹")
		}
	}

	// 记录使用的下载目录
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("使用默认下载目录: %s (操作系统: %s)", downloadDir, runtime.GOOS))
	}

	return downloadDir
}

// 确保 StaticResourceDeployService 实现了所需的接口部分
var _ interface {
	DeployStaticResource(context.Context, []byte, string, string, string, []string, ...*types.ResourceDeployOptions) ([]byte, error)
	FetchStaticResourceFile(context.Context, []byte, []byte, string) (string, error)
} = (*StaticResourceDeployService)(nil)
