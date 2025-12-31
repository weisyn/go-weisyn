package launcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/weisyn/v1/configs"
)

// ConfigOverrides 配置覆盖项
type ConfigOverrides struct {
	HTTPPort   int    // API HTTP 端口
	GRPCPort   int    // gRPC 端口
	DataDir    string // 数据目录
	LogPath    string // 日志路径
	KeepData   bool   // 是否保留历史数据（仅影响清理行为）
}

// GenerateTempNodeConfig 基于内嵌链模板生成临时节点配置文件。
//
// 设计约束：
//   - CLIENT 仅作为 CLI 的可选“可视化启动壳”，不参与正式部署配置管理
//   - 为简化实现，这里统一使用 **内嵌公链测试网配置（test-public-demo）+ 本地环境(dev/test/prod)** 的约定
//   - 生成的配置仅用于本机临时节点；节点数据目录/日志目录默认由节点自身策略决定（不强制覆盖 data_root）
//
// 参数：
//   - env: 运行环境（dev/test/prod），空值时默认 dev
//   - overrides: 节点级覆盖项（端口、目录等）
//
// 返回：
//   - 临时配置文件路径
//   - 错误信息（如有）
func GenerateTempNodeConfig(env string, overrides ConfigOverrides) (string, error) {
	// 0. 规范化环境值（dev/test/prod），默认 dev
	env = strings.TrimSpace(strings.ToLower(env))
	if env == "" {
		env = "dev"
	}
	if env != "dev" && env != "test" && env != "prod" {
		return "", fmt.Errorf("无效的环境值 %q，期望 dev/test/prod", env)
	}

	// 1. 获取基础配置：
	// 使用公共测试网配置（内嵌的 test-public-demo），对应 --chain public 场景。
	baseConfig := configs.GetPublicChainConfig()

	// 2. 解析为 map 以便修改
	var cfgMap map[string]interface{}
	if err := json.Unmarshal(baseConfig, &cfgMap); err != nil {
		return "", fmt.Errorf("解析基础配置失败: %w", err)
	}

	// 3. 应用覆盖项

	// 3.1 覆盖 environment 字段（dev/test/prod）
	cfgMap["environment"] = env

	// 3.2 API 端口覆盖
	if overrides.HTTPPort > 0 {
		ensureMapPath(cfgMap, "api", "http_port")
		cfgMap["api"].(map[string]interface{})["http_port"] = overrides.HTTPPort
	}
	if overrides.GRPCPort > 0 {
		ensureMapPath(cfgMap, "api", "grpc_port")
		cfgMap["api"].(map[string]interface{})["grpc_port"] = overrides.GRPCPort
	}

	// 3.3 数据目录与日志路径覆盖
	if overrides.DataDir != "" {
		ensureMapPath(cfgMap, "storage", "data_root")
		cfgMap["storage"].(map[string]interface{})["data_root"] = overrides.DataDir
	}
	if overrides.LogPath != "" {
		ensureMapPath(cfgMap, "log", "file_path")
		cfgMap["log"].(map[string]interface{})["file_path"] = overrides.LogPath
	}

	// 4. 序列化修改后的配置
	modifiedConfig, err := json.MarshalIndent(cfgMap, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化配置失败: %w", err)
	}

	// 5. 写入临时文件
	tempDir := "./config-temp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}

	tempFile, err := os.CreateTemp(tempDir, fmt.Sprintf("wes-cli-managed-%s-*.json", env))
	if err != nil {
		return "", fmt.Errorf("创建临时配置文件失败: %w", err)
	}
	defer func() {
		if err := tempFile.Close(); err != nil {
			// 记录错误但继续执行
			_ = err
		}
	}()

	if _, err := tempFile.Write(modifiedConfig); err != nil {
		_ = os.Remove(tempFile.Name())
		return "", fmt.Errorf("写入临时配置文件失败: %w", err)
	}

	return tempFile.Name(), nil
}

// CleanupTempConfig 清理临时配置文件
func CleanupTempConfig(path string) error {
	if path == "" {
		return nil
	}
	return os.Remove(path)
}

// CleanupAllTempConfigs 清理所有 CLI 托管的临时配置文件
func CleanupAllTempConfigs() error {
	pattern := "./config-temp/wes-cli-managed-*.json"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, match := range matches {
		if err := os.Remove(match); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("清理 %s 失败: %w", match, err)
		}
	}
	return nil
}

// ensureMapPath 确保嵌套 map 路径存在
func ensureMapPath(m map[string]interface{}, keys ...string) {
	current := m
	for i := 0; i < len(keys)-1; i++ {
		key := keys[i]
		if _, exists := current[key]; !exists {
			current[key] = make(map[string]interface{})
		}
		var ok bool
		current, ok = current[key].(map[string]interface{})
		if !ok {
			// 类型不匹配，无法继续
			return
		}
	}
}

