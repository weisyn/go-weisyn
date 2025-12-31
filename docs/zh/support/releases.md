# 版本发布

---

## 概述

本文档记录 WES 的版本发布历史和计划。

---

## 版本矩阵

| 版本 | 类型 | 发布日期 | 支持状态 | EOL 日期 |
|------|------|----------|----------|----------|
| *待发布* | - | - | - | - |

---

## 发布渠道

### 稳定版

- GitHub Releases
- Docker Hub
- 包管理器（未来计划）

### 预览版

- GitHub Releases（Pre-release）
- Docker Hub（dev 标签）

---

## 发布频率

| 类型 | 频率 |
|------|------|
| 主版本 | 每年 1-2 次 |
| 次版本 | 每月 1 次 |
| 修订版 | 按需 |
| 安全补丁 | 即时 |

---

## 发布流程

### 1. 准备阶段

- 代码冻结
- 功能完成
- 测试通过

### 2. 发布候选

- 创建 RC 版本
- 社区测试
- 收集反馈

### 3. 正式发布

- 创建 Release
- 发布公告
- 更新文档

### 4. 发布后

- 监控问题
- 快速响应
- 收集反馈

---

## 下载

### 二进制文件

访问 [GitHub Releases](https://github.com/weisyn/weisyn/releases) 下载。

### Docker 镜像

```bash
docker pull weisyn/wes-node:latest
docker pull weisyn/wes-node:<version>
```

### 从源码构建

```bash
git clone https://github.com/weisyn/weisyn.git
cd weisyn
make build
```

---

## 变更日志

每个版本的详细变更日志请参见：
- [GitHub Releases](https://github.com/weisyn/weisyn/releases)
- `CHANGELOG.md` 文件

---

## 订阅更新

### 发布通知

- Watch GitHub 仓库
- 订阅邮件列表
- 关注官方博客

---

## 相关文档

- [兼容性策略](./compatibility.md) - API 兼容性
- [支持策略](./support-policy.md) - 版本支持
- [安装指南](../getting-started/installation.md) - 安装说明

