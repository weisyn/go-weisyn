// Package config provides profile management functionality for client configuration.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Profile CLI配置Profile
type Profile struct {
	Name    string `json:"name"`     // Profile名称: mainnet/testnet/local
	ChainID string `json:"chain_id"` // 链ID

	// 节点端点(按优先级排序)
	Endpoints []EndpointConfig `json:"endpoints"`

	// 本地路径
	KeystorePath string `json:"keystore_path"` // Keystore目录
	CachePath    string `json:"cache_path"`    // 缓存目录
	DataPath     string `json:"data_path"`     // 数据目录

	// 网络配置
	Timeout       Duration `json:"timeout"`        // 请求超时
	RetryAttempts int      `json:"retry_attempts"` // 重试次数
	RetryBackoff  Duration `json:"retry_backoff"`  // 退避时间

	// 故障转移
	HealthCheckInterval Duration `json:"health_check_interval"` // 健康检查间隔

	// 交易默认值
	DefaultFeeRate  string `json:"default_fee_rate,omitempty"`  // 默认费率
	DefaultGasLimit uint64 `json:"default_gas_limit,omitempty"` // 默认Gas限制
}

// EndpointConfig 端点配置
type EndpointConfig struct {
	Name     string `json:"name"`     // 端点名称
	Priority int    `json:"priority"` // 优先级(数字越小越优先)

	// 协议端点
	JSONRPC string `json:"jsonrpc,omitempty"` // JSON-RPC地址
	REST    string `json:"rest,omitempty"`    // REST API地址
	WS      string `json:"ws,omitempty"`      // WebSocket地址
	GRPC    string `json:"grpc,omitempty"`    // gRPC地址
}

// Duration 时间duration(支持JSON序列化)
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}

	*d = Duration(dur)
	return nil
}

// ProfileManager Profile管理器
type ProfileManager struct {
	configDir      string
	currentProfile string
	profiles       map[string]*Profile
}

// NewProfileManager 创建Profile管理器
func NewProfileManager(configDir string) (*ProfileManager, error) {
	if configDir == "" {
		// 默认配置目录: ~/.wes
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		configDir = filepath.Join(homeDir, ".wes")
	}

	// 确保配置目录存在
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	pm := &ProfileManager{
		configDir: configDir,
		profiles:  make(map[string]*Profile),
	}

	// 加载所有profiles
	if err := pm.loadProfiles(); err != nil {
		return nil, err
	}

	// 加载当前profile
	if err := pm.loadCurrentProfile(); err != nil {
		// 如果没有当前profile,使用默认
		pm.currentProfile = "local"
	}

	return pm, nil
}

// loadProfiles 加载所有profiles
func (pm *ProfileManager) loadProfiles() error {
	profilesDir := filepath.Join(pm.configDir, "profiles")

	// 如果profiles目录不存在,创建默认profiles
	if _, err := os.Stat(profilesDir); os.IsNotExist(err) {
		if err := os.MkdirAll(profilesDir, 0700); err != nil {
			return fmt.Errorf("create profiles dir: %w", err)
		}

		// 创建默认profiles
		if err := pm.createDefaultProfiles(); err != nil {
			return err
		}
	}

	// 遍历profiles目录
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		return fmt.Errorf("read profiles dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !isJSONFile(entry.Name()) {
			continue
		}

		profilePath := filepath.Join(profilesDir, entry.Name())
		profile, err := pm.loadProfile(profilePath)
		if err != nil {
			// 记录错误但继续
			fmt.Fprintf(os.Stderr, "Warning: failed to load profile %s: %v\n", entry.Name(), err)
			continue
		}

		pm.profiles[profile.Name] = profile
	}

	return nil
}

// loadProfile 加载单个profile
func (pm *ProfileManager) loadProfile(filePath string) (*Profile, error) {
	//nolint:gosec // G304: filePath 来自配置目录，路径安全可控
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read profile: %w", err)
	}

	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("unmarshal profile: %w", err)
	}

	// 填充默认路径
	if profile.KeystorePath == "" {
		profile.KeystorePath = filepath.Join(pm.configDir, "keystores", profile.Name)
	}
	if profile.CachePath == "" {
		profile.CachePath = filepath.Join(pm.configDir, "cache", profile.Name)
	}
	if profile.DataPath == "" {
		profile.DataPath = filepath.Join(pm.configDir, "data", profile.Name)
	}

	// 填充默认网络配置
	if profile.Timeout == 0 {
		profile.Timeout = Duration(30 * time.Second)
	}
	if profile.RetryAttempts == 0 {
		profile.RetryAttempts = 3
	}
	if profile.RetryBackoff == 0 {
		profile.RetryBackoff = Duration(time.Second)
	}
	if profile.HealthCheckInterval == 0 {
		profile.HealthCheckInterval = Duration(30 * time.Second)
	}

	return &profile, nil
}

// loadCurrentProfile 加载当前profile
func (pm *ProfileManager) loadCurrentProfile() error {
	currentFile := filepath.Join(pm.configDir, "current")
	//nolint:gosec // G304: currentFile 来自配置目录，路径安全可控
	data, err := os.ReadFile(currentFile)
	if err != nil {
		return err
	}

	pm.currentProfile = string(data)
	return nil
}

// saveCurrentProfile 保存当前profile
func (pm *ProfileManager) saveCurrentProfile() error {
	currentFile := filepath.Join(pm.configDir, "current")
	return os.WriteFile(currentFile, []byte(pm.currentProfile), 0600)
}

// createDefaultProfiles 创建默认profiles
func (pm *ProfileManager) createDefaultProfiles() error {
	profiles := []*Profile{
		{
			Name:    "local",
			ChainID: "wes-local-1",
			Endpoints: []EndpointConfig{
				{
					Name:     "local-node",
					Priority: 1,
					JSONRPC:  "http://localhost:28680/jsonrpc",
					REST:     "http://localhost:28680/api/v1",
					WS:       "ws://localhost:28681",
					GRPC:     "localhost:28682",
				},
			},
			Timeout:             Duration(30 * time.Second),
			RetryAttempts:       3,
			RetryBackoff:        Duration(time.Second),
			HealthCheckInterval: Duration(30 * time.Second),
		},
		{
			Name:    "testnet",
			ChainID: "wes-testnet-1",
			Endpoints: []EndpointConfig{
				{
					Name:     "testnet-primary",
					Priority: 1,
					JSONRPC:  "https://testnet-rpc.wes.io",
					REST:     "https://testnet-api.wes.io/api/v1",
					WS:       "wss://testnet-ws.wes.io",
				},
				{
					Name:     "testnet-backup",
					Priority: 2,
					JSONRPC:  "https://testnet-rpc2.wes.io",
				},
			},
			Timeout:             Duration(60 * time.Second),
			RetryAttempts:       5,
			RetryBackoff:        Duration(2 * time.Second),
			HealthCheckInterval: Duration(60 * time.Second),
			DefaultFeeRate:      "1000",
			DefaultGasLimit:     21000,
		},
		{
			Name:    "mainnet",
			ChainID: "wes-mainnet-1",
			Endpoints: []EndpointConfig{
				{
					Name:     "mainnet-primary",
					Priority: 1,
					JSONRPC:  "https://mainnet-rpc.wes.io",
					REST:     "https://mainnet-api.wes.io/api/v1",
					WS:       "wss://mainnet-ws.wes.io",
				},
				{
					Name:     "mainnet-backup",
					Priority: 2,
					JSONRPC:  "https://mainnet-rpc2.wes.io",
				},
			},
			Timeout:             Duration(60 * time.Second),
			RetryAttempts:       5,
			RetryBackoff:        Duration(2 * time.Second),
			HealthCheckInterval: Duration(60 * time.Second),
			DefaultFeeRate:      "2000",
			DefaultGasLimit:     21000,
		},
	}

	for _, profile := range profiles {
		if err := pm.SaveProfile(profile); err != nil {
			return err
		}
	}

	// 设置local为当前profile
	pm.currentProfile = "local"
	return pm.saveCurrentProfile()
}

// GetProfile 获取指定profile
func (pm *ProfileManager) GetProfile(name string) (*Profile, error) {
	profile, exists := pm.profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile not found: %s", name)
	}
	return profile, nil
}

// GetCurrentProfile 获取当前profile
func (pm *ProfileManager) GetCurrentProfile() (*Profile, error) {
	return pm.GetProfile(pm.currentProfile)
}

// ListProfiles 列出所有profiles
func (pm *ProfileManager) ListProfiles() []string {
	names := make([]string, 0, len(pm.profiles))
	for name := range pm.profiles {
		names = append(names, name)
	}
	return names
}

// SaveProfile 保存profile
func (pm *ProfileManager) SaveProfile(profile *Profile) error {
	// 在保存前填充本地路径的默认值，保持与 loadProfile 行为一致
	if profile.KeystorePath == "" {
		profile.KeystorePath = filepath.Join(pm.configDir, "keystores", profile.Name)
	}
	if profile.CachePath == "" {
		profile.CachePath = filepath.Join(pm.configDir, "cache", profile.Name)
	}
	if profile.DataPath == "" {
		profile.DataPath = filepath.Join(pm.configDir, "data", profile.Name)
	}

	profilePath := filepath.Join(pm.configDir, "profiles", profile.Name+".json")

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal profile: %w", err)
	}

	if err := os.WriteFile(profilePath, data, 0600); err != nil {
		return fmt.Errorf("write profile: %w", err)
	}

	pm.profiles[profile.Name] = profile
	return nil
}

// SwitchProfile 切换profile
func (pm *ProfileManager) SwitchProfile(name string) error {
	if _, exists := pm.profiles[name]; !exists {
		return fmt.Errorf("profile not found: %s", name)
	}

	pm.currentProfile = name
	return pm.saveCurrentProfile()
}

// DeleteProfile 删除profile
func (pm *ProfileManager) DeleteProfile(name string) error {
	// 不能删除当前profile
	if name == pm.currentProfile {
		return fmt.Errorf("cannot delete current profile")
	}

	profilePath := filepath.Join(pm.configDir, "profiles", name+".json")
	if err := os.Remove(profilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete profile file: %w", err)
	}

	delete(pm.profiles, name)
	return nil
}

// isJSONFile 检查是否是JSON文件
func isJSONFile(name string) bool {
	return filepath.Ext(name) == ".json"
}
