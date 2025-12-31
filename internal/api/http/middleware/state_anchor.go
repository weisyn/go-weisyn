package middleware

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"go.uber.org/zap"
)

// StateAnchor 状态锚定中间件
// 🔗 状态锚点：允许客户端查询历史状态，实现重组安全查询
// 参考：EIP-1898 (https://eips.ethereum.org/EIPS/eip-1898)
type StateAnchor struct {
	logger       *zap.Logger
	chainQuery   persistence.ChainQuery
	blockQuery   persistence.BlockQuery
}

// NewStateAnchor 创建状态锚定中间件
func NewStateAnchor(
	logger *zap.Logger,
	chainQuery persistence.ChainQuery,
	blockQuery persistence.BlockQuery,
) *StateAnchor {
	return &StateAnchor{
		logger:     logger,
		chainQuery: chainQuery,
		blockQuery: blockQuery,
	}
}

// Middleware 返回Gin中间件
// 🔗 处理流程：
// 1. 解析atHeight/atHash参数
// 2. 验证状态锚点有效性
// 3. 注入上下文供下游handler使用
// 4. 在响应中添加状态锚点字段
func (m *StateAnchor) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 仅对查询操作启用状态锚定
		if !isQueryOperation(c.Request.URL.Path) {
			c.Next()
			return
		}

		// 解析状态锚点参数
		anchor := parseStateAnchor(c)

		// 验证状态锚点有效性
		if anchor != nil && !anchor.UseLatest {
			if err := m.validateStateAnchor(c.Request.Context(), anchor); err != nil {
				m.logger.Warn("Invalid state anchor",
					zap.Error(err),
					zap.Any("anchor", anchor))
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Invalid state anchor",
					"code":    "INVALID_STATE_ANCHOR",
					"details": err.Error(),
				})
				c.Abort()
				return
			}

			m.logger.Debug("State anchor validated",
				zap.Any("anchor", anchor))
		}

		// 注入上下文供下游handler使用
		c.Set("state_anchor", anchor)

		c.Next()

		// 在响应中自动添加状态锚点字段
		// 拦截响应并注入 meta 字段
		if c.Writer.Status() == 200 && anchor != nil && !anchor.UseLatest {
			// 从上下文获取响应数据（handler 需要设置）
			if data, exists := c.Get("response_data"); exists {
				responseData := data.(map[string]interface{})

				// 添加 meta 字段
				meta := make(map[string]interface{})
				if anchor.Height != nil {
					meta["height"] = fmt.Sprintf("0x%x", *anchor.Height)
				}
				if anchor.Hash != nil {
					meta["hash"] = "0x" + *anchor.Hash
				}

				responseData["meta"] = meta
				c.Set("response_data", responseData)
			}
		}
	}
}

// StateAnchorInfo 状态锚点信息
type StateAnchorInfo struct {
	Height    *uint64 // 指定高度
	Hash      *string // 指定哈希
	UseLatest bool    // 使用最新状态
}

// parseStateAnchor 从请求中解析状态锚点
func parseStateAnchor(c *gin.Context) *StateAnchorInfo {
	anchor := &StateAnchorInfo{
		UseLatest: true, // 默认使用最新状态
	}

	// 解析 atHeight 参数（支持十进制和十六进制）
	if heightStr := c.Query("atHeight"); heightStr != "" {
		// 移除0x前缀
		if strings.HasPrefix(heightStr, "0x") {
			heightStr = heightStr[2:]
			// 十六进制解析
			if height, err := strconv.ParseUint(heightStr, 16, 64); err == nil {
				anchor.Height = &height
				anchor.UseLatest = false
			}
		} else {
			// 十进制解析
			if height, err := strconv.ParseUint(heightStr, 10, 64); err == nil {
				anchor.Height = &height
				anchor.UseLatest = false
			}
		}
	}

	// 解析 atHash 参数（移除0x前缀）
	if hash := c.Query("atHash"); hash != "" {
		hash = strings.TrimPrefix(hash, "0x")
		anchor.Hash = &hash
		anchor.UseLatest = false
	}

	return anchor
}

// validateStateAnchor 验证状态锚点有效性
func (m *StateAnchor) validateStateAnchor(ctx context.Context, anchor *StateAnchorInfo) error {
	// 获取当前链信息
	chainInfo, err := m.chainQuery.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get chain info: %w", err)
	}

	// 验证atHeight
	if anchor.Height != nil {
		if *anchor.Height > chainInfo.Height {
			return fmt.Errorf("height %d exceeds current height %d", *anchor.Height, chainInfo.Height)
		}
		// 检查该高度的区块是否存在
		block, err := m.blockQuery.GetBlockByHeight(ctx, *anchor.Height)
		if err != nil || block == nil {
			return fmt.Errorf("block at height %d not found", *anchor.Height)
		}
	}

	// 验证atHash
	if anchor.Hash != nil {
		hashBytes, err := hex.DecodeString(*anchor.Hash)
		if err != nil {
			return fmt.Errorf("invalid hash format: %w", err)
		}
		if len(hashBytes) != 32 {
			return fmt.Errorf("hash must be 32 bytes")
		}
		// 检查该哈希的区块是否存在
		block, err := m.blockQuery.GetBlockByHash(ctx, hashBytes)
		if err != nil || block == nil {
			return fmt.Errorf("block with hash %s not found", *anchor.Hash)
		}
	}

	return nil
}

// isQueryOperation 判断是否为查询操作
func isQueryOperation(path string) bool {
	// 查询操作的路径模式
	queryPatterns := []string{
		"/api/v1/blocks",
		"/api/v1/transactions",
		"/api/v1/utxos",
		"/api/v1/balances",
		"/wes_getBlock",
		"/wes_getTransaction",
		"/wes_getBalance",
	}

	for _, pattern := range queryPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}
