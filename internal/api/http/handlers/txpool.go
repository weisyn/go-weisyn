package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"go.uber.org/zap"
)

// TxPoolHandler 交易池端点处理器
//
// 📊 **交易池查询端点**
//
// 提供交易池状态和内容查询：
// - /txpool/status: 交易池状态（待处理交易数量）
// - /txpool/content: 交易池内容（所有待处理交易）
// - /txpool/inspect: 查询特定地址的待处理交易
type TxPoolHandler struct {
	logger *zap.Logger
	pool   mempool.TxPool
}

// NewTxPoolHandler 创建交易池处理器
//
// 参数：
//   - logger: 日志记录器
//   - pool: 交易池服务
//
// 返回：交易池处理器实例
func NewTxPoolHandler(logger *zap.Logger, pool mempool.TxPool) *TxPoolHandler {
	return &TxPoolHandler{
		logger: logger,
		pool:   pool,
	}
}

// RegisterRoutes 注册交易池路由
//
// 注册三个交易池端点：
// - GET /txpool/status: 交易池状态
// - GET /txpool/content: 交易池内容
// - GET /txpool/inspect: 查询特定地址的待处理交易
func (h *TxPoolHandler) RegisterRoutes(r *gin.RouterGroup) {
	txpool := r.Group("/txpool")
	{
		txpool.GET("/status", h.GetStatus)   // 交易池状态
		txpool.GET("/content", h.GetContent) // 交易池内容
		txpool.GET("/inspect", h.Inspect)    // 查询特定地址的待处理交易
	}
}

// GetStatus 获取交易池状态
//
// GET /api/v1/txpool/status
//
// 返回交易池的基本状态信息：
// - pending: 待处理交易数量
// - queued: 排队交易数量（当前为0）
func (h *TxPoolHandler) GetStatus(c *gin.Context) {
	ctx := c.Request.Context()

	if h.pool == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "txpool not available",
			"code":  "SERVICE_UNAVAILABLE",
		})
		return
	}

	// 获取待处理交易
	pendingTxs, err := h.pool.GetPendingTransactions()
	if err != nil {
		h.logger.Error("Failed to get pending transactions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pending": len(pendingTxs),
		"queued":  0, // 可选：后续扩展queued统计
	})
	_ = ctx // 避免未使用变量警告
}

// GetContent 获取交易池内容
//
// GET /api/v1/txpool/content
//
// 返回交易池中的所有待处理交易：
// - pending: 待处理交易列表
// - queued: 排队交易列表（当前为空）
func (h *TxPoolHandler) GetContent(c *gin.Context) {
	ctx := c.Request.Context()

	if h.pool == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "txpool not available",
			"code":  "SERVICE_UNAVAILABLE",
		})
		return
	}

	// 获取待处理交易
	pendingTxs, err := h.pool.GetPendingTransactions()
	if err != nil {
		h.logger.Error("Failed to get pending transactions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// 简化：返回交易数量和总结信息
	// 完整的地址分组需要计算交易哈希或从输入推导发送者，暂简化
	pendingList := make([]interface{}, 0, len(pendingTxs))
	for _, tx := range pendingTxs {
		if tx == nil {
			continue
		}
		// 简化信息：只显示输入输出数量
		txInfo := map[string]interface{}{
			"version":    tx.Version,
			"numInputs":  len(tx.Inputs),
			"numOutputs": len(tx.Outputs),
		}
		pendingList = append(pendingList, txInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"pending": pendingList,
		"queued":  []interface{}{},
	})
	_ = ctx // 避免未使用变量警告
}

// Inspect 查询特定地址的待处理交易
//
// GET /api/v1/txpool/inspect?address=<address>
//
// 查询参数：
//   - address: Base58格式的WES地址（如CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR）
//
// 返回该地址的待处理交易信息：
// - address: 查询的地址（Base58格式）
// - pending: 匹配的待处理交易数量
// - queued: 排队交易数量（当前为0）
// - txCount: 匹配的交易数量
// - totalInPool: 交易池中的总交易数量
func (h *TxPoolHandler) Inspect(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取地址参数
	addressStr := c.Query("address")
	if addressStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "missing address parameter",
			"code":  "INVALID_PARAMS",
		})
		return
	}

	if h.pool == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "txpool not available",
			"code":  "SERVICE_UNAVAILABLE",
		})
		return
	}

	// 获取待处理交易
	pendingTxs, err := h.pool.GetPendingTransactions()
	if err != nil {
		h.logger.Error("Failed to get pending transactions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// 筛选属于该地址的交易
	// 🔍 地址匹配策略（当前为简化实现）：
	//
	// WES 中交易输入使用 LockingCondition，不直接包含发送者地址。
	// 完整的 sender 推导需要：
	// 1. 通过 input.PreviousOutput 查询 UTXO
	// 2. 从 UTXO 的 LockingCondition 中提取地址
	// 3. 或从 UnlockingProof 中恢复公钥并派生地址
	//
	// 🚧 当前实现：粗略匹配解锁证明中的公钥字节
	// 🎯 后续优化方向：
	// - 引入 pkg/interfaces/tx 的 sender 推导接口
	// - 使用 crypto.AddressManager 规范化地址派生
	// - 支持多种解锁证明类型（MultiKey、Delegation、Threshold 等）
	// - 建立 txpool 索引加速地址查询
	matchedTxs := make([]interface{}, 0)
	for _, tx := range pendingTxs {
		if tx == nil || len(tx.Inputs) == 0 {
			continue
		}

		// 检查输入的解锁证明（简化版）
		isMatch := false
		for _, input := range tx.Inputs {
			if input == nil || input.PreviousOutput == nil {
				continue
			}

			// 🔍 策略1：检查单密钥证明
			if singleKey := input.GetSingleKeyProof(); singleKey != nil && singleKey.PublicKey != nil {
				// 粗略匹配：比较公钥字节前缀
				// TODO: 替换为规范化地址派生（PublicKey -> Address）
				pubKeyBytes := singleKey.PublicKey.Value
				addressBytes := []byte(addressStr)
				if len(pubKeyBytes) >= len(addressBytes) &&
					string(pubKeyBytes[:len(addressBytes)]) == string(addressBytes) {
					isMatch = true
					break
				}
			}

			// 🔍 策略2：检查多密钥证明（扩展点）
			// if multiKey := input.GetMultiKeyProof(); multiKey != nil {
			//     // TODO: 检查多个公钥是否包含目标地址
			// }

			// 🔍 策略3：检查委托证明（扩展点）
			// if delegation := input.GetDelegationProof(); delegation != nil {
			//     // TODO: 检查委托者或被委托者地址
			// }
		}

		if isMatch {
			txInfo := map[string]interface{}{
				"version":    tx.Version,
				"numInputs":  len(tx.Inputs),
				"numOutputs": len(tx.Outputs),
			}
			matchedTxs = append(matchedTxs, txInfo)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"address":     addressStr, // 返回Base58格式地址
		"pending":     len(matchedTxs),
		"queued":      0,
		"txCount":     len(matchedTxs),
		"totalInPool": len(pendingTxs),
	})
	_ = ctx // 避免未使用变量警告
}
