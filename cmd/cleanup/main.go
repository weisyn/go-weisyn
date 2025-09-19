package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var (
		dryRun   = flag.Bool("dry-run", false, "仅显示将要删除的文件，不实际删除")
		confirm  = flag.Bool("yes", false, "跳过确认提示，直接删除")
		showHelp = flag.Bool("help", false, "显示帮助信息")
	)
	flag.Parse()

	if *showHelp {
		showUsage()
		return
	}

	fmt.Println("🧹 WES数据清理工具")
	fmt.Println("==================")

	// 查找可能的数据目录
	dataDirs := findDataDirectories()

	if len(dataDirs) == 0 {
		fmt.Println("✅ 未发现任何数据目录")
		return
	}

	fmt.Printf("发现 %d 个数据目录:\n\n", len(dataDirs))

	totalSize := int64(0)
	for i, dir := range dataDirs {
		size, err := getDirSize(dir)
		if err != nil {
			fmt.Printf("%d. %s (大小计算失败: %v)\n", i+1, dir, err)
		} else {
			fmt.Printf("%d. %s (%s)\n", i+1, dir, formatBytes(size))
			totalSize += size
		}
	}

	fmt.Printf("\n总大小: %s\n\n", formatBytes(totalSize))

	if *dryRun {
		fmt.Println("🔍 预览模式 - 以下目录将被删除:")
		for _, dir := range dataDirs {
			fmt.Printf("  - %s\n", dir)
		}
		return
	}

	if !*confirm {
		fmt.Print("⚠️ 确认删除所有数据目录? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("操作已取消")
			return
		}
	}

	// 执行清理
	fmt.Println("🗑️  开始清理...")
	for _, dir := range dataDirs {
		fmt.Printf("删除: %s... ", dir)
		if err := os.RemoveAll(dir); err != nil {
			fmt.Printf("失败: %v\n", err)
		} else {
			fmt.Println("成功")
		}
	}

	fmt.Println("\n✅ 清理完成！")
}

func findDataDirectories() []string {
	var dirs []string

	// 常见的数据目录位置
	candidates := []string{
		"./data",
		"./data/badger",
		"./internal/core/infrastructure/storage/badger/data",
		// 启动配置临时目录
		"./config-temp",
	}

	// 检查每个候选目录
	for _, candidate := range candidates {
		if absPath, err := filepath.Abs(candidate); err == nil {
			if info, err := os.Stat(absPath); err == nil && info.IsDir() {
				// 检查目录是否包含区块链数据
				if isBlockchainDataDir(absPath) {
					dirs = append(dirs, absPath)
				}
			}
		}
	}

	// 查找临时配置文件
	if matches, err := filepath.Glob("./config-temp/weisyn-*-config-*.json"); err == nil {
		for _, match := range matches {
			if absPath, err := filepath.Abs(match); err == nil {
				dirs = append(dirs, absPath)
			}
		}
	}

	return dirs
}

func isBlockchainDataDir(dir string) bool {
	// 检查是否包含BadgerDB特征文件
	badgerFiles := []string{"MANIFEST", "KEYREGISTRY", "BADGER_RUNNING"}
	for _, file := range badgerFiles {
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			return true
		}
	}

	// 检查是否为data目录结构
	if strings.HasSuffix(dir, "/data") || strings.HasSuffix(dir, "\\data") {
		return true
	}

	// 检查是否为badger目录
	if strings.Contains(dir, "badger") {
		return true
	}

	// 检查是否为tmp目录且包含临时文件
	if strings.Contains(dir, "tmp") {
		return true
	}

	return false
}

func getDirSize(dir string) (int64, error) {
	var size int64
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func showUsage() {
	fmt.Println("WES数据清理工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  go run ./cmd/cleanup [选项]")
	fmt.Println("  ./bin/cleanup [选项]")
	fmt.Println()
	fmt.Println("选项:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  go run ./cmd/cleanup --dry-run    # 预览要删除的文件")
	fmt.Println("  go run ./cmd/cleanup --yes        # 直接删除，不询问确认")
	fmt.Println("  go run ./cmd/cleanup              # 交互式删除")
}
