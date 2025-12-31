package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/weisyn/v1/client/core/transport"
)

var (
	contractWasmFile string
	contractInitArgs string
	contractMethod   string
	contractArgs     string
	contractFrom     string
	contractValue    string
	contractGasLimit uint64
	contractOutput   string
	contractReadOnly bool
)

// contractCmd 合约相关命令
var contractCmd = &cobra.Command{
	Use:   "contract",
	Short: "智能合约管理",
	Long:  "部署和调用智能合约",
}

// contractDeployCmd 部署合约
var contractDeployCmd = &cobra.Command{
	Use:   "deploy <wasm-file>",
	Short: "部署合约",
	Long:  "部署WASM智能合约到区块链",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wasmFile := args[0]

		// 读取WASM文件
		wasmBytes, err := os.ReadFile(wasmFile)
		if err != nil {
			return fmt.Errorf("读取WASM文件失败: %w", err)
		}

		// 验证WASM文件头（魔数：0x00 0x61 0x73 0x6D）
		if len(wasmBytes) < 4 || wasmBytes[0] != 0x00 || wasmBytes[1] != 0x61 || wasmBytes[2] != 0x73 || wasmBytes[3] != 0x6D {
			return fmt.Errorf("无效的WASM文件：魔数不匹配")
		}

		formatter.PrintInfo(fmt.Sprintf("WASM文件大小: %d bytes", len(wasmBytes)))

		client, err := getClient()
		if err != nil {
			return err
		}
		defer func() {
			if err := client.Close(); err != nil {
				log.Printf("Failed to close client: %v", err)
			}
		}()

		ctx := context.Background()

		// 解析初始化参数
		var initArgsMap map[string]interface{}
		if contractInitArgs != "" {
			if err := json.Unmarshal([]byte(contractInitArgs), &initArgsMap); err != nil {
				return fmt.Errorf("解析初始化参数失败: %w", err)
			}
		}

		// 构建部署交易数据
		deployData := map[string]interface{}{
			"type":      "deploy",
			"wasm_code": "0x" + hex.EncodeToString(wasmBytes),
			"init_args": initArgsMap,
		}

		deployDataBytes, err := json.Marshal(deployData)
		if err != nil {
			return fmt.Errorf("序列化部署数据失败: %w", err)
		}

		// 调用合约部署（使用 Call 接口模拟）
		// 注意: 根据 WES 地址规范，使用 Base58Check 编码的零地址表示合约创建
		// 零地址 (20 字节全零) 的 Base58Check 编码: CGTta3M4t3yXu8uRgkKvaWd2d8DQvDPnpL
		const ContractCreationAddress = "CGTta3M4t3yXu8uRgkKvaWd2d8DQvDPnpL"
		callReq := &transport.CallRequest{
			From:  contractFrom,
			To:    ContractCreationAddress,
			Data:  "0x" + hex.EncodeToString(deployDataBytes),
			Value: contractValue,
		}

		// 估算Gas（模拟调用）
		result, err := client.Call(ctx, callReq, nil)
		if err != nil {
			return fmt.Errorf("部署合约失败: %w", err)
		}

		if !result.Success {
			return fmt.Errorf("部署失败: %s", result.Error)
		}

		// 解析合约地址（假设在output中返回）
		contractAddress := result.Output

		formatter.PrintSuccess("合约部署成功")
		formatter.PrintInfo(fmt.Sprintf("合约地址: %s", contractAddress))
		formatter.PrintInfo(fmt.Sprintf("Gas消耗: %s", result.GasUsed))

		output := map[string]interface{}{
			"contract_address": contractAddress,
			"gas_used":         result.GasUsed,
			"wasm_size":        len(wasmBytes),
		}

		if contractInitArgs != "" {
			output["init_args"] = initArgsMap
		}

		return formatter.Print(output)
	},
}

// contractCallCmd 调用合约
var contractCallCmd = &cobra.Command{
	Use:   "call <contract-address> <method>",
	Short: "调用合约",
	Long:  "调用已部署的智能合约方法",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		contractAddress := args[0]
		method := args[1]

		client, err := getClient()
		if err != nil {
			return err
		}
		defer func() {
			if err := client.Close(); err != nil {
				log.Printf("Failed to close client: %v", err)
			}
		}()

		ctx := context.Background()

		// 解析调用参数
		var argsMap map[string]interface{}
		if contractArgs != "" {
			if err := json.Unmarshal([]byte(contractArgs), &argsMap); err != nil {
				return fmt.Errorf("解析调用参数失败: %w", err)
			}
		}

		// 构建调用数据
		callData := map[string]interface{}{
			"method": method,
			"args":   argsMap,
		}

		callDataBytes, err := json.Marshal(callData)
		if err != nil {
			return fmt.Errorf("序列化调用数据失败: %w", err)
		}

		// 构建调用请求
		callReq := &transport.CallRequest{
			From:  contractFrom,
			To:    contractAddress,
			Data:  "0x" + hex.EncodeToString(callDataBytes),
			Value: contractValue,
		}

		formatter.PrintInfo(fmt.Sprintf("调用合约: %s.%s()", contractAddress, method))

		// 执行调用
		result, err := client.Call(ctx, callReq, nil)
		if err != nil {
			return fmt.Errorf("调用合约失败: %w", err)
		}

		if !result.Success {
			formatter.PrintError(fmt.Errorf("调用失败: %s", result.Error))
			return fmt.Errorf("调用失败")
		}

		formatter.PrintSuccess("调用成功")
		formatter.PrintInfo(fmt.Sprintf("Gas消耗: %s", result.GasUsed))

		// 尝试解析输出为JSON
		var outputData interface{}
		if strings.HasPrefix(result.Output, "0x") {
			// 十六进制输出，尝试解码
			outputBytes, err := hex.DecodeString(strings.TrimPrefix(result.Output, "0x"))
			if err == nil {
				if err := json.Unmarshal(outputBytes, &outputData); err != nil {
					// 无法解析为JSON，直接显示原始输出
					outputData = result.Output
				}
			}
		} else {
			outputData = result.Output
		}

		return formatter.Print(map[string]interface{}{
			"method":    method,
			"gas_used":  result.GasUsed,
			"output":    outputData,
			"read_only": contractReadOnly,
		})
	},
}

// contractQueryCmd 查询合约（只读）
var contractQueryCmd = &cobra.Command{
	Use:   "query <contract-address> <method>",
	Short: "查询合约（只读）",
	Long:  "以只读方式查询智能合约状态，不消耗Gas",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		contractAddress := args[0]
		method := args[1]

		client, err := getClient()
		if err != nil {
			return err
		}
		defer func() {
			if err := client.Close(); err != nil {
				log.Printf("Failed to close client: %v", err)
			}
		}()

		ctx := context.Background()

		// 解析查询参数
		var argsMap map[string]interface{}
		if contractArgs != "" {
			if err := json.Unmarshal([]byte(contractArgs), &argsMap); err != nil {
				return fmt.Errorf("解析查询参数失败: %w", err)
			}
		}

		// 构建查询数据
		queryData := map[string]interface{}{
			"method": method,
			"args":   argsMap,
		}

		queryDataBytes, err := json.Marshal(queryData)
		if err != nil {
			return fmt.Errorf("序列化查询数据失败: %w", err)
		}

		// 构建查询请求（只读，不需要from）
		callReq := &transport.CallRequest{
			To:   contractAddress,
			Data: "0x" + hex.EncodeToString(queryDataBytes),
		}

		formatter.PrintInfo(fmt.Sprintf("查询合约: %s.%s() [只读]", contractAddress, method))

		// 执行查询
		result, err := client.Call(ctx, callReq, nil)
		if err != nil {
			return fmt.Errorf("查询合约失败: %w", err)
		}

		if !result.Success {
			formatter.PrintError(fmt.Errorf("查询失败: %s", result.Error))
			return fmt.Errorf("查询失败")
		}

		// 尝试解析输出为JSON
		var outputData interface{}
		if strings.HasPrefix(result.Output, "0x") {
			outputBytes, err := hex.DecodeString(strings.TrimPrefix(result.Output, "0x"))
			if err == nil {
				if err := json.Unmarshal(outputBytes, &outputData); err != nil {
					outputData = result.Output
				}
			}
		} else {
			outputData = result.Output
		}

		formatter.PrintSuccess("查询成功")

		return formatter.Print(map[string]interface{}{
			"method": method,
			"output": outputData,
		})
	},
}

func init() {
	contractCmd.AddCommand(contractDeployCmd)
	contractCmd.AddCommand(contractCallCmd)
	contractCmd.AddCommand(contractQueryCmd)

	// deploy 标志
	contractDeployCmd.Flags().StringVar(&contractInitArgs, "init-args", "", "初始化参数 (JSON格式)")
	contractDeployCmd.Flags().StringVar(&contractFrom, "from", "", "部署者地址")
	contractDeployCmd.Flags().StringVar(&contractValue, "value", "0", "附带金额")
	contractDeployCmd.Flags().Uint64Var(&contractGasLimit, "gas", 0, "Gas限制")

	// call 标志
	contractCallCmd.Flags().StringVar(&contractArgs, "args", "", "调用参数 (JSON格式)")
	contractCallCmd.Flags().StringVar(&contractFrom, "from", "", "调用者地址")
	contractCallCmd.Flags().StringVar(&contractValue, "value", "0", "附带金额")
	contractCallCmd.Flags().Uint64Var(&contractGasLimit, "gas", 0, "Gas限制")

	// query 标志
	contractQueryCmd.Flags().StringVar(&contractArgs, "args", "", "查询参数 (JSON格式)")
}
