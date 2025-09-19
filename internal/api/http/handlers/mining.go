// Package handlers 实现HTTP API处理器
//
// mining.go - 挖矿控制处理器
//
// 职责：处理挖矿相关的HTTP请求，包括：
// - 启动挖矿：开始持续挖矿进程
// - 停止挖矿：停止正在进行的挖矿
// - 挖矿状态：查询当前挖矿状态和性能指标
//
// 设计原则：
// 1. 最薄API层：只处理HTTP请求/响应，不包含业务逻辑
// 2. 直接使用标准类型：优先使用标准结构，避免数据转换
// 3. 错误处理：统一的错误响应格式和日志记录
// 4. 权限控制：挖矿控制需要适当的权限验证
//
// 接口映射关系：
// - StartMining -> ConsensusService.StartMining()     // 共识层：启动挖矿
// - StopMining -> ConsensusService.StopMining()       // 共识层：停止挖矿
// - GetMiningStatus -> ConsensusService.GetMiningStatus() // 共识层：挖矿状态
package handlers

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/consensus"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// MiningHandlers 挖矿控制处理器
//
// 负责处理所有与挖矿控制相关的HTTP请求，提供面向管理员的挖矿操作接口。
// 通过依赖注入的方式获取共识服务，确保职责分离和可测试性。
//
// 功能范围：
// - 挖矿进程控制（启动、停止）
// - 挖矿状态监控（状态查询、性能指标）
// - 挖矿配置管理（未来扩展）
//
// 安全考虑：
// - 挖矿控制是敏感操作，需要权限验证
// - 防止恶意启动/停止挖矿
// - 记录所有挖矿控制操作
type MiningHandlers struct {
	consensusService consensus.MinerService  // 矿工服务接口，提供挖矿控制功能
	configProvider   config.Provider         // 配置提供者，用于获取挖矿配置
	addressManager   crypto.AddressManager   // 地址管理器，用于地址转换和验证
	chainService     blockchain.ChainService // 链服务，用于获取区块链状态
	logger           log.Logger              // 日志记录器，用于记录操作日志和错误信息
}

// NewMiningHandlers 创建挖矿处理器实例
//
// 通过依赖注入的方式创建MiningHandlers实例，确保所有依赖都正确初始化。
// 这种设计模式便于单元测试和模块解耦。
//
// 参数：
//   - consensusService: 共识服务接口，提供挖矿控制的底层实现
//   - chainService: 链服务，用于获取区块链状态
//   - logger: 日志记录器，用于记录操作过程和错误信息
//
// 返回：
//   - 完全初始化的MiningHandlers实例
func NewMiningHandlers(
	consensusService consensus.MinerService,
	configProvider config.Provider,
	addressManager crypto.AddressManager,
	chainService blockchain.ChainService,
	logger log.Logger,
) *MiningHandlers {
	return &MiningHandlers{
		consensusService: consensusService,
		configProvider:   configProvider,
		addressManager:   addressManager,
		chainService:     chainService,
		logger:           logger,
	}
}

// validateMinerAddress 验证并解析矿工地址
//
// 🎯 正确设计：矿工地址必须由用户明确提供，不使用默认值
//
// 设计原则：
// ✅ 去中心化：每个矿工必须明确指定自己的地址
// ✅ 安全性：防止配置错误导致奖励丢失给错误的地址
// ✅ 透明性：用户必须明确知道奖励将发送到哪个地址
//
// 参数：
//   - minerAddress: 用户提供的矿工地址字符串
//
// 返回：
//   - []byte: 解析后的矿工地址字节数组
//   - error: 地址无效或为空时的错误信息
func (h *MiningHandlers) validateMinerAddress(minerAddress string) ([]byte, error) {
	h.logger.Infof("🔍 [validateMinerAddress] 开始验证矿工地址: %s", minerAddress)

	if minerAddress == "" {
		h.logger.Error("❌ [validateMinerAddress] 矿工地址为空")
		return nil, fmt.Errorf(`
矿工地址是必需的

🎯WES采用去中心化挖矿设计：
• 每个矿工必须明确指定自己的地址
• 不存在"默认"矿工地址
• 挖矿奖励将发送到您指定的地址

📋 请在API请求中提供 miner_address 字段：
{
  "miner_address": "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn"
}`)
	}

	// 解析并验证地址格式
	h.logger.Info("🔍 [validateMinerAddress] 开始解析地址字符串")
	return h.parseAddressString(minerAddress)
}

// parseAddressString 解析地址字符串为字节数组
//
// 🔧 修复：统一使用AddressManager确保地址处理一致性
// 支持标准Base58Check地址格式，
// 并进行格式验证确保地址的有效性。
//
// 参数：
//   - addressStr:WES标准地址字符串 (Base58Check格式)
//
// 返回：
//   - []byte: 地址的字节数组表示 (20字节)
//   - error: 解析失败时的错误信息
func (h *MiningHandlers) parseAddressString(addressStr string) ([]byte, error) {
	h.logger.Infof("🔍 [parseAddressString] 开始解析地址字符串: %s", addressStr)

	if addressStr == "" {
		h.logger.Error("❌ [parseAddressString] 地址字符串为空")
		return nil, fmt.Errorf("地址字符串不能为空")
	}

	// 🔧 修复：统一使用AddressManager进行地址验证和转换
	// 这确保了与验证时使用相同的地址算法，避免格式不匹配

	// 1. 验证地址格式
	h.logger.Info("🔍 [parseAddressString] 步骤1: 开始验证地址格式")
	valid, err := h.addressManager.ValidateAddress(addressStr)
	if err != nil || !valid {
		h.logger.Errorf("❌ [parseAddressString] 地址格式验证失败: %v, valid: %t", err, valid)
		return nil, fmt.Errorf("地址格式验证失败: %v", err)
	}
	h.logger.Info("✅ [parseAddressString] 地址格式验证成功")

	// 2. 使用AddressManager标准化并转换为字节
	h.logger.Info("🔍 [parseAddressString] 步骤2: 开始转换地址为字节")
	addressBytes, err := h.addressManager.AddressToBytes(addressStr)
	if err != nil {
		h.logger.Errorf("❌ [parseAddressString] 地址转换失败: %v", err)
		return nil, fmt.Errorf("地址转换失败: %v", err)
	}
	h.logger.Infof("✅ [parseAddressString] 地址转换成功，字节长度: %d", len(addressBytes))

	// 3. 验证转换结果
	h.logger.Info("🔍 [parseAddressString] 步骤3: 开始验证转换结果")
	if len(addressBytes) != 20 {
		h.logger.Errorf("❌ [parseAddressString] 地址字节长度错误，期望20字节，实际%d字节", len(addressBytes))
		return nil, fmt.Errorf("地址字节长度错误，期望20字节，实际%d字节", len(addressBytes))
	}
	h.logger.Info("✅ [parseAddressString] 地址字节长度验证成功")

	h.logger.Infof("✅ [parseAddressString] 地址解析完全成功: %s -> %x (使用AddressManager标准算法)", addressStr, addressBytes)
	return addressBytes, nil
}

// getCurrentHeight 获取当前区块链高度
//
// 辅助方法，用于获取当前区块链的最新高度
func (h *MiningHandlers) getCurrentHeight(ctx context.Context) (uint64, error) {
	h.logger.Info("🔍 [getCurrentHeight] 开始获取当前区块链高度")

	if h.chainService != nil {
		h.logger.Info("🔍 [getCurrentHeight] ChainService存在，开始调用GetChainInfo")
		chainInfo, err := h.chainService.GetChainInfo(ctx)
		if err != nil {
			h.logger.Errorf("❌ [getCurrentHeight] GetChainInfo调用失败: %v", err)
			return 0, err
		}
		h.logger.Infof("✅ [getCurrentHeight] 获取链信息成功，高度: %d", chainInfo.Height)
		return chainInfo.Height, nil
	}

	h.logger.Warnf("⚠️ [getCurrentHeight] ChainService为空，返回固定值0")
	return 0, nil
}

// StartMining 启动挖矿
//
// 📌 **接口说明**：启动持续挖矿进程，自动挖掘新区块并打包交易
//
// **HTTP Method**: `POST`
// **URL Path**: `/mining/start`
//
// **请求体参数**：
//   - miner_address (string, required): 矿工地址，挖矿奖励将发送到此地址
//   - threads (number, optional): 挖矿线程数，默认4
//
// **请求体示例**：
//
//	{
//	  "miner_address": "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
//	  "threads": 4
//	}
//
// ✅ **成功响应示例**：
//
//	{
//	  "message": "挖矿启动成功",
//	  "status": "mining_started",
//	  "miner_address": "1234567890abcdef..."
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "error": "启动挖矿失败",
//	  "details": "挖矿已在进行中"
//	}
//
// 💡 **使用说明**：
// - 挖矿是资源密集型操作，会持续运行直到手动停止
// - 矿工地址必须有效，奖励将发送到此地址
// - 建议线程数设置为CPU核心数的50-80%
func (h *MiningHandlers) StartMining(c *gin.Context) {
	h.logger.Info("收到启动挖矿请求")

	// 解析请求体获取矿工地址（可选）
	var request struct {
		MinerAddress string `json:"miner_address,omitempty"`
	}

	// 如果有请求体，尝试解析
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			h.logger.Errorf("解析请求体失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "请求格式错误",
				"details": err.Error(),
			})
			return
		}
	}

	// 验证用户必须提供矿工地址
	minerAddress, err := h.validateMinerAddress(request.MinerAddress)
	if err != nil {
		h.logger.Errorf("矿工地址验证失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "矿工地址验证失败",
			"details": err.Error(),
		})
		return
	}

	h.logger.Infof("启动挖矿 - 矿工地址: %x", minerAddress)

	// 调用共识层接口启动挖矿
	// 注意：使用context.Background()而不是HTTP请求的context
	// 因为挖矿是长期运行的后台任务，不应该在HTTP请求结束时被取消
	err = h.consensusService.StartMining(context.Background(), minerAddress)
	if err != nil {
		h.logger.Errorf("启动挖矿失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "启动挖矿失败",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("挖矿启动成功")
	c.JSON(http.StatusOK, gin.H{
		"message":       "挖矿启动成功",
		"status":        "mining_started",
		"miner_address": hex.EncodeToString(minerAddress),
	})
}

// StopMining 停止挖矿
//
// 📌 **接口说明**：停止正在进行的挖矿进程，优雅关闭挖矿服务
//
// **HTTP Method**: `POST`
// **URL Path**: `/mining/stop`
//
// **请求体参数**：无需请求体
//
// ✅ **成功响应示例**：
//
//	{
//	  "message": "挖矿停止成功",
//	  "status": "mining_stopped"
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "error": "停止挖矿失败",
//	  "details": "挖矿未在进行中"
//	}
//
// 💡 **使用说明**：
// - 停止操作是优雅的，会等待当前工作完成
// - 确保数据一致性，不会中断正在处理的区块
func (h *MiningHandlers) StopMining(c *gin.Context) {
	h.logger.Info("收到停止挖矿请求")

	// 调用共识层接口停止挖矿
	err := h.consensusService.StopMining(context.Background())
	if err != nil {
		h.logger.Errorf("停止挖矿失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "停止挖矿失败",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("挖矿停止成功")
	c.JSON(http.StatusOK, gin.H{
		"message": "挖矿停止成功",
		"status":  "mining_stopped",
	})
}

// GetMiningStatus 获取挖矿状态
//
// 📌 **接口说明**：查询当前挖矿状态的详细信息
//
// **HTTP Method**: `GET`
// **URL Path**: `/mining/status`
//
// ✅ **成功响应示例**：
//
//	{
//	  "is_mining": true,
//	  "miner_address": "1234567890abcdef...",
//	  "start_time": "2024-01-15T10:30:00Z",
//	  "current_height": 12345
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "error": "获取挖矿状态失败",
//	  "details": "共识服务不可用"
//	}
//
// 💡 **使用说明**：
// - 实时监控挖矿状态，用于系统管理和性能分析
// - 只读操作，无副作用，可以频繁调用
// - 返回挖矿进程的完整状态信息
func (h *MiningHandlers) GetMiningStatus(c *gin.Context) {
	h.logger.Info("查询挖矿状态")

	// 调用共识层接口获取挖矿状态
	isRunning, minerAddress, err := h.consensusService.GetMiningStatus(context.Background())
	if err != nil {
		h.logger.Errorf("获取挖矿状态失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取挖矿状态失败",
			"details": err.Error(),
		})
		return
	}

	// 构建状态响应
	status := gin.H{
		"is_mining":      isRunning,
		"miner_address":  "",
		"start_time":     nil,
		"current_height": nil,
	}

	if isRunning && len(minerAddress) > 0 {
		status["miner_address"] = hex.EncodeToString(minerAddress)
	}

	h.logger.Info("挖矿状态查询成功")
	c.JSON(http.StatusOK, status)
}

// MineOnce 单次挖矿
//
// HTTP端点：POST /api/v1/mining/once
//
// 功能：执行一次挖矿操作，挖掘一个区块后停止。
// 这对于测试和观察挖矿过程非常有用。
//
// 请求体：
//
//	{
//	  "miner_address": "0x1111111111111111111111111111111111111111",  // 可选，矿工地址
//	  "max_txs": 1000  // 可选，最大交易数
//	}
//
// 响应：
// - 成功：返回挖掘的区块信息
// - 失败：返回错误码和详细错误信息
//
// 注意：这是一个同步操作，会等待挖矿完成后返回
func (h *MiningHandlers) MineOnce(c *gin.Context) {
	h.logger.Info("🔍 [MineOnce] 收到单次挖矿请求")

	// 解析请求体
	var request struct {
		MinerAddress string `json:"miner_address,omitempty"`
		MaxTxs       uint32 `json:"max_txs,omitempty"`
	}

	h.logger.Infof("🔍 [MineOnce] 请求体长度: %d", c.Request.ContentLength)

	// 如果有请求体，尝试解析
	if c.Request.ContentLength > 0 {
		h.logger.Info("🔍 [MineOnce] 开始解析JSON请求体")
		if err := c.ShouldBindJSON(&request); err != nil {
			h.logger.Errorf("❌ [MineOnce] 解析请求体失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "请求格式错误",
				"details": err.Error(),
			})
			return
		}
		h.logger.Infof("✅ [MineOnce] JSON解析成功 - miner_address: %s, max_txs: %d", request.MinerAddress, request.MaxTxs)
	} else {
		h.logger.Info("🔍 [MineOnce] 无请求体，使用默认值")
	}

	// 验证用户必须提供矿工地址
	h.logger.Infof("🔍 [MineOnce] 开始验证矿工地址: %s", request.MinerAddress)
	minerAddress, err := h.validateMinerAddress(request.MinerAddress)
	if err != nil {
		h.logger.Errorf("❌ [MineOnce] 矿工地址验证失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "矿工地址验证失败",
			"details": err.Error(),
		})
		return
	}
	h.logger.Infof("✅ [MineOnce] 矿工地址验证成功: %x", minerAddress)

	h.logger.Infof("🔍 [MineOnce] 单次挖矿 - 矿工地址: %x", minerAddress)

	maxTxs := uint32(1000)
	if request.MaxTxs > 0 {
		maxTxs = request.MaxTxs
	}
	h.logger.Infof("🔍 [MineOnce] 单次挖矿最大交易数限制: %d", maxTxs)

	h.logger.Infof("🔍 [MineOnce] 开始单次挖矿 - 矿工地址: %x", minerAddress)

	// 获取当前区块链状态
	h.logger.Info("🔍 [MineOnce] 开始获取当前区块链状态")
	currentHeight, err := h.getCurrentHeight(context.Background())
	if err != nil {
		h.logger.Errorf("❌ [MineOnce] 获取当前高度失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "单次挖矿失败",
			"details": fmt.Sprintf("获取当前高度失败: %v", err),
		})
		return
	}
	h.logger.Infof("✅ [MineOnce] 获取当前高度成功: %d", currentHeight)

	nextHeight := currentHeight + 1
	h.logger.Infof("🔍 [MineOnce] 当前高度: %d, 将挖掘高度: %d", currentHeight, nextHeight)

	// 记录挖矿前的高度
	heightBefore := currentHeight

	// 记录开始时间
	startTime := time.Now()

	// 执行真正的单次挖矿 - 启动挖矿，监控高度变化，挖到一个区块后立即停止
	h.logger.Info("🔍 [MineOnce] 开始执行单次挖矿")

	// 启动挖矿
	miningCtx := context.Background()
	err = h.consensusService.StartMining(miningCtx, minerAddress)
	if err != nil {
		h.logger.Errorf("❌ [MineOnce] 启动挖矿失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "单次挖矿失败",
			"details": fmt.Sprintf("启动挖矿失败: %v", err),
		})
		return
	}
	h.logger.Infof("✅ [MineOnce] 挖矿已启动，开始监控高度变化")

	// 在后台协程中监控挖矿进度，挖到一个区块后立即停止
	go func() {
		h.monitorMiningProgressForOnce(minerAddress, heightBefore, startTime)
	}()

	// 立即返回，让用户知道挖矿已启动
	h.logger.Infof("✅ [MineOnce] 单次挖矿已启动，将在挖到第一个区块后自动停止")

	// 记录时间
	elapsed := time.Since(startTime)
	h.logger.Infof("单次挖矿已启动，耗时: %s", elapsed)

	c.JSON(http.StatusOK, gin.H{
		"message":       "单次挖矿已启动",
		"status":        "mining_started",
		"height_before": heightBefore,
		"elapsed_time":  elapsed.String(),
		"miner_address": hex.EncodeToString(minerAddress),
		"note":          "挖矿将在后台运行，挖到第一个区块后自动停止",
	})
}

// monitorMiningProgressForOnce 单次挖矿监控 - 挖到第一个区块后立即停止
func (h *MiningHandlers) monitorMiningProgressForOnce(minerAddress []byte, heightBefore uint64, startTime time.Time) {
	h.logger.Infof("🔍 开始单次挖矿监控: height=%d, miner=%x", heightBefore, minerAddress)

	timeout := 60 * time.Second                      // 1分钟超时
	ticker := time.NewTicker(500 * time.Millisecond) // 更频繁的检查
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 检查是否超时
			if time.Since(startTime) > timeout {
				h.logger.Warnf("⏰ 单次挖矿监控超时，停止挖矿")
				h.consensusService.StopMining(context.Background())
				return
			}

			// 检查区块高度是否增加
			newHeight, err := h.getCurrentHeight(context.Background())
			if err == nil && newHeight > heightBefore {
				// 检测到新区块，立即停止挖矿
				elapsed := time.Since(startTime)
				h.logger.Infof("✅ 单次挖矿成功！高度从 %d 增加到 %d，耗时: %s",
					heightBefore, newHeight, elapsed)

				// 立即停止挖矿
				stopErr := h.consensusService.StopMining(context.Background())
				if stopErr != nil {
					h.logger.Warnf("停止挖矿失败: %v", stopErr)
				} else {
					h.logger.Infof("✅ 单次挖矿完成，已自动停止挖矿")
				}
				return
			}
		}
	}
}

// monitorMiningProgress 后台监控挖矿进度
func (h *MiningHandlers) monitorMiningProgress(minerAddress []byte, heightBefore uint64, startTime time.Time) {
	h.logger.Infof("🔍 开始后台监控挖矿进度: height=%d, miner=%x", heightBefore, minerAddress)

	timeout := 300 * time.Second // 5分钟超时
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 检查是否超时
			if time.Since(startTime) > timeout {
				h.logger.Warnf("⏰ 单次挖矿监控超时，停止挖矿")
				h.consensusService.StopMining(context.Background())
				return
			}

			// 检查区块高度是否增加
			newHeight, err := h.getCurrentHeight(context.Background())
			if err == nil && newHeight > heightBefore {
				// 检测到新区块
				elapsed := time.Since(startTime)
				h.logger.Infof("✅ 检测到新区块，高度从 %d 增加到 %d，耗时: %s",
					heightBefore, newHeight, elapsed)

				// 停止挖矿
				stopErr := h.consensusService.StopMining(context.Background())
				if stopErr != nil {
					h.logger.Warnf("停止挖矿失败: %v", stopErr)
				} else {
					h.logger.Infof("✅ 单次挖矿监控完成，已自动停止挖矿")
				}
				return
			}
		}
	}
}

// RegisterRoutes 注册挖矿相关路由
//
// 将挖矿控制的所有HTTP端点注册到指定的路由组中。
// 这种设计模式便于路由管理和中间件应用。
//
// 参数：
//   - router: Gin路由组，通常是/api/v1的子组
//
// 注册的路由：
//   - POST /mining/start - 启动挖矿
//   - POST /mining/stop - 停止挖矿
//   - POST /mining/once - 单次挖矿
//   - GET /mining/status - 获取挖矿状态
//
// 中间件建议：
//   - 权限验证：挖矿控制需要管理员权限
//   - 速率限制：防止频繁的启动/停止操作
//   - 审计日志：记录所有挖矿控制操作
func (h *MiningHandlers) RegisterRoutes(router *gin.RouterGroup) {
	// 创建挖矿路由组
	miningGroup := router.Group("/mining")

	// 挖矿控制端点
	miningGroup.POST("/start", h.StartMining)     // 启动挖矿
	miningGroup.POST("/stop", h.StopMining)       // 停止挖矿
	miningGroup.POST("/once", h.MineOnce)         // 单次挖矿
	miningGroup.GET("/status", h.GetMiningStatus) // 获取挖矿状态

	h.logger.Info("挖矿控制路由注册完成")
}
