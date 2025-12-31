package middleware

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	txpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// SignatureValidation 签名验证中间件
// 🔐 零信任架构核心：验证交易签名，拒绝未签名交易
type SignatureValidation struct {
	logger     *zap.Logger
	txVerifier tx.TxVerifier
}

// NewSignatureValidation 创建签名验证中间件
func NewSignatureValidation(logger *zap.Logger, txVerifier tx.TxVerifier) *SignatureValidation {
	return &SignatureValidation{
		logger:     logger,
		txVerifier: txVerifier,
	}
}

// Middleware 返回Gin中间件
// 🔐 零信任架构核心：
// - 仅验证已签名交易
// - 拒绝未签名交易
// - 拒绝包含私钥的请求
func (m *SignatureValidation) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 仅对写操作进行签名验证（POST /api/v1/transactions 等）
		if !isWriteOperation(c.Request.URL.Path, c.Request.Method) {
			c.Next()
			return
		}

		// 对于JSON-RPC请求，验证已在方法层完成，这里直接放行
		// 因为JSON-RPC的wes_sendRawTransaction方法会验证签名
		if isJSONRPCRequest(c.Request) {
			c.Next()
			return
		}

		// 读取请求体
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			m.logger.Error("Failed to read request body",
				zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request body",
			})
			c.Abort()
			return
		}

		// 检查是否包含私钥字段（零信任审计）
		if containsPrivateKey(bodyBytes) {
			m.logger.Warn("Request contains private_key field - REJECTED",
				zap.String("path", c.Request.URL.Path),
				zap.String("client_ip", c.ClientIP()))
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Private keys are not accepted. Please sign transaction on client side.",
				"code":  "PRIVATE_KEY_FORBIDDEN",
			})
			c.Abort()
			return
		}

		// 尝试提取已签名交易
		signedTx, err := extractSignedTransaction(bodyBytes)
		if err != nil {
			m.logger.Warn("Failed to extract signed transaction",
				zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Missing or invalid signed transaction",
				"code":  "INVALID_SIGNED_TX",
			})
			c.Abort()
			return
		}

		// 1. 反序列化protobuf交易
		txObj := &txpb.Transaction{}
		if err := proto.Unmarshal(signedTx, txObj); err != nil {
			m.logger.Warn("Failed to unmarshal transaction",
				zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid transaction format",
				"code":  "INVALID_TX_FORMAT",
			})
			c.Abort()
			return
		}

		// 2. 调用TxVerifier验证签名
		if m.txVerifier != nil {
			if err := m.txVerifier.Verify(c.Request.Context(), txObj); err != nil {
				m.logger.Warn("Signature verification failed",
					zap.Error(err),
					zap.String("path", c.Request.URL.Path),
					zap.String("client_ip", c.ClientIP()))
				c.JSON(http.StatusUnauthorized, gin.H{
					"error":  "Signature verification failed",
					"code":   "INVALID_SIGNATURE",
					"detail": err.Error(),
				})
				c.Abort()
				return
			}
		}

		// 3. 验证通过，记录审计日志
		m.logger.Info("Transaction signature validated",
			zap.Int("tx_size", len(signedTx)),
			zap.Int("num_inputs", len(txObj.Inputs)),
			zap.Int("num_outputs", len(txObj.Outputs)),
			zap.String("client_ip", c.ClientIP()))

		// 4. 将交易对象存入上下文，供后续handler使用
		c.Set("validated_tx", txObj)

		c.Next()
	}
}

// isWriteOperation 判断是否为写操作
func isWriteOperation(path string, method string) bool {
	// 写操作的路径模式
	writePatterns := []string{
		"/api/v1/transactions",
		"/wes_sendRawTransaction",
	}

	if method != http.MethodPost {
		return false
	}

	for _, pattern := range writePatterns {
		if contains(path, pattern) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// isJSONRPCRequest 判断是否为JSON-RPC请求
func isJSONRPCRequest(req *http.Request) bool {
	// JSON-RPC请求通常通过Content-Type判断
	contentType := req.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/json") &&
		strings.Contains(req.URL.Path, "/rpc")
}

// containsPrivateKey 检查请求体是否包含私钥字段（零信任审计）
// 🔐 这是零信任架构的关键检查：拒绝任何包含私钥的请求
func containsPrivateKey(body []byte) bool {
	// 解析JSON查找private_key相关字段
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return false
	}

	// 检查危险字段
	dangerousFields := []string{
		"private_key",
		"privateKey",
		"privKey",
		"priv_key",
		"secret_key",
		"secretKey",
	}

	for _, field := range dangerousFields {
		if _, exists := data[field]; exists {
			return true
		}
	}

	return false
}

// extractSignedTransaction 从请求体中提取已签名交易
func extractSignedTransaction(body []byte) ([]byte, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	// 尝试从不同字段提取signed_tx
	possibleFields := []string{
		"signed_tx",
		"signedTx",
		"signed_transaction",
		"raw_transaction",
		"rawTransaction",
	}

	for _, field := range possibleFields {
		if val, exists := data[field]; exists {
			if str, ok := val.(string); ok {
				// 移除0x前缀并解码
				if len(str) > 2 && str[:2] == "0x" {
					str = str[2:]
				}
				return hex.DecodeString(str)
			}
		}
	}

	return nil, fmt.Errorf("no signed transaction found in request")
}
