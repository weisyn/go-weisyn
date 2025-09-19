# BadgerDB存储路径修复记录

## 问题描述

之前发现badger数据存储在错误的路径 `internal/core/infrastructure/storage/badger/data/` 中，而不是配置文件指定的正确路径。

## 根本原因

1. **配置不一致**：
   - 配置文件设置：`./data/development/single` 
   - 代码默认设置：硬编码 `./data/badger`

2. **相对路径问题**：
   - 当程序从子目录运行时，相对路径在当前工作目录创建了目录
   - 缺乏统一的路径解析机制

3. **未使用项目提供的路径工具**：
   - 项目已有 `pkg/utils/path.go` 路径处理工具
   - badger配置没有使用这些工具

## 修复方案

### 1. 更新BadgerDB配置 (`internal/config/storage/badger/`)

**config.go 修复**：
- 添加路径工具导入
- 修改 `applyUserConfig` 使用配置的 `data_path` + `badger` 子目录
- 使用 `utils.ResolveDataPath()` 确保路径解析正确

**defaults.go 修复**：  
- 将硬编码常量改为函数 `getDefaultPath()`
- 使用 `utils.ResolveDataPath()` 处理默认路径

### 2. 更新BadgerDB存储实现 (`internal/core/infrastructure/storage/badger/store.go`)

- 添加路径工具导入
- 在备用路径逻辑中也使用 `utils.ResolveDataPath()`

### 3. 清理错误数据

- 删除 `internal/core/infrastructure/storage/badger/data/` 目录

## 预期效果

修复后，badger数据将存储在正确的路径：
- 开发环境单节点：`./data/development/single/badger/`
- 其他环境：根据配置文件的 `storage.data_path` + `/badger`
- 默认情况：`./data/badger/`

所有路径都会被解析为绝对路径，避免工作目录变化导致的问题。

## 验证方法

1. 启动weisyn节点
2. 检查日志中显示的badger存储路径
3. 确认数据文件创建在正确位置
4. 验证API可以正常访问区块链数据

---
修复完成时间：2025-09-18
