package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	apitypes "github.com/weisyn/v1/internal/api/types"
	"go.uber.org/zap"
)

// ErrorHandler 错误处理中间件
func ErrorHandler(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 检查是否有错误
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			// 强制要求所有错误必须是 Problem Details
			problem, ok := apitypes.IsProblemDetails(err)
			if !ok {
				// 如果不是 Problem Details，记录错误并返回通用错误
				logger.Error("Handler returned non-ProblemDetails error",
					zap.String("path", c.Request.URL.Path),
					zap.Error(err))
				problem = apitypes.NewProblemDetails(
					apitypes.CodeCommonInternalError,
					apitypes.LayerBlockchainService,
					"服务器内部错误，请稍后重试或联系管理员。",
					fmt.Sprintf("Internal error: %v", err),
					500,
					map[string]interface{}{
						"path": c.Request.URL.Path,
					},
				)
			}
			
			logger.Error("HTTP error",
				zap.String("code", problem.Code),
				zap.String("traceId", problem.TraceID),
				zap.String("path", c.Request.URL.Path),
				zap.Error(err))
			problem.WriteJSON(c.Writer)
			c.Abort()
		}
	}
}

// WriteProblemDetails 写入 Problem Details 响应
func WriteProblemDetails(c *gin.Context, problem *apitypes.ProblemDetails) {
	c.Header("Content-Type", "application/problem+json")
	c.JSON(problem.Status, problem)
	c.Abort()
}

// WriteError 写入错误响应（自动转换为 Problem Details）
func WriteError(c *gin.Context, code string, userMessage string, detail string, status int, details map[string]interface{}) {
	problem := apitypes.NewProblemDetails(
		code,
		apitypes.LayerBlockchainService,
		userMessage,
		detail,
		status,
		details,
	)
	WriteProblemDetails(c, problem)
}

