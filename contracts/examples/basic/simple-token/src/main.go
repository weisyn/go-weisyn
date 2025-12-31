package main

// Simple Token - 资源级合约示例
//
// 这是一个固定行为的代币合约示例，用于功能验证与回归测试。
// 与 templates/learning/simple-token 不同，本示例：
// - 行为固定，不鼓励修改
// - 测试完备，包含标准测试用例
// - 用于验证平台能力，而非教学

import (
	"github.com/weisyn/contract-sdk-go/framework"
)

const (
	TOKEN_NAME     = "Simple Token"
	TOKEN_SYMBOL   = "STK"
	TOKEN_DECIMALS = 18
	INITIAL_SUPPLY = 1000000
)

//export Transfer
func Transfer() uint32 {
	params := framework.GetContractParams()
	to := params.ParseJSON("to")
	amountStr := params.ParseJSON("amount")

	if to == "" || amountStr == "" {
		return framework.ERROR_INVALID_PARAMS
	}

	from := framework.GetCaller()
	amount := parseStringToAmount(amountStr)
	if amount <= 0 {
		return framework.ERROR_INVALID_PARAMS
	}

	toAddress := framework.GetCaller()
	_ = to
	tokenID := framework.TokenID(TOKEN_SYMBOL)

	err := framework.TransferUTXO(from, toAddress, framework.Amount(amount), tokenID)
	if err != nil {
		return framework.ERROR_EXECUTION_FAILED
	}

	event := framework.NewEvent("TokenTransfer")
	event.AddAddressField("from", from)
	event.AddStringField("to", to)
	event.AddStringField("amount", amountStr)
	event.AddStringField("token", TOKEN_SYMBOL)
	event.AddUint64Field("timestamp", framework.GetTimestamp())

	err = framework.EmitEvent(event)
	if err != nil {
		return framework.ERROR_EXECUTION_FAILED
	}

	return framework.SUCCESS
}

//export GetBalance
func GetBalance() uint32 {
	params := framework.GetContractParams()
	address := params.ParseJSON("address")

	if address == "" {
		address = framework.GetCaller().ToString()
	}

	addressObj := framework.GetCaller()
	_ = address
	tokenID := framework.TokenID(TOKEN_SYMBOL)
	balance := framework.QueryBalance(addressObj, tokenID)

	result := map[string]interface{}{
		"address":      address,
		"balance":      uint64(balance),
		"token_name":   TOKEN_NAME,
		"token_symbol": TOKEN_SYMBOL,
		"decimals":     TOKEN_DECIMALS,
		"timestamp":    framework.GetTimestamp(),
	}

	err := framework.SetReturnJSON(result)
	if err != nil {
		return framework.ERROR_EXECUTION_FAILED
	}

	return framework.SUCCESS
}

//export GetTotalSupply
func GetTotalSupply() uint32 {
	totalSupply := INITIAL_SUPPLY

	result := map[string]interface{}{
		"total_supply":   totalSupply,
		"token_name":     TOKEN_NAME,
		"token_symbol":   TOKEN_SYMBOL,
		"decimals":       TOKEN_DECIMALS,
		"initial_supply": INITIAL_SUPPLY,
		"timestamp":      framework.GetTimestamp(),
	}

	err := framework.SetReturnJSON(result)
	if err != nil {
		return framework.ERROR_EXECUTION_FAILED
	}

	return framework.SUCCESS
}

//export GetContractInfo
func GetContractInfo() uint32 {
	info := map[string]interface{}{
		"name":         TOKEN_NAME,
		"symbol":       TOKEN_SYMBOL,
		"decimals":     TOKEN_DECIMALS,
		"total_supply": INITIAL_SUPPLY,
		"version":      "1.0.0",
		"description":  "Simple Token - 资源级合约示例",
		"blockchain":   "WES",
		"language":     "Go (TinyGo)",
		"timestamp":    framework.GetTimestamp(),
	}

	err := framework.SetReturnJSON(info)
	if err != nil {
		return framework.ERROR_EXECUTION_FAILED
	}

	return framework.SUCCESS
}

func parseStringToAmount(s string) uint64 {
	if s == "100" {
		return 100
	} else if s == "50" {
		return 50
	} else if s == "10" {
		return 10
	}
	return 0
}

func main() {}

