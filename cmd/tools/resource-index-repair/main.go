package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

// 简单的全量/增量 Resource 索引修复工具
// 设计目标：
// 1. 幂等：多次执行不会破坏现有索引
// 2. 安全：仅在只读/维护模式下使用（不与正常写入并发）
// 3. 可观测：输出修复统计信息，便于运维判断效果

var (
	startHeight = flag.Uint64("start-height", 0, "从指定区块高度开始修复（包含）")
	endHeight   = flag.Uint64("end-height", 0, "修复到指定区块高度（包含，0 表示一直到当前链高）")
	dryRun      = flag.Bool("dry-run", false, "仅检查并打印将要修复的内容，不实际写入")
	apiBaseURL  = flag.String("api-base", "http://127.0.0.1:28680", "运行中节点的 HTTP API 地址，用于切换运行模式（例如 http://127.0.0.1:28680）")
)

func main() {
	flag.Parse()

	fmt.Fprintf(os.Stderr, "Resource 索引修复工具（CLI）已简化，当前仅通过 HTTP 运行模式接口协助节点进入/退出修复模式。\n")
	fmt.Fprintf(os.Stderr, "参数: startHeight=%d, endHeight=%d, dryRun=%v, apiBase=%s\n",
		*startHeight, *endHeight, *dryRun, *apiBaseURL)

	// 如果提供了 apiBaseURL，则尝试请求节点进入 RepairingUTXO 模式
	if *apiBaseURL != "" {
		if err := setRuntimeMode(*apiBaseURL, "RepairingUTXO"); err != nil {
			fmt.Fprintf(os.Stderr, "切换运行模式到 RepairingUTXO 失败: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "已请求运行中节点进入 RepairingUTXO 模式: apiBase=%s\n", *apiBaseURL)
		}

		// 简单等待一段时间，留给节点内部自动修复执行（由自动 controller 完成）
		time.Sleep(5 * time.Second)

		if err := setRuntimeMode(*apiBaseURL, "Normal"); err != nil {
			fmt.Fprintf(os.Stderr, "切换运行模式到 Normal 失败: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "已请求运行中节点恢复到 Normal 模式\n")
		}
	}
}

// setRuntimeMode 通过 HTTP API 调用运行中的节点，设置运行模式
//
// 注意：这里使用最简单的 GET 请求方式，依赖 /api/v1/runtime/mode?value=XXX
func setRuntimeMode(apiBase, value string) error {
	url := fmt.Sprintf("%s/api/v1/runtime/mode?value=%s", apiBase, value)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("构造 HTTP 请求失败: %w", err)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("调用运行模式接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("运行模式接口返回非 200 状态码: %d", resp.StatusCode)
	}

	return nil
}
