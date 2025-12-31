package launcher

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/weisyn/v1/client/pkg/transport"
)

// NodeProcess 节点子进程句柄
type NodeProcess struct {
	cmd            *exec.Cmd
	endpoint       string
	tempConfigPath string
	logFile        *os.File
	done           chan struct{}
	err            error
	mu             sync.Mutex
}

// LaunchOptions 启动选项
//
// 设计说明：
//   - CLIENT 仅作为 CLI 的可选"可视化启动壳"，不负责生产环境部署
//   - 这里默认启动公共测试网（--chain public）
//   - 配置文件由 GenerateTempNodeConfig 生成，位于当前工作目录的 ./config-temp 下
type LaunchOptions struct {
	// Env 表示运行环境（dev/test/prod），用于写入临时配置中的 environment 字段。
	// 旧值（development/testing/production）已废弃，这里做向后兼容映射。
	Env          string // 运行环境：dev/test/prod（空值默认 dev）
	KeepData     bool   // 是否保留历史数据
	ConfigPath   string // 自定义配置路径（如果指定则不生成临时配置）
	Endpoint     string // API 端点（默认 http://localhost:28680）
	Daemon       bool   // 后台运行（静默模式）
	LogToConsole bool   // 日志输出到控制台（开发模式）
}

// LaunchNode 启动节点子进程
func LaunchNode(ctx context.Context, opts LaunchOptions) (*NodeProcess, error) {
	// 1. 查找节点二进制（weisyn-node）
	nodeBinary, err := findNodeBinary()
	if err != nil {
		return nil, err
	}

	// 2. 准备配置文件
	configPath := opts.ConfigPath
	var tempConfigPath string

	if configPath == "" {
		// 生成临时配置：
		// - 使用内嵌公链测试网配置（test-public-demo），用于本机快速拉起 public 节点
		// - ⚠️ CLI/TUI 只是“外壳”，不应擅自改变节点的数据目录结构；
		//   因此这里默认不覆盖 storage.data_root，让节点遵循自身的数据目录分桶策略（./data/{env}/...）。
		// - 如确需自定义 data_root，可通过 opts.ConfigPath 提供自定义配置文件来完成。
		env := normalizeEnv(opts.Env)
		overrides := ConfigOverrides{
			HTTPPort: 28680,
			GRPCPort: 28682,
			// 不覆盖 DataDir/LogPath：保持节点使用默认数据根目录与日志策略
			DataDir:  "",
			LogPath:  "",
			KeepData: opts.KeepData,
		}

		tempConfigPath, err = GenerateTempNodeConfig(env, overrides)
		if err != nil {
			return nil, fmt.Errorf("生成临时配置失败: %w", err)
		}
		configPath = tempConfigPath
	}

	// 3. 构建命令参数
	//
	// 新的节点入口为 weisyn-node，必须显式指定 --chain。
	// 这里使用 --chain public，连接公共测试网。
	args := []string{"--chain", "public"}
	if configPath != "" {
		args = append(args, "--config", configPath)
	}
	if opts.KeepData {
		args = append(args, "--keep-data")
	}
	if opts.Daemon {
		args = append(args, "--daemon")
	}

	// 4. 创建命令
	cmd := exec.CommandContext(ctx, nodeBinary, args...)

	// 设置环境变量
	cmd.Env = os.Environ()
	// ✅ CLI/TUI 模式：强制关闭节点控制台日志（避免刷屏影响交互界面）
	// 说明：日志模块会读取 WES_CLI_MODE=true 并将 ToConsole 置为 false。
	if !opts.LogToConsole {
		cmd.Env = append(cmd.Env, "WES_CLI_MODE=true")
	}

	// 5. 准备日志文件
	var logFile *os.File
	if !opts.LogToConsole {
		// 使用环境隔离的日志目录（CLI 进程日志，不是节点业务日志）
		// 注意：节点应用日志仍由节点配置（log.*）决定；这里仅保存子进程 stdout/stderr，便于排障。
		env := normalizeEnv(opts.Env)
		logDir := fmt.Sprintf("./data/%s/cli-managed/logs", env)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			if tempConfigPath != "" {
				_ = CleanupTempConfig(tempConfigPath)
			}
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}
		logPath := filepath.Join(logDir, fmt.Sprintf("wes-cli-node-%s-%d.log", env, os.Getpid()))
		logFile, err = os.Create(logPath)
		if err != nil {
			if tempConfigPath != "" {
				_ = CleanupTempConfig(tempConfigPath)
			}
			return nil, fmt.Errorf("创建日志文件失败: %w", err)
		}

		// 重定向输出到日志文件
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else {
		// 开发模式：输出到控制台
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		go streamLogs(stdout, "[节点STDOUT]")
		go streamLogs(stderr, "[节点STDERR]")
	}

	// 6. 启动进程
	if err := cmd.Start(); err != nil {
		if tempConfigPath != "" {
			_ = CleanupTempConfig(tempConfigPath)
		}
		if logFile != nil {
			_ = logFile.Close()
		}
		return nil, fmt.Errorf("启动节点进程失败: %w", err)
	}

	// 7. 创建进程句柄
	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = "http://localhost:28680"
	}

	np := &NodeProcess{
		cmd:            cmd,
		endpoint:       endpoint,
		tempConfigPath: tempConfigPath,
		logFile:        logFile,
		done:           make(chan struct{}),
	}

	// 8. 监控进程退出
	go func() {
		err := cmd.Wait()
		np.mu.Lock()
		np.err = err
		np.mu.Unlock()
		close(np.done)
	}()

	return np, nil
}

// Wait 等待节点就绪
func (np *NodeProcess) Wait(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return transport.WaitForNodeReady(ctx, np.endpoint, timeout)
}

// Stop 停止节点
func (np *NodeProcess) Stop() error {
	if np.cmd == nil || np.cmd.Process == nil {
		return nil
	}

	// 发送 SIGTERM（优雅停机）
	if err := np.cmd.Process.Signal(os.Interrupt); err != nil {
		// 如果 SIGTERM 失败，强制 Kill
		_ = np.cmd.Process.Kill()
	}

	// 等待进程退出（最多 10 秒）
	timeout := time.After(10 * time.Second)
	select {
	case <-np.done:
		// 进程已退出
	case <-timeout:
		// 超时，强制 Kill
		_ = np.cmd.Process.Kill()
		<-np.done
	}

	// 清理资源
	if np.tempConfigPath != "" {
		CleanupTempConfig(np.tempConfigPath)
	}
	if np.logFile != nil {
		np.logFile.Close()
	}

	return nil
}

// GetEndpoint 获取节点 API 端点
func (np *NodeProcess) GetEndpoint() string {
	return np.endpoint
}

// GetError 获取进程错误（如果已退出）
func (np *NodeProcess) GetError() error {
	np.mu.Lock()
	defer np.mu.Unlock()
	return np.err
}

// IsRunning 检查进程是否还在运行
func (np *NodeProcess) IsRunning() bool {
	select {
	case <-np.done:
		return false
	default:
		return true
	}
}

// findNodeBinary 查找节点二进制文件
func findNodeBinary() (string, error) {
	// 查找优先级：
	// 1. 与 weisyn（启动器）同目录的 weisyn-node（发布/分发推荐：两个二进制放在同一目录）
	// 2. 当前目录的 bin/weisyn-node（源码仓库内推荐位置）
	// 3. 相对路径 ./weisyn-node
	// 4. PATH 环境变量中的 weisyn-node
	//
	// 注意：不再查找 "weisyn"，因为 cmd/weisyn 是可视化启动器，不是节点程序

	var candidates []string

	// 1) 先尝试从启动器可执行文件所在目录查找（解决“用户只有二进制、没有源码”场景）
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "weisyn-node"),
			filepath.Join(exeDir, "bin", "weisyn-node"),
		)
	}

	// 2) 再尝试工作目录相对路径（源码仓库/开发习惯）
	candidates = append(candidates,
		"./bin/weisyn-node",
		"./weisyn-node",
		"weisyn-node",
	)

	for _, candidate := range candidates {
		// 对于相对路径，先检查文件是否存在
		if strings.HasPrefix(candidate, "./") {
			if _, err := os.Stat(candidate); err == nil {
				// 文件存在，返回绝对路径
				absPath, err := filepath.Abs(candidate)
				if err == nil {
					return absPath, nil
				}
			}
		} else {
			// 对于 PATH 中的命令，使用 LookPath
			if path, err := exec.LookPath(candidate); err == nil {
				return path, nil
			}
		}
	}

	// 兜底：源码仓库开发便捷性。
	//
	// ✅ 但发布/日常使用时，weisyn 不应在运行时隐式“编译 node”：
	// - 用户期望编译产物可直接运行；
	// - 自动编译依赖 Go 环境与源码树，且会让行为不可预测。
	//
	// 因此：仅在以下场景允许自动编译：
	// - 显式设置环境变量 WES_AUTO_BUILD_NODE=true
	// - 或者当前 weisyn 是通过 `go run` 运行（可执行文件位于临时 go-build 目录）
	if shouldAutoBuildNodeBinary() {
		if buildErr := tryAutoBuildNodeBinary(); buildErr == nil {
			// 编译成功，重新探测 bin/weisyn-node
			if _, statErr := os.Stat("./bin/weisyn-node"); statErr == nil {
				if absPath, absErr := filepath.Abs("./bin/weisyn-node"); absErr == nil {
					return absPath, nil
				}
			}
		} else {
			// 编译失败，返回编译错误（比通用的"未找到"更有用）
			return "", fmt.Errorf(
				"自动编译 weisyn-node 失败: %w\n"+
					"💡 请手动编译节点二进制：\n"+
					"   make build-node\n"+
					"   或: go build -o bin/weisyn-node ./cmd/node\n"+
					"💡 若你是在“仅有二进制”的环境（无源码），请确保将 weisyn-node 与 weisyn 放在同一目录：\n"+
					"   weisyn\n"+
					"   weisyn-node\n"+
					"⚠️  如果是 onnx 依赖问题，请尝试：\n"+
					"   make build-node-no-onnx\n"+
					"   或: go build -tags noonnx -o bin/weisyn-node ./cmd/node",
				buildErr,
			)
		}
	}

	return "", fmt.Errorf(
		"未找到 weisyn 节点程序\n" +
			"💡 请先准备节点二进制（weisyn-node）：\n" +
			"   make build-node\n" +
			"   或: go build -o bin/weisyn-node ./cmd/node\n" +
			"💡 若你是通过压缩包/发布物拿到 weisyn（无源码），请同时下载/携带对应平台的 weisyn-node，并放在同一目录。\n" +
			"💡 若你希望在源码仓库里启用“自动编译 node”，可设置：WES_AUTO_BUILD_NODE=true\n" +
			"✅ 编译完成后，可通过可视化启动器或直接运行：\n" +
			"   bin/weisyn-node --chain public",
	)
}

// shouldAutoBuildNodeBinary 判断是否允许在运行时自动编译 weisyn-node。
// 默认关闭，仅对开发者的 `go run` 场景提供便利。
func shouldAutoBuildNodeBinary() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("WES_AUTO_BUILD_NODE")))
	if v == "1" || v == "true" || v == "yes" {
		return true
	}

	// go run 场景：可执行文件通常位于临时目录的 go-build 下
	if exe, err := os.Executable(); err == nil {
		exeLower := strings.ToLower(exe)
		if strings.Contains(exeLower, "go-build") {
			return true
		}
		// 额外兜底：若 exe 位于系统临时目录中，也视为开发态
		tmp := strings.ToLower(os.TempDir())
		if tmp != "" && strings.HasPrefix(exeLower, tmp) {
			return true
		}
	}

	return false
}

// tryAutoBuildNodeBinary 尝试在源码树内自动编译节点二进制。
// 成功时返回 nil；失败时返回错误。
func tryAutoBuildNodeBinary() error {
	// 仅在源码树存在时尝试（避免在"只有二进制"的环境里误触发）
	if _, err := os.Stat("./cmd/node"); err != nil {
		return err
	}

	// 直接使用 go build 编译节点
	// 说明：ONNX 引擎在 cgo 不可用时会自动 fallback 到 stub 实现，不需要预下载库文件
	fmt.Println("🔧 检测到源码树，自动编译节点（go build）...")
	if err := os.MkdirAll("./bin", 0755); err != nil {
		return err
	}

	cmd := exec.Command("go", "build", "-o", "bin/weisyn-node", "./cmd/node")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// normalizeEnv 将历史写法映射为当前约定（dev/test/prod），用于本地临时节点。
func normalizeEnv(env string) string {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "", "dev", "development":
		return "dev"
	case "test", "testing":
		return "test"
	case "prod", "production":
		return "prod"
	default:
		// 未知值时回退到 dev，避免启动失败；仅用于本地开发链
		return "dev"
	}
}

// streamLogs 流式输出日志（开发模式）
func streamLogs(reader io.Reader, prefix string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			fmt.Printf("%s %s\n", prefix, line)
		}
	}
}
