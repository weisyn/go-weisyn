package main

import (
	"context"
	"encoding/base64"
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
	aiModelHash    string
	aiInputsFile   string
	aiInputsJSON   string
	aiPrivateKey   string
	aiFrom         string
	aiOnnxFile     string
	aiModelName    string
	aiModelDesc    string
)

// aiCmd AI模型相关命令
var aiCmd = &cobra.Command{
	Use:   "ai",
	Short: "AI模型管理",
	Long:  "调用已部署的AI模型进行推理",
}

// aiCallCmd 调用AI模型
var aiCallCmd = &cobra.Command{
	Use:   "call <model-hash>",
	Short: "调用AI模型",
	Long: `调用已部署的AI模型进行推理

支持多种数据类型输入：
- float32: 使用 data 字段
- int64: 使用 int64_data 字段（用于文本模型）
- uint8: 使用 uint8_data 字段（用于图像原始数据）

示例：
  # 使用JSON文件指定输入
  wes ai call 0x1234... --inputs-file inputs.json --private-key 0x...

  # 使用命令行JSON指定输入
  wes ai call 0x1234... --inputs '[{"data": [1.0, 2.0], "shape": [1, 2]}]' --private-key 0x...

输入JSON格式：
  [
    {
      "name": "input",              // 可选：输入名称
      "data": [1.0, 2.0, ...],      // float32类型数据
      "int64_data": [101, 2023],    // 可选：int64类型数据
      "uint8_data": [255, 128],     // 可选：uint8类型数据
      "shape": [1, 3, 224, 224],    // 形状信息
      "data_type": "float32"        // 可选：数据类型
    }
  ]`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelHash := args[0]

		// 移除0x前缀（如果有）
		modelHash = strings.TrimPrefix(modelHash, "0x")

		// 验证modelHash格式（64位hex）
		if len(modelHash) != 64 {
			return fmt.Errorf("无效的模型哈希长度: 期望64位十六进制字符，实际%d", len(modelHash))
		}

		// 验证hex格式
		if _, err := hex.DecodeString(modelHash); err != nil {
			return fmt.Errorf("无效的模型哈希格式: %w", err)
		}

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

		// 解析输入数据
		var inputs []map[string]interface{}
		if aiInputsFile != "" {
			// 从文件读取
			inputBytes, err := os.ReadFile(aiInputsFile)
			if err != nil {
				return fmt.Errorf("读取输入文件失败: %w", err)
			}
			if err := json.Unmarshal(inputBytes, &inputs); err != nil {
				return fmt.Errorf("解析输入文件JSON失败: %w", err)
			}
		} else if aiInputsJSON != "" {
			// 从命令行参数读取
			if err := json.Unmarshal([]byte(aiInputsJSON), &inputs); err != nil {
				return fmt.Errorf("解析输入JSON失败: %w", err)
			}
		} else {
			return fmt.Errorf("必须提供输入数据（--inputs-file 或 --inputs）")
		}

		if len(inputs) == 0 {
			return fmt.Errorf("输入数据不能为空")
		}

		formatter.PrintInfo(fmt.Sprintf("调用AI模型: %s", modelHash))
		formatter.PrintInfo(fmt.Sprintf("输入张量数量: %d", len(inputs)))

		// 获取私钥
		privateKey := aiPrivateKey
		if privateKey == "" && aiFrom != "" {
			// TODO: 从钱包获取私钥（如果实现了钱包管理）
			return fmt.Errorf("必须提供私钥（--private-key）")
		}
		if privateKey == "" {
			return fmt.Errorf("必须提供私钥（--private-key）")
		}

		// 移除私钥的0x前缀
		privateKey = strings.TrimPrefix(privateKey, "0x")

		// 构建调用请求
		callReq := &transport.CallAIModelRequest{
			PrivateKey: "0x" + privateKey,
			ModelHash:  "0x" + modelHash,
			Inputs:     inputs,
		}

		// 执行调用
		result, err := client.CallAIModel(ctx, callReq)
		if err != nil {
			return fmt.Errorf("调用AI模型失败: %w", err)
		}

		if !result.Success {
			formatter.PrintError(fmt.Errorf("调用失败: %s", result.Message))
			return fmt.Errorf("调用失败")
		}

		formatter.PrintSuccess("AI模型调用成功")
		if result.TxHash != "" {
			formatter.PrintInfo(fmt.Sprintf("交易哈希: %s", result.TxHash))
		}
		formatter.PrintInfo(fmt.Sprintf("输出张量数量: %d", len(result.TensorOutputs)))

		// 格式化输出
		output := map[string]interface{}{
			"success":       result.Success,
			"message":       result.Message,
			"tensor_outputs": result.TensorOutputs,
			"output_count":  len(result.TensorOutputs),
		}

		if result.TxHash != "" {
			output["tx_hash"] = result.TxHash
		}

		// 显示输出形状信息
		if len(result.TensorOutputs) > 0 {
			shapes := make([][]int, len(result.TensorOutputs))
			for i, outputTensor := range result.TensorOutputs {
				shapes[i] = []int{len(outputTensor.Values)}
			}
			output["output_shapes"] = shapes
		}

		return formatter.Print(output)
	},
}

// aiDeployCmd 部署AI模型
var aiDeployCmd = &cobra.Command{
	Use:   "deploy <onnx-file>",
	Short: "部署AI模型",
	Long: `部署ONNX模型到区块链

示例：
  wes ai deploy model.onnx --name "MyModel" --private-key 0x...
  wes ai deploy model.onnx --name "MyModel" --description "模型描述" --private-key 0x...`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		onnxFile := args[0]

		// 读取ONNX文件
		onnxBytes, err := os.ReadFile(onnxFile)
		if err != nil {
			return fmt.Errorf("读取ONNX文件失败: %w", err)
		}

		// 验证ONNX文件大小
		if len(onnxBytes) < 16 {
			return fmt.Errorf("无效的ONNX文件：文件太小")
		}

		formatter.PrintInfo(fmt.Sprintf("ONNX文件大小: %d bytes", len(onnxBytes)))

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

		// 参数校验
		if aiModelName == "" {
			return fmt.Errorf("必须提供模型名称（--name）")
		}
		if aiPrivateKey == "" {
			return fmt.Errorf("必须提供私钥（--private-key）")
		}

		// 移除私钥的0x前缀
		privateKey := strings.TrimPrefix(aiPrivateKey, "0x")

		// Base64编码ONNX内容
		onnxContentBase64 := base64.StdEncoding.EncodeToString(onnxBytes)

		formatter.PrintInfo(fmt.Sprintf("部署AI模型: %s", aiModelName))

		// 构建部署请求
		deployReq := &transport.DeployAIModelRequest{
			PrivateKey:  "0x" + privateKey,
			OnnxContent: onnxContentBase64,
			Name:        aiModelName,
			Description: aiModelDesc,
		}

		// 执行部署
		result, err := client.DeployAIModel(ctx, deployReq)
		if err != nil {
			return fmt.Errorf("部署AI模型失败: %w", err)
		}

		if !result.Success {
			formatter.PrintError(fmt.Errorf("部署失败: %s", result.Message))
			return fmt.Errorf("部署失败")
		}

		formatter.PrintSuccess("AI模型部署成功")
		formatter.PrintInfo(fmt.Sprintf("模型ID: %s", result.ContentHash))
		formatter.PrintInfo(fmt.Sprintf("交易哈希: %s", result.TxHash))

		output := map[string]interface{}{
			"content_hash": result.ContentHash,
			"tx_hash":      result.TxHash,
			"success":      result.Success,
			"message":      result.Message,
			"model_name":   aiModelName,
			"file_size":    len(onnxBytes),
		}

		if aiModelDesc != "" {
			output["description"] = aiModelDesc
		}

		return formatter.Print(output)
	},
}

func init() {
	aiCmd.AddCommand(aiCallCmd)
	aiCmd.AddCommand(aiDeployCmd)

	// call 标志
	aiCallCmd.Flags().StringVar(&aiInputsFile, "inputs-file", "", "输入数据文件路径 (JSON格式)")
	aiCallCmd.Flags().StringVar(&aiInputsJSON, "inputs", "", "输入数据 (JSON格式)")
	aiCallCmd.Flags().StringVar(&aiPrivateKey, "private-key", "", "私钥 (hex格式)")
	aiCallCmd.Flags().StringVar(&aiFrom, "from", "", "调用者地址（暂未实现钱包管理）")

	// deploy 标志
	aiDeployCmd.Flags().StringVar(&aiModelName, "name", "", "模型名称（必需）")
	aiDeployCmd.Flags().StringVar(&aiModelDesc, "description", "", "模型描述（可选）")
	aiDeployCmd.Flags().StringVar(&aiPrivateKey, "private-key", "", "私钥 (hex格式)")

	// 标记互斥标志
	aiCallCmd.MarkFlagsMutuallyExclusive("inputs-file", "inputs")
}

