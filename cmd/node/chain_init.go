package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/weisyn/v1/configs"
)

// chainInitCommand 实现 chain init 子命令
func chainInitCommand(args []string) {
	var (
		mode  string // 链模式：consortium | private
		out   string // 输出文件路径
		force bool   // 强制覆盖，跳过交互确认
	)

	fs := flag.NewFlagSet("chain init", flag.ExitOnError)
	fs.StringVar(&mode, "mode", "", "链模式：consortium（联盟链）| private（私链）")
	fs.StringVar(&out, "out", "", "输出文件路径（必需）")
	fs.BoolVar(&force, "force", false, "强制覆盖已存在的文件，跳过交互确认（用于 CI/CD）")
	fs.BoolVar(&force, "yes", false, "同 --force，用于兼容性")
	fs.Usage = func() {
		fmt.Println("用法: weisyn-node chain init --mode <mode> --out <path> [--force]")
		fmt.Println()
		fmt.Println("选项:")
		fmt.Println("  --mode <mode>  链模式：consortium（联盟链）| private（私链）")
		fmt.Println("  --out <path>    输出文件路径（必需）")
		fmt.Println("  --force, --yes 强制覆盖已存在的文件，跳过交互确认（用于 CI/CD）")
		fmt.Println()
		fmt.Println("示例:")
		fmt.Println("  weisyn-node chain init --mode consortium --out ./my-consortium.json")
		fmt.Println("  weisyn-node chain init --mode private --out ./my-private.json --force")
		fmt.Println()
		fmt.Println("注意：")
		fmt.Println("  - 公有链模板由 BaaS 侧负责生成，请通过 BaaS Web 控制台创建公有链实例")
		fmt.Println("  - 官方公有链（prod-public-mainnet）通过 'weisyn-node --chain public' 直接启动（无需 --config）")
	}

	if err := fs.Parse(args); err != nil {
		fmt.Printf("❌ 解析参数失败: %v\n", err)
		os.Exit(1)
	}

	// 验证参数
	if mode == "" {
		fmt.Println("❌ 错误: 必须指定 --mode 参数")
		fs.Usage()
		os.Exit(1)
	}

	mode = toLower(mode)
	if mode != "consortium" && mode != "private" {
		fmt.Printf("❌ 错误: 无效的链模式 '%s'\n", mode)
		fmt.Println("💡 有效选项: consortium | private")
		fmt.Println("💡 注意: 公有链模板由 BaaS 侧负责，请通过 BaaS Web 控制台创建公有链实例")
		os.Exit(1)
	}

	if out == "" {
		fmt.Println("❌ 错误: 必须指定 --out 参数")
		fs.Usage()
		os.Exit(1)
	}

	// 获取模板
	var templateData []byte
	switch mode {
	case "consortium":
		templateData = configs.GetConsortiumChainTemplate()
	case "private":
		templateData = configs.GetPrivateChainTemplate()
	}

	// 格式化JSON（美化输出）
	var templateMap map[string]interface{}
	if err := json.Unmarshal(templateData, &templateMap); err != nil {
		fmt.Printf("❌ 解析模板失败: %v\n", err)
		os.Exit(1)
	}

	formattedData, err := json.MarshalIndent(templateMap, "", "  ")
	if err != nil {
		fmt.Printf("❌ 格式化模板失败: %v\n", err)
		os.Exit(1)
	}

	// 确保输出目录存在
	outDir := filepath.Dir(out)
	if outDir != "." && outDir != "" {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			fmt.Printf("❌ 创建输出目录失败: %v\n", err)
			os.Exit(1)
		}
	}

	// 检查文件是否已存在
	if _, err := os.Stat(out); err == nil {
		if !force {
			fmt.Printf("⚠️  警告: 文件 %s 已存在\n", out)
			fmt.Print("是否覆盖？(y/N): ")
			var response string
			fmt.Scanln(&response)
			if toLower(response) != "y" && toLower(response) != "yes" {
				fmt.Println("已取消")
				return
			}
		} else {
			fmt.Printf("ℹ️  使用 --force 参数，将覆盖已存在的文件: %s\n", out)
		}
	}

	// 写入文件
	if err := os.WriteFile(out, formattedData, 0644); err != nil {
		fmt.Printf("❌ 写入文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 已生成 %s 链配置文件: %s\n", mode, out)
	fmt.Println()
	fmt.Println("⚠️  重要提示:")
	fmt.Println("  1. 请编辑配置文件，修改以下必需字段：")
	fmt.Println("     - environment（运行环境：dev | test | prod）")
	fmt.Println("     - network.chain_id（设置唯一的链ID）")
	fmt.Println("     - network.network_id（设置网络标识符）")
	fmt.Println("     - network.network_namespace（设置网络命名空间）")
	fmt.Println("     - genesis.timestamp（设置创世时间戳）")
	fmt.Println("     - genesis.accounts（添加初始账户）")
	if mode == "consortium" {
		fmt.Println("     - node.host.gater.allow_cidrs（配置联盟成员IP段）")
		fmt.Println("     - node.bootstrap_peers（配置联盟引导节点）")
	}
	fmt.Println()
	fmt.Println("  2. 配置完成后，使用以下命令启动节点：")
	fmt.Printf("     weisyn-node --chain %s --config %s\n", mode, out)
	fmt.Println()
}

func toLower(s string) string {
	result := ""
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result += string(r + 32)
		} else {
			result += string(r)
		}
	}
	return result
}
