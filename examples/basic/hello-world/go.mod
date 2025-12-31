module github.com/weisyn/v1/examples/hello-world

go 1.24.0

toolchain go1.24.7

// 智能合约独立模块
// 使用 Go 1.24 以兼容 TinyGo 0.39.0
//
// 注意：此模块独立于主项目，不共享依赖
// 这样可以：
// 1. 使用 TinyGo 支持的 Go 版本
// 2. 避免引入不必要的依赖
// 3. 保持合约代码的简洁和独立性

require (
	github.com/tetratelabs/wazero v1.9.0
	github.com/weisyn/contract-sdk-go v0.0.0
)

// 使用本地的 SDK 源码（相对路径，仅限仓库内部开发）
replace github.com/weisyn/contract-sdk-go => ../../../_sdks/contract-sdk-go
