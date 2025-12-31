// Package flows 提供可复用的交互流程
package flows

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/weisyn/v1/client/pkg/ux/ui"
	"github.com/weisyn/v1/pkg/utils"
	"github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
)

// ============================================================================
// ContractFlow - 合约管理交互式流程
//
// 参考旧CLI: _archived/old-internal-cli/internal/cli/presentation/screens/contract_deploy_screen.go
// 提供完整的分步引导式交互体验
// ============================================================================

// ContractFlow 合约管理流程
type ContractFlow struct {
	ui              ui.Components
	contractService ContractService
	walletService   WalletService
}

// NewContractFlow 创建合约管理流程
func NewContractFlow(
	uiComponents ui.Components,
	contractService ContractService,
	walletService WalletService,
) *ContractFlow {
	return &ContractFlow{
		ui:              uiComponents,
		contractService: contractService,
		walletService:   walletService,
	}
}

// ============================================================================
// 合约部署流程（对齐旧CLI的6步流程）
// ============================================================================

// ShowDeployContract 展示合约部署流程（交互式）
//
// 完整流程（对齐旧CLI）：
//   步骤1: 选择钱包
//   步骤2: 验证身份（输入并验证密码）
//   步骤3: 选择WASM文件
//   步骤4: 配置合约执行参数（ABI版本）
//   步骤5: 输入合约元数据（名称、描述）
//   步骤6: 确认并部署
func (f *ContractFlow) ShowDeployContract(ctx context.Context) error {
	f.ui.ShowHeader("📤 部署智能合约")

	// ====== 步骤1: 选择钱包 ======
	f.ui.ShowInfo("💼 步骤 1/6：选择钱包")
	fmt.Println()

	wallets, err := f.walletService.ListWallets(ctx)
	if err != nil {
		f.ui.ShowError("获取钱包列表失败: " + err.Error())
		return fmt.Errorf("获取钱包列表失败: %w", err)
	}

	if len(wallets) == 0 {
		f.ui.ShowWarning("暂无钱包，请先创建钱包")
		fmt.Println()
		f.ui.ShowInfo("💡 提示：返回主菜单 → 账户管理 → 创建账户")
		return fmt.Errorf("暂无钱包")
	}

	// 构建钱包选项
	walletNames := make([]string, len(wallets))
	for i, w := range wallets {
		defaultTag := ""
		if w.IsDefault {
			defaultTag = " [默认]"
		}
		walletNames[i] = fmt.Sprintf("%s - %s%s", w.Name, w.Address, defaultTag)
	}

	selectedIdx, err := f.ui.ShowMenu("选择钱包", walletNames)
	if err != nil {
		f.ui.ShowError("选择失败: " + err.Error())
		return fmt.Errorf("选择钱包失败: %w", err)
	}

	selectedWallet := wallets[selectedIdx]
	f.ui.ShowSuccess(fmt.Sprintf("✅ 已选择钱包: %s", selectedWallet.Name))
	fmt.Println()

	// ====== 步骤2: 验证身份 ======
	f.ui.ShowInfo("🔐 步骤 2/6：验证身份")
	fmt.Println()

	password, err := f.ui.ShowInputDialog("钱包密码", "请输入钱包密码以解锁私钥", true)
	if err != nil {
		return fmt.Errorf("输入密码失败: %w", err)
	}

	// 🔒 立即验证密码（避免用户填写完所有信息后才发现密码错误）
	_, err = f.walletService.ExportPrivateKey(ctx, selectedWallet.Name, password)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("❌ 密码验证失败: %v", err))
		fmt.Println()
		f.ui.ShowWarning("💡 请检查密码是否正确")
		return fmt.Errorf("密码验证失败: %w", err)
	}

	f.ui.ShowSuccess("✅ 密码验证成功")
	fmt.Println()

	// ====== 步骤3: 选择WASM文件 ======
	f.ui.ShowInfo("📁 步骤 3/6：选择WASM文件")
	fmt.Println()

	filePath, err := f.ui.ShowInputDialog("WASM文件路径", "请输入WASM合约文件的完整路径", false)
	if err != nil {
		return fmt.Errorf("输入文件路径失败: %w", err)
	}

	// 验证文件存在性
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("❌ 文件不存在或无法访问: %v", err))
		return fmt.Errorf("文件访问失败: %w", err)
	}

	if fileInfo.IsDir() {
		f.ui.ShowError("❌ 指定的路径是目录，请指定WASM文件")
		return fmt.Errorf("路径是目录而非文件")
	}

	// 验证WASM文件格式
	wasmBytes, err := os.ReadFile(filePath)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("❌ 读取文件失败: %v", err))
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 验证WASM魔数（0x00 0x61 0x73 0x6D）
	if len(wasmBytes) < 4 || wasmBytes[0] != 0x00 || wasmBytes[1] != 0x61 || wasmBytes[2] != 0x73 || wasmBytes[3] != 0x6D {
		f.ui.ShowError("❌ 无效的WASM文件：魔数不匹配")
		return fmt.Errorf("无效的WASM文件")
	}

	f.ui.ShowSuccess(fmt.Sprintf("✅ 文件找到: %s", filePath))
	f.ui.ShowInfo(fmt.Sprintf("  • 文件大小: %s", formatFileSize(fileInfo.Size())))
	fmt.Println()

	// ====== 步骤4: 配置合约执行参数 ======
	f.ui.ShowInfo("⚙️  步骤 4/6：配置合约执行参数")
	fmt.Println()

	abiVersion, err := f.ui.ShowInputDialog("ABI版本", "请输入ABI版本（留空默认: v1）", false)
	if err != nil {
		return fmt.Errorf("输入ABI版本失败: %w", err)
	}
	if abiVersion == "" {
		abiVersion = "v1"
	}

	f.ui.ShowInfo("")
	f.ui.ShowInfo("💡 提示：导出函数将由服务端自动解析，确保准确性与安全性")
	fmt.Println()

	config := &resource.ContractExecutionConfig{
		AbiVersion: abiVersion,
		// ExportedFunctions: 留空，由服务端自动解析WASM并填充
	}

	// ====== 步骤5: 输入合约元数据 ======
	f.ui.ShowInfo("📝 步骤 5/6：合约元数据")
	fmt.Println()

	// 从文件路径提取文件名（不含扩展名）作为默认合约名称
	defaultName := extractContractNameFromPath(filePath)

	var name string
	for {
		prompt := fmt.Sprintf("请输入合约名称（留空使用默认: %s）", defaultName)
		name, err = f.ui.ShowInputDialog("合约名称", prompt, false)
		if err != nil {
			return fmt.Errorf("输入合约名称失败: %w", err)
		}
		name = strings.TrimSpace(name)

		// 如果用户留空，使用默认文件名
		if name == "" {
			name = defaultName
			f.ui.ShowInfo(fmt.Sprintf("💡 使用默认合约名称: %s", name))
			fmt.Println()
		}

		// 验证名称合法性
		if name == "" {
			f.ui.ShowError("❌ 合约名称不能为空，请重新输入")
			fmt.Println()
			continue
		}

		break
	}

	description, err := f.ui.ShowInputDialog("合约描述", "请输入合约描述（可选，直接回车跳过）", false)
	if err != nil {
		return fmt.Errorf("输入合约描述失败: %w", err)
	}

	// ====== 步骤6: 显示部署摘要并确认 ======
	fmt.Println()
	f.ui.ShowInfo("📋 步骤 6/6：确认部署")
	fmt.Println()
	f.ui.ShowInfo("部署摘要：")
	f.ui.ShowInfo(fmt.Sprintf("  • 钱包: %s", selectedWallet.Name))
	f.ui.ShowInfo(fmt.Sprintf("  • 地址: %s", selectedWallet.Address))
	f.ui.ShowInfo(fmt.Sprintf("  • 文件: %s", filePath))
	f.ui.ShowInfo(fmt.Sprintf("  • 大小: %s", formatFileSize(fileInfo.Size())))
	f.ui.ShowInfo(fmt.Sprintf("  • 名称: %s", name))
	if description != "" {
		f.ui.ShowInfo(fmt.Sprintf("  • 描述: %s", description))
	}
	f.ui.ShowInfo(fmt.Sprintf("  • ABI版本: %s", config.AbiVersion))
	f.ui.ShowInfo("  • 导出函数: 由服务端自动解析")
	fmt.Println()

	confirmed, err := f.ui.ShowConfirmDialog("确认部署", "确认部署此合约吗？")
	if err != nil || !confirmed {
		f.ui.ShowWarning("❌ 部署已取消")
		return nil
	}

	// ====== 执行部署 ======
	fmt.Println()
	f.ui.ShowInfo("📊 正在部署合约...")
	f.ui.ShowInfo("  • 处理中：WASM入库 → 构建交易 → 签名 → 提交网络...")
	fmt.Println()

	spinner := f.ui.ShowSpinner("部署中，请稍候...")
	spinner.Start()

	result, err := f.contractService.DeployContract(ctx, &ContractDeployRequest{
		WalletName:  selectedWallet.Name,
		Password:    password,
		FilePath:    filePath,
		Config:      config,
		Name:        name,
		Description: description,
	})

	spinner.Stop()

	// ====== 显示结果 ======
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("❌ 部署失败: %v", err))
		return fmt.Errorf("部署失败: %w", err)
	}

	f.ui.ShowSuccess("✅ 部署成功！合约已提交到区块链网络")
	fmt.Println()
	f.ui.ShowInfo("📋 合约信息：")
	f.ui.ShowInfo(fmt.Sprintf("  • 合约ID（内容哈希）: %s", result.ContentHash))
	f.ui.ShowInfo(fmt.Sprintf("  • 交易哈希: %s", result.TxHash))
	fmt.Println()
	f.ui.ShowWarning("💡 重要提示：")
	f.ui.ShowInfo("  • 请保存【合约ID】，用于后续调用合约方法")
	f.ui.ShowInfo("  • 合约ID是32字节的内容寻址哈希(ContentHash)")
	f.ui.ShowInfo("  • 合约代码永久存储在区块链上，不可篡改")
	fmt.Println()

	return nil
}

// ============================================================================
// 合约调用流程
// ============================================================================

// ShowCallContract 展示合约调用流程（交互式）
//
// 完整流程：
//   步骤1: 选择钱包
//   步骤2: 验证身份（输入并验证密码）
//   步骤3: 输入合约ID（ContentHash）
//   步骤4: 输入调用方法名
//   步骤5: 输入方法参数
//   步骤6: 确认并调用
func (f *ContractFlow) ShowCallContract(ctx context.Context) error {
	f.ui.ShowHeader("📞 调用智能合约")

	// ====== 步骤1: 选择钱包 ======
	f.ui.ShowInfo("💼 步骤 1/6：选择钱包")
	fmt.Println()

	wallets, err := f.walletService.ListWallets(ctx)
	if err != nil {
		f.ui.ShowError("获取钱包列表失败: " + err.Error())
		return fmt.Errorf("获取钱包列表失败: %w", err)
	}

	if len(wallets) == 0 {
		f.ui.ShowWarning("暂无钱包，请先创建钱包")
		fmt.Println()
		f.ui.ShowInfo("💡 提示：返回主菜单 → 账户管理 → 创建账户")
		return fmt.Errorf("暂无钱包")
	}

	// 构建钱包选项
	walletNames := make([]string, len(wallets))
	for i, w := range wallets {
		defaultTag := ""
		if w.IsDefault {
			defaultTag = " [默认]"
		}
		walletNames[i] = fmt.Sprintf("%s - %s%s", w.Name, w.Address, defaultTag)
	}

	selectedIdx, err := f.ui.ShowMenu("选择钱包", walletNames)
	if err != nil {
		f.ui.ShowError("选择失败: " + err.Error())
		return fmt.Errorf("选择钱包失败: %w", err)
	}

	selectedWallet := wallets[selectedIdx]
	f.ui.ShowSuccess(fmt.Sprintf("✅ 已选择钱包: %s", selectedWallet.Name))
	fmt.Println()

	// ====== 步骤2: 验证身份 ======
	f.ui.ShowInfo("🔐 步骤 2/6：验证身份")
	fmt.Println()

	password, err := f.ui.ShowInputDialog("钱包密码", "请输入钱包密码以解锁私钥", true)
	if err != nil {
		return fmt.Errorf("输入密码失败: %w", err)
	}

	// 🔒 立即验证密码
	_, err = f.walletService.ExportPrivateKey(ctx, selectedWallet.Name, password)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("❌ 密码验证失败: %v", err))
		fmt.Println()
		f.ui.ShowWarning("💡 请检查密码是否正确")
		return fmt.Errorf("密码验证失败: %w", err)
	}

	f.ui.ShowSuccess("✅ 密码验证成功")
	fmt.Println()

	// ====== 步骤3: 输入合约ID ======
	f.ui.ShowInfo("🔗 步骤 3/6：输入合约ID")
	fmt.Println()

	contractIDStr, err := f.ui.ShowInputDialog("合约ID", "请输入合约ID（64位十六进制的ContentHash）", false)
	if err != nil {
		return fmt.Errorf("输入合约ID失败: %w", err)
	}

	// 验证合约ID格式
	contractIDStr = strings.TrimSpace(contractIDStr)
	contractIDStr = strings.TrimPrefix(contractIDStr, "0x") // 兼容 0x 前缀
	
	contractIDBytes, err := hex.DecodeString(contractIDStr)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("❌ 合约ID格式错误（应为64位十六进制）: %v", err))
		return fmt.Errorf("合约ID格式错误: %w", err)
	}

	if len(contractIDBytes) != 32 {
		f.ui.ShowError(fmt.Sprintf("❌ 合约ID长度错误（应为32字节，当前: %d字节）", len(contractIDBytes)))
		return fmt.Errorf("合约ID长度错误")
	}

	f.ui.ShowSuccess(fmt.Sprintf("✅ 合约ID: %s", contractIDStr))
	fmt.Println()

	// ====== 步骤4: 输入调用方法名 ======
	f.ui.ShowInfo("🎯 步骤 4/6：输入调用方法")
	fmt.Println()

	method, err := f.ui.ShowInputDialog("方法名", "请输入要调用的方法名（如: add, get_balance）", false)
	if err != nil {
		return fmt.Errorf("输入方法名失败: %w", err)
	}

	method = strings.TrimSpace(method)
	if method == "" {
		f.ui.ShowError("❌ 方法名不能为空")
		return fmt.Errorf("方法名为空")
	}

	f.ui.ShowSuccess(fmt.Sprintf("✅ 调用方法: %s", method))
	fmt.Println()

	// ====== 步骤5: 选择参数类型并输入 ======
	f.ui.ShowInfo("📝 步骤 5/6：输入方法参数")
	fmt.Println()
	f.ui.ShowInfo("请选择参数类型：")
	f.ui.ShowInfo("  1. 无参数")
	f.ui.ShowInfo("  2. u64数组（如: 100,200,300）")
	f.ui.ShowInfo("  3. JSON payload（如: {\"action\":\"balance\"}）")
	fmt.Println()

	paramTypeStr, err := f.ui.ShowInputDialog("参数类型", "请输入选项 (1/2/3)", false)
	if err != nil {
		return fmt.Errorf("输入参数类型失败: %w", err)
	}

	var params []uint64
	var payload []byte
	paramType := strings.TrimSpace(paramTypeStr)

	switch paramType {
	case "1", "":
		// 无参数
		f.ui.ShowInfo("  • 无参数")

	case "2":
		// u64数组参数
		fmt.Println()
		paramsStr, err := f.ui.ShowInputDialog("u64参数", "请输入参数（逗号分隔，如: 100,200）", false)
		if err != nil {
			return fmt.Errorf("输入参数失败: %w", err)
		}

		paramsStr = strings.TrimSpace(paramsStr)
		if paramsStr != "" {
			paramParts := strings.Split(paramsStr, ",")
			params = make([]uint64, 0, len(paramParts))
			for _, part := range paramParts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				var val uint64
				_, err := fmt.Sscanf(part, "%d", &val)
				if err != nil {
					f.ui.ShowError(fmt.Sprintf("❌ 参数解析失败: %v", err))
					return fmt.Errorf("参数解析失败: %w", err)
				}
				params = append(params, val)
			}
			f.ui.ShowSuccess(fmt.Sprintf("✅ u64参数: %v", params))
		}

	case "3":
		// JSON payload
		fmt.Println()
		f.ui.ShowInfo("💡 JSON示例:")
		f.ui.ShowInfo("  • 查询区块高度: {\"action\":\"block_height\"}")
		f.ui.ShowInfo("  • 查询余额: {\"action\":\"balance\"}")
		fmt.Println()

		payloadStr, err := f.ui.ShowInputDialog("JSON Payload", "请输入JSON格式参数", false)
		if err != nil {
			return fmt.Errorf("输入payload失败: %w", err)
		}

		payloadStr = strings.TrimSpace(payloadStr)
		if payloadStr != "" {
			payload = []byte(payloadStr)
			f.ui.ShowSuccess(fmt.Sprintf("✅ JSON Payload: %s", payloadStr))
		}

	default:
		f.ui.ShowError("❌ 无效的参数类型")
		return fmt.Errorf("无效的参数类型: %s", paramType)
	}

	fmt.Println()

	// ====== 步骤6: 确认并调用 ======
	f.ui.ShowInfo("📋 步骤 6/6：确认调用")
	fmt.Println()
	f.ui.ShowInfo("调用摘要：")
	f.ui.ShowInfo(fmt.Sprintf("  • 钱包: %s", selectedWallet.Name))
	f.ui.ShowInfo(fmt.Sprintf("  • 合约ID: %s", contractIDStr))
	f.ui.ShowInfo(fmt.Sprintf("  • 方法: %s", method))
	if len(payload) > 0 {
		f.ui.ShowInfo(fmt.Sprintf("  • Payload: %s", string(payload)))
	} else if len(params) > 0 {
		f.ui.ShowInfo(fmt.Sprintf("  • 参数: %v", params))
	} else {
		f.ui.ShowInfo("  • 参数: 无")
	}
	fmt.Println()

	confirmed, err := f.ui.ShowConfirmDialog("确认调用", "确认调用此合约方法吗？")
	if err != nil || !confirmed {
		f.ui.ShowWarning("❌ 调用已取消")
		return nil
	}

	// ====== 执行调用 ======
	fmt.Println()
	spinner := f.ui.ShowSpinner("调用中，请稍候...")
	spinner.Start()

	result, err := f.contractService.CallContract(ctx, &ContractCallRequest{
		WalletName:  selectedWallet.Name,
		Password:    password,
		ContentHash: contractIDBytes,
		Method:      method,
		Params:      params,
		Payload:     payload, // ✅ 支持JSON payload
	})

	spinner.Stop()

	// ====== 显示结果 ======
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("❌ 调用失败: %v", err))
		return fmt.Errorf("调用失败: %w", err)
	}

	f.ui.ShowSuccess("✅ 调用成功！交易已提交到区块链网络")
	fmt.Println()
	f.ui.ShowInfo("📋 调用结果：")
	f.ui.ShowInfo(fmt.Sprintf("  • 交易哈希: %s", result.TxHash))
	
	if len(result.Results) > 0 {
		f.ui.ShowInfo(fmt.Sprintf("  • 返回值(u64): %v", result.Results))
	}
	
	if len(result.ReturnData) > 0 {
		f.ui.ShowInfo(fmt.Sprintf("  • 返回数据: %s", formatReturnData(result.ReturnData)))
	}

	if len(result.Events) > 0 {
		f.ui.ShowInfo(fmt.Sprintf("  • 事件数量: %d", len(result.Events)))
		for i, evt := range result.Events {
			f.ui.ShowInfo(fmt.Sprintf("    [%d] 类型: %s", i+1, evt.Type))
		}
	}
	fmt.Println()

	return nil
}

// ============================================================================
// 合约查询流程（只读调用）
// ============================================================================

// ShowQueryContract 展示合约查询流程（交互式）
//
// 完整流程：
//   步骤1: 输入合约ID
//   步骤2: 输入查询方法名
//   步骤3: 输入方法参数
//   步骤4: 执行查询并显示结果
func (f *ContractFlow) ShowQueryContract(ctx context.Context) error {
	f.ui.ShowHeader("🔍 查询智能合约（只读）")

	// ====== 步骤1: 输入合约ID ======
	f.ui.ShowInfo("🔗 步骤 1/4：输入合约ID")
	fmt.Println()

	contractIDStr, err := f.ui.ShowInputDialog("合约ID", "请输入合约ID（64位十六进制的ContentHash）", false)
	if err != nil {
		return fmt.Errorf("输入合约ID失败: %w", err)
	}

	// 验证合约ID格式
	contractIDStr = strings.TrimSpace(contractIDStr)
	contractIDStr = strings.TrimPrefix(contractIDStr, "0x")
	
	_, err = hex.DecodeString(contractIDStr)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("❌ 合约ID格式错误: %v", err))
		return fmt.Errorf("合约ID格式错误: %w", err)
	}

	f.ui.ShowSuccess(fmt.Sprintf("✅ 合约ID: %s", contractIDStr))
	fmt.Println()

	// ====== 步骤2: 输入查询方法名 ======
	f.ui.ShowInfo("🎯 步骤 2/4：输入查询方法")
	fmt.Println()

	method, err := f.ui.ShowInputDialog("方法名", "请输入要查询的方法名", false)
	if err != nil {
		return fmt.Errorf("输入方法名失败: %w", err)
	}

	method = strings.TrimSpace(method)
	if method == "" {
		f.ui.ShowError("❌ 方法名不能为空")
		return fmt.Errorf("方法名为空")
	}

	f.ui.ShowSuccess(fmt.Sprintf("✅ 查询方法: %s", method))
	fmt.Println()

	// ====== 步骤3: 输入方法参数 ======
	f.ui.ShowInfo("📝 步骤 3/4：输入方法参数")
	fmt.Println()

	paramsStr, err := f.ui.ShowInputDialog("方法参数", "请输入参数（u64数组，逗号分隔，无参数直接回车）", false)
	if err != nil {
		return fmt.Errorf("输入参数失败: %w", err)
	}

	var params []uint64
	paramsStr = strings.TrimSpace(paramsStr)
	if paramsStr != "" {
		paramParts := strings.Split(paramsStr, ",")
		params = make([]uint64, 0, len(paramParts))
		for _, part := range paramParts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			var val uint64
			_, err := fmt.Sscanf(part, "%d", &val)
			if err != nil {
				f.ui.ShowError(fmt.Sprintf("❌ 参数解析失败: %v", err))
				return fmt.Errorf("参数解析失败: %w", err)
			}
			params = append(params, val)
		}
	}

	if len(params) > 0 {
		f.ui.ShowSuccess(fmt.Sprintf("✅ 参数: %v", params))
	} else {
		f.ui.ShowInfo("  • 无参数")
	}
	fmt.Println()

	// ====== 步骤4: 执行查询 ======
	f.ui.ShowInfo("📊 步骤 4/4：执行查询")
	fmt.Println()

	spinner := f.ui.ShowSpinner("查询中，请稍候...")
	spinner.Start()

	result, err := f.contractService.QueryContract(ctx, &ContractQueryRequest{
		ContentHash: contractIDStr,
		Method:      method,
		Params:      params,
	})

	spinner.Stop()

	// ====== 显示结果 ======
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("❌ 查询失败: %v", err))
		return fmt.Errorf("查询失败: %w", err)
	}

	f.ui.ShowSuccess("✅ 查询成功（只读调用，不消耗Gas）")
	fmt.Println()
	f.ui.ShowInfo("📋 查询结果：")
	
	if len(result.Results) > 0 {
		f.ui.ShowInfo(fmt.Sprintf("  • 返回值(u64): %v", result.Results))
	}
	
	if len(result.ReturnData) > 0 {
		f.ui.ShowInfo(fmt.Sprintf("  • 返回数据: %s", formatReturnData(result.ReturnData)))
	}

	if result.Metadata != nil && len(result.Metadata) > 0 {
		f.ui.ShowInfo("  • 元数据:")
		for k, v := range result.Metadata {
			f.ui.ShowInfo(fmt.Sprintf("    %s: %v", k, v))
		}
	}
	fmt.Println()

	return nil
}

// ============================================================================
// 辅助函数
// ============================================================================

// extractContractNameFromPath 从文件路径提取合约名称
//
// 提取规则：
//   - 提取文件名（不含扩展名）
//   - 去除路径分隔符
//   - 示例："/path/to/hello_world.wasm" → "hello_world"
func extractContractNameFromPath(filePath string) string {
	// 获取文件名（含扩展名）
	fileName := filepath.Base(filePath)

	// 去除扩展名
	ext := filepath.Ext(fileName)
	if ext != "" {
		fileName = strings.TrimSuffix(fileName, ext)
	}

	// 如果提取失败，使用默认值
	if fileName == "" {
		return "UnnamedContract"
	}

	return fileName
}

// formatFileSize 格式化文件大小
func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", size)
	}
}

// formatReturnData 格式化返回数据
func formatReturnData(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	if formatted, ok := tryFormatBalanceJSON(data); ok {
		return formatted
	}

	// 尝试解析为UTF-8字符串
	if isPrintable(data) {
		return string(data)
	}

	// 否则显示十六进制（截断显示前64字节）
	if len(data) > 64 {
		return fmt.Sprintf("0x%s... (%d bytes)", hex.EncodeToString(data[:64]), len(data))
	}
	return fmt.Sprintf("0x%s", hex.EncodeToString(data))
}

func tryFormatBalanceJSON(data []byte) (string, bool) {
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", false
	}

	balanceWei, ok := extractBalanceWei(payload)
	if !ok {
		return "", false
	}

	balanceWES := utils.FormatWeiToDecimal(balanceWei)

	address := ""
	if addr, ok := payload["address"].(string); ok && addr != "" {
		address = addr
	}

	tokenID := ""
	if token, ok := payload["token_id"].(string); ok && token != "" {
		tokenID = token
	}

	parts := make([]string, 0, 3)
	if address != "" {
		parts = append(parts, fmt.Sprintf("地址: %s", address))
	}
	if tokenID != "" {
		parts = append(parts, fmt.Sprintf("Token ID: %s", tokenID))
	}
	parts = append(parts, fmt.Sprintf("余额: %s WES (%d wei)", balanceWES, balanceWei))

	return strings.Join(parts, " | "), true
}

func extractBalanceWei(payload map[string]interface{}) (uint64, bool) {
	if rawWei, exists := payload["balance_wei"]; exists {
		if wei, ok := parseBalanceUint64(rawWei); ok {
			return wei, true
		}
	}

	if rawBalance, exists := payload["balance"]; exists {
		switch v := rawBalance.(type) {
		case string:
			trimmed := strings.TrimSpace(v)
			if trimmed == "" {
				return 0, false
			}
			if strings.ContainsRune(trimmed, '.') {
				if wei, err := utils.ParseDecimalToWei(trimmed); err == nil {
					return wei, true
				}
			}
			return parseBalanceUint64(trimmed)
		default:
			return parseBalanceUint64(v)
		}
	}

	return 0, false
}

func parseBalanceUint64(value interface{}) (uint64, bool) {
	switch v := value.(type) {
	case float64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case string:
		val, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0, false
		}
		return val, true
	case json.Number:
		val, err := v.Int64()
		if err != nil || val < 0 {
			return 0, false
		}
		return uint64(val), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case uint64:
		return v, true
	default:
		return 0, false
	}
}

// isPrintable 检查字节数组是否为可打印字符
func isPrintable(data []byte) bool {
	for _, b := range data {
		if b < 32 || b > 126 {
			return false
		}
	}
	return true
}

