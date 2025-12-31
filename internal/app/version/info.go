// Package version provides version information for the application.
package version

import (
	"fmt"
	"runtime"
	"time"
)

// 构建时注入的变量，通过ldflags设置
var (
	// 语义化版本信息
	Version = "v0.0.1" // 主版本号，如v1.2.3

	// 构建信息
	BuildTime = "unknown"     // 构建时间戳（RFC3339格式）
	BuildUser = "unknown"     // 构建用户
	BuildHost = "unknown"     // 构建主机
	BuildEnv  = "development" // 构建环境：development, testing, production

	// Go构建信息
	GoVersion = runtime.Version() // Go版本
	GoArch    = runtime.GOARCH    // 目标架构
	GoOS      = runtime.GOOS      // 目标操作系统
)

// BuildInfo 完整构建信息结构
type BuildInfo struct {
	// 版本信息
	Version string `json:"version"`

	// 构建信息
	BuildTime string `json:"build_time"`
	BuildUser string `json:"build_user"`
	BuildHost string `json:"build_host"`
	BuildEnv  string `json:"build_env"`

	// 运行时信息
	GoVersion string `json:"go_version"`
	GoArch    string `json:"go_arch"`
	GoOS      string `json:"go_os"`
}

// GetDisplayVersion 返回用于UI/状态栏显示的环境化版本
// 规则：如果提供了配置，可根据链ID或环境拼接；此处仅返回Version，
// 由上层根据需要追加环境标签，避免跨层依赖。
func GetDisplayVersion(_ interface{}) string { return Version }

// GetVersion 获取版本号
func GetVersion() string {
	return Version
}

// GetBuildInfo 获取完整构建信息
func GetBuildInfo() *BuildInfo {
	return &BuildInfo{
		Version:   Version,
		BuildTime: BuildTime,
		BuildUser: BuildUser,
		BuildHost: BuildHost,
		BuildEnv:  BuildEnv,
		GoVersion: GoVersion,
		GoArch:    GoArch,
		GoOS:      GoOS,
	}
}

// GetFullVersion 获取完整版本信息（用于详细输出）
func GetFullVersion() string {
	buildInfo := GetBuildInfo()

	versionStr := fmt.Sprintf("微迅区块链 %s", buildInfo.Version)

	if buildInfo.BuildTime != "unknown" {
		if parsedTime, err := time.Parse(time.RFC3339, buildInfo.BuildTime); err == nil {
			versionStr += fmt.Sprintf("\n构建时间: %s", parsedTime.Format("2006-01-02 15:04:05 MST"))
		} else {
			versionStr += fmt.Sprintf("\n构建时间: %s", buildInfo.BuildTime)
		}
	}

	versionStr += fmt.Sprintf("\n构建环境: %s", buildInfo.BuildEnv)
	versionStr += fmt.Sprintf("\nGo版本: %s", buildInfo.GoVersion)
	versionStr += fmt.Sprintf("\n平台: %s/%s", buildInfo.GoOS, buildInfo.GoArch)

	return versionStr
}

// IsProductionBuild 判断是否为生产构建
func IsProductionBuild() bool { return BuildEnv == "production" }

// IsTestingBuild 判断是否为测试构建
func IsTestingBuild() bool { return BuildEnv == "testing" }

// IsDevelopmentBuild 判断是否为开发构建
func IsDevelopmentBuild() bool { return BuildEnv == "development" }
