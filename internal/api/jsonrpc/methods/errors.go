package methods

import (
	apitypes "github.com/weisyn/v1/internal/api/types"
)

// NewBlockNotFoundError 创建区块不存在错误（Problem Details）
func NewBlockNotFoundError(heightOrHash interface{}) *apitypes.ProblemDetails {
	return apitypes.NewProblemDetails(
		apitypes.CodeBCBlockNotFound,
		apitypes.LayerBlockchainService,
		"区块不存在。",
		"Block not found",
		404,
		map[string]interface{}{
			"heightOrHash": heightOrHash,
		},
	)
}

// NewTxValidationFailedError 创建交易验证失败错误（Problem Details）
func NewTxValidationFailedError(reason string, details map[string]interface{}) *apitypes.ProblemDetails {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["reason"] = reason
	
	return apitypes.NewProblemDetails(
		apitypes.CodeBCTxValidationFailed,
		apitypes.LayerBlockchainService,
		"交易验证失败，请检查交易签名和参数。",
		reason,
		422,
		details,
	)
}

// NewTxNotFoundError 创建交易不存在错误（Problem Details）
func NewTxNotFoundError(txId interface{}) *apitypes.ProblemDetails {
	return apitypes.NewProblemDetails(
		apitypes.CodeBCTxNotFound,
		apitypes.LayerBlockchainService,
		"交易不存在。",
		"Transaction not found",
		404,
		map[string]interface{}{
			"txId": txId,
		},
	)
}

// NewInsufficientBalanceError 创建余额不足错误（Problem Details）
func NewInsufficientBalanceError(address string, required, available string) *apitypes.ProblemDetails {
	return apitypes.NewProblemDetails(
		apitypes.CodeBCInsufficientBalance,
		apitypes.LayerBlockchainService,
		"余额不足，无法完成交易。",
		"Insufficient balance",
		422,
		map[string]interface{}{
			"address":  address,
			"required": required,
			"available": available,
		},
	)
}

// NewInvalidParamsError 创建参数验证错误（Problem Details）
func NewInvalidParamsError(detail string, details map[string]interface{}) *apitypes.ProblemDetails {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["detail"] = detail
	
	return apitypes.NewProblemDetails(
		apitypes.CodeCommonValidationError,
		apitypes.LayerBlockchainService,
		"请求参数验证失败，请检查输入参数。",
		detail,
		400,
		details,
	)
}

// NewInternalError 创建内部错误（Problem Details）
func NewInternalError(detail string, details map[string]interface{}) *apitypes.ProblemDetails {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["detail"] = detail
	
	return apitypes.NewProblemDetails(
		apitypes.CodeCommonInternalError,
		apitypes.LayerBlockchainService,
		"服务器内部错误，请稍后重试或联系管理员。",
		detail,
		500,
		details,
	)
}

// NewServiceUnavailableError 创建服务不可用错误（Problem Details）
func NewServiceUnavailableError(detail string, details map[string]interface{}) *apitypes.ProblemDetails {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["detail"] = detail
	
	return apitypes.NewProblemDetails(
		apitypes.CodeCommonServiceUnavailable,
		apitypes.LayerBlockchainService,
		"服务暂时不可用，请稍后重试。",
		detail,
		503,
		details,
	)
}

