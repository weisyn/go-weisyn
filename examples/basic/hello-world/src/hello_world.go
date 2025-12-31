// WES 智能合约入门示例 - Hello World
//
// 🎯 学习目标：
// ✅ 理解合约的三种基本交互模式：简单返回、链上查询、带参数查询
// ✅ 掌握 Results（状态码）、ReturnData（业务数据）、Events（日志）的区别
// ✅ 学习如何与区块链状态（高度、时间戳、余额）交互

package main

import (
	"github.com/weisyn/contract-sdk-go/framework"
)

// ==================== Hello - 最简单的返回字符串 ====================
//
// 🎯 功能：返回一个问候字符串，验证合约→返回数据→CLI展示的最短路径
//
// 💡 调用方式：无参数，直接调用
//
// 📋 返回说明：
//   - Results[0] = framework.SUCCESS (0)
//   - ReturnData = "Hello, WES!" (UTF-8字符串)
//   - Events = 无
//
//export Hello
func Hello() uint32 {
	greeting := "Hello, WES!"

	if err := framework.SetReturnString(greeting); err != nil {
		return framework.ERROR_EXECUTION_FAILED
	}

	return framework.SUCCESS
}

// ==================== ChainStatus - 链上状态查询 ====================
//
// 🎯 功能：查询并返回链上核心信息（高度、时间戳、调用者、余额）
//
// 💡 调用方式：无参数，直接调用
//
// 📋 返回说明：
//   - Results[0] = framework.SUCCESS (0)
//   - ReturnData = JSON格式：
//     {
//     "block_height": 12345,
//     "timestamp": 1700000000,
//     "caller": "0x1234...",
//     "caller_balance": 1000000
//     }
//   - Events = 无
//
//export ChainStatus
func ChainStatus() uint32 {
	// 获取区块高度
	blockHeight := framework.GetBlockHeight()

	// 获取时间戳
	timestamp := framework.GetTimestamp()

	// 获取调用者地址
	caller := framework.GetCaller()

	// 获取调用者余额
	callerBalance := framework.QueryBalance(caller, "")

	// 构建JSON响应
	statusData := map[string]interface{}{
		"block_height":   blockHeight,
		"timestamp":      timestamp,
		"caller":         caller.ToString(),
		"caller_balance": callerBalance,
	}

	if err := framework.SetReturnJSON(statusData); err != nil {
		return framework.ERROR_EXECUTION_FAILED
	}

	return framework.SUCCESS
}

// ==================== Inspect - 带参数的动态查询 ====================
//
// 🎯 功能：根据 action 参数执行不同的查询操作
//
// 💡 调用方式：通过 Payload（initParams）传入 JSON
//
//	示例1：{"action":"block_height"}
//	示例2：{"action":"balance"}
//	示例3：{"action":"balance","address":"0x..."}
//
// 📋 返回说明：
//   - Results[0] = framework.SUCCESS (0) 或 framework.ERROR_INVALID_PARAMS (1)
//   - ReturnData = JSON格式，根据 action 不同而变化
//   - Events = 无
//
//export Inspect
func Inspect() uint32 {
	// 获取合约调用参数（JSON负载）
	params := framework.GetContractParams()

	// 解析 action 字段
	action := params.ParseJSON("action")
	if action == "" {
		// action 缺失，返回错误信息
		errorResp := map[string]interface{}{
			"error": "missing required field: action",
		}
		framework.SetReturnJSON(errorResp)
		return framework.ERROR_INVALID_PARAMS
	}

	// 根据 action 执行不同操作
	switch action {
	case "block_height":
		// 返回当前区块高度
		height := framework.GetBlockHeight()
		resp := map[string]interface{}{
			"action": "block_height",
			"result": height,
		}
		if err := framework.SetReturnJSON(resp); err != nil {
			return framework.ERROR_EXECUTION_FAILED
		}

	case "balance":
		// 查询余额（address 可选，缺省则用调用者）
		addressStr := params.GetStringOr("address", "")
		var targetAddr framework.Address

		if addressStr == "" {
			// 使用调用者地址
			targetAddr = framework.GetCaller()
		} else {
			// 尝试解析 Base58Check 地址（推荐）
			parsedAddr, err := framework.ParseAddressBase58(addressStr)
			if err != nil {
				// 如果 Base58 解析失败，尝试 hex 格式（兼容旧代码）
				parsedAddr, err = framework.ParseAddressFromHex(addressStr)
				if err != nil {
					// 两种格式都失败，返回错误响应
					errorResp := map[string]interface{}{
						"error":   "invalid address format",
						"address": addressStr,
						"hint":    "expected Base58Check (e.g., Cf1Kes...) or 40-char hex (e.g., 0x1234...)",
					}
					framework.SetReturnJSON(errorResp)
					return framework.ERROR_INVALID_PARAMS
				}
			}
			targetAddr = parsedAddr
		}

		balance := framework.QueryBalance(targetAddr, "")
		resp := map[string]interface{}{
			"action":  "balance",
			"address": targetAddr.ToString(),
			"balance": balance,
		}
		if err := framework.SetReturnJSON(resp); err != nil {
			return framework.ERROR_EXECUTION_FAILED
		}

	default:
		// 不支持的 action
		errorResp := map[string]interface{}{
			"error":     "unsupported action",
			"action":    action,
			"supported": []string{"block_height", "balance"},
		}
		framework.SetReturnJSON(errorResp)
		return framework.ERROR_INVALID_PARAMS
	}

	return framework.SUCCESS
}

// ==================== invoke & main ====================
//
// 🎯 说明：
//   - invoke：合约初始化入口（当前未被自动调用，保持空实现）
//   - main：Go编译器要求的程序入口（WASM环境中不会执行，必须保持空的）
//
// ⚠️ 业务逻辑应放在 Hello/ChainStatus/Inspect 等导出函数中

//export invoke
func invoke() uint32 {
	return framework.SUCCESS
}

func main() {
	// 保持空的，业务逻辑在导出函数中实现
}
