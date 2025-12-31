# WES合规工具脚本

🛡️ **WES合规系统辅助工具集 (WES Compliance System Utilities)**

本目录包含WES系统合规功能的辅助脚本和工具，用于数据库管理、配置维护和故障排除。

## 📁 文件说明

### `download_dbip.sh` - DB-IP数据库下载工具

**🌍 用途：** 预先下载DB-IP免费地理位置数据库，避免应用启动时的网络依赖。

**✨ 功能特性：**
- ✅ 自动下载最新DB-IP免费数据库
- ✅ 支持断点续传和网络重试
- ✅ 自动解压缩和完整性验证
- ✅ 智能跳过已存在的文件
- ✅ 详细的进度显示和错误处理

**📖 使用方法：**

```bash
# 基本使用（推荐）
./scripts/compliance/download_dbip.sh

# 强制重新下载
./scripts/compliance/download_dbip.sh --force

# 详细输出模式
./scripts/compliance/download_dbip.sh --verbose

# 查看帮助
./scripts/compliance/download_dbip.sh --help
```

**📊 输出示例：**

```
🌍 WES DB-IP数据库下载工具
================================
[DB-IP下载] 开始下载DB-IP数据库...
[DB-IP下载] 下载地址: https://download.db-ip.com/free/dbip-country-lite-2025-09.mmdb.gz
[DB-IP下载] 目标文件: ./data/compliance/dbip-country-lite.mmdb
[DB-IP下载] ✅ 压缩文件下载完成 (2.1MB)
[DB-IP下载] 解压缩数据库文件...
[DB-IP下载] ✅ 解压缩完成 (5.8MB)
[DB-IP下载] 验证文件完整性...
[DB-IP下载] ✅ 文件完整性验证通过
[DB-IP下载] 🎉 DB-IP数据库下载完成！

📍 文件路径: ./data/compliance/dbip-country-lite.mmdb
📊 文件大小: 5.8MB
🏷️  Attribution: IP Geolocation by DB-IP
📄 许可协议: Creative Commons Attribution 4.0

[DB-IP下载] 现在可以启动WES节点，GeoIP服务将使用本地数据库文件
```

## 🔧 解决启动问题

如果您遇到WES节点启动时由于DB-IP下载失败而导致的启动失败，请按以下步骤操作：

### 方案一：预下载数据库（推荐）

```bash
# 1. 手动预下载DB-IP数据库
./scripts/compliance/download_dbip.sh

# 2. 正常启动节点
go run cmd/node/main.go -config=configs/development/single/config.json
```

### 方案二：禁用合规功能（开发环境）

在配置文件中添加合规配置：

```json
{
  "compliance": {
    "enabled": false,
    "geoip": {
      "auto_update": false
    }
  }
}
```

### 方案三：离线模式

如果网络环境不稳定，可以：

1. 在网络良好的环境下预下载数据库
2. 将 `./data/compliance/dbip-country-lite.mmdb` 文件复制到目标环境
3. 在目标环境启动节点（会自动使用本地文件）

## 🚨 故障排除

### 常见问题

**Q: 下载脚本执行失败？**
A: 检查网络连接，确保可以访问 `https://download.db-ip.com`

**Q: 权限错误？**
A: 确保脚本有执行权限：`chmod +x scripts/compliance/download_dbip.sh`

**Q: 节点仍然启动失败？**
A: 检查 `./data/compliance/dbip-country-lite.mmdb` 文件是否存在且非空

**Q: 想要更新数据库？**
A: 使用 `--force` 选项重新下载：`./scripts/compliance/download_dbip.sh --force`

### 系统要求

- **操作系统：** Linux, macOS, 或其他Unix-like系统
- **必需命令：** `curl`, `gunzip`, `bc`
- **网络要求：** 能够访问DB-IP下载服务器
- **存储空间：** 至少10MB可用空间

## 📄 许可证信息

**DB-IP数据：**
- 来源：[DB-IP](https://db-ip.com/)
- 许可证：Creative Commons Attribution 4.0 International License
- Attribution: "IP Geolocation by DB-IP"

**脚本代码：**
- 遵循WES项目许可证

## 🔗 相关文档

- [WES合规系统文档](../../_docs/implementation/)
- [配置指南](../../configs/README.md)
- [故障排除指南](../../_docs/guides/)
