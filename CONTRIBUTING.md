# 贡献指南

感谢您对 WES 区块链核心的关注！

## 如何贡献

### 报告 Bug

请使用 [Issue 模板](.github/ISSUE_TEMPLATE/bug_report.md) 报告 bug。

### 提出新功能

请使用 [Feature Request 模板](.github/ISSUE_TEMPLATE/feature_request.md) 提出新功能建议。

### 提交代码

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 代码规范

- 遵循 Go 官方代码规范
- 运行 `go fmt` 格式化代码
- 运行 `go vet` 检查代码
- 添加必要的单元测试和集成测试
- 遵循项目架构边界（参考 `docs/components/` 文档）

## 测试要求

- 新功能需要添加相应的测试用例
- 所有测试必须通过
- 集成测试需要完整的 WES 节点环境
- 性能关键代码需要性能测试

详见：[测试文档](test/README.md)

## 提交信息规范

提交信息应清晰描述变更内容：

```
<type>(<scope>): <subject>

<body>

<footer>
```

**类型（type）**：
- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档更新
- `style`: 代码格式调整
- `refactor`: 代码重构
- `test`: 测试相关
- `chore`: 构建/工具相关
- `perf`: 性能优化

**范围（scope）**（可选）：
- `core`: 核心功能
- `api`: API 相关
- `cli`: 命令行工具
- `contract`: 智能合约
- `network`: 网络相关
- `consensus`: 共识机制
- `ispc`: ISPC 相关
- `docs`: 文档

**示例**：
```
feat(ispc): add ONNX model inference support

Add ONNX runtime integration for AI model inference on-chain.
Support CU calculation and ZK proof generation.

Closes #123
```

## 架构原则

- **模块化**：保持模块边界清晰
- **可测试性**：代码应易于测试
- **文档化**：重要功能需要文档说明
- **向后兼容**：重大变更需要考虑向后兼容性

详见：[架构文档](docs/system/architecture/)

## 相关资源

- [WES 文档中心](docs/)
- [架构文档](docs/system/architecture/)
- [组件文档](docs/components/)
- [开发指南](docs/tutorials/)

---



