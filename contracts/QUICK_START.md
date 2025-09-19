# 🚀 Contracts快速上手指南

**⏰ 预计时间**: 30分钟完成第一个合约项目

## 🎯 目标

通过这个快速指南，你将：
- ✅ **5分钟**: 创建你的第一个智能合约项目
- ✅ **10分钟**: 理解项目结构和关键文件
- ✅ **10分钟**: 编译、测试、部署合约
- ✅ **5分钟**: 了解下一步学习方向

## 📋 前置条件

在开始之前，确保你已经：
- ✅ 完成了 `examples/basic/hello-world` 学习
- ✅ 安装了TinyGo编译器 (`brew install tinygo`)
- ✅ 理解了智能合约的基本概念

## 🚀 第一步：创建项目 (5分钟)

### 使用交互式工具创建

```bash
# 进入contracts目录
cd contracts/

# 启动项目创建助手
./tools/beginner/easy-scaffold.sh
```

按照提示选择：
1. **合约类型**: 选择 `1) 💰 代币合约`（推荐初学者）
2. **合约名称**: 输入 `MyFirstToken`
3. **作者姓名**: 输入你的名字
4. **其他选项**: 使用默认值即可

### 项目创建完成

工具会自动生成完整的项目结构：
```
MyFirstToken/
├── 📄 README.md          # 项目说明
├── 📝 src/main.go        # 合约源代码
├── ⚙️  project.json       # 项目配置
├── 🔨 build.sh           # 编译脚本
├── 🧪 test.sh            # 测试脚本
└── 🚀 deploy.sh          # 部署脚本
```

## 📖 第二步：理解项目结构 (10分钟)

### 进入项目目录
```bash
cd MyFirstToken/
```

### 查看合约源代码
```bash
# 查看主要的合约文件
cat src/main.go
```

**关键理解点**：
- 📍 **导入语句**: `import "github.com/weisyn/v1/contracts/sdk/go/framework"`
- 🎯 **主要功能**: `Transfer()`, `GetBalance()`, `GetContractInfo()`
- 🔧 **UTXO机制**: WES独特的资产管理方式
- 📢 **事件系统**: 通过`EmitEvent`记录重要操作

### 查看项目配置
```bash
# 查看项目基本信息
cat project.json
```

这个文件记录了项目的元数据，包括名称、类型、作者等信息。

### 阅读README文档
```bash
# 查看详细的功能说明
cat README.md
```

README包含了：
- 合约功能的详细解释
- 使用示例和测试方法
- 定制化指导
- 进阶学习建议

## 🔨 第三步：编译和测试 (10分钟)

### 编译合约
```bash
# 使用项目自带的编译脚本
./build.sh

# 或使用简化编译工具
../tools/beginner/simple-build.sh
```

**成功标志**：
```
✅ 编译成功！🎉
📁 输出文件: build/main.wasm
📏 文件大小: 15.2K
```

### 测试合约功能
```bash
# 运行基础测试
./test.sh
```

测试将验证：
- 合约编译是否成功
- 基本函数是否正常工作
- 返回值是否正确

### 部署到测试网
```bash
# 部署到测试网络（安全，使用测试代币）
./deploy.sh testnet

# 或使用简化部署工具
../tools/beginner/quick-deploy.sh testnet
```

**部署成功后**会显示：
```
🎉 部署成功！
📝 交易哈希: 0x123...
📍 合约地址: 0x456...
💰 部署成本: 1000 WES
```

## 🎓 第四步：了解下一步 (5分钟)

### 🎯 如果你想深入学习代币开发
1. **阅读详细文档**: `README.md`中的定制化指南
2. **尝试修改功能**: 
   - 改变代币名称和符号
   - 调整初始发行量
   - 添加转账手续费
3. **学习高级特性**: 查看`templates/standard/token/`

### 🖼️ 如果你想学习NFT开发
```bash
# 创建NFT项目
cd ../
./tools/beginner/easy-scaffold.sh
# 选择 "2) 🖼️ NFT合约"
```

### 🛠️ 如果你想开发自定义合约
```bash
# 创建自定义项目
cd ../
./tools/beginner/easy-scaffold.sh
# 选择 "5) 💡 自定义合约"
```

### 📚 深入学习资源
- **完整指南**: [BEGINNER_GUIDE.md](BEGINNER_GUIDE.md)
- **核心概念**: [CONCEPTS.md](CONCEPTS.md)
- **帮助系统**: `./tools/beginner/help.sh`
- **社区支持**: 加入WES开发者社区

## 🎊 恭喜完成！

你已经成功：
- ✅ 创建了第一个智能合约项目
- ✅ 理解了项目结构和核心概念
- ✅ 完成了编译、测试、部署全流程
- ✅ 知道了继续学习的方向

## 💡 实用技巧

### 🔧 开发工作流
```bash
# 典型的开发循环
./build.sh                    # 编译
./test.sh                     # 测试
./deploy.sh testnet           # 部署测试
# 修改代码...
./build.sh                    # 重新编译
```

### 🆘 遇到问题？
1. **查看帮助**: `../tools/beginner/help.sh`
2. **详细编译**: `../tools/beginner/simple-build.sh --verbose`
3. **模拟部署**: `../tools/beginner/quick-deploy.sh --dry-run`
4. **查阅文档**: 各个README.md文件
5. **社区求助**: 在GitHub Issues提问

### 🎯 最佳实践
- **从简单开始**: 先掌握基础功能再添加复杂特性
- **频繁测试**: 每次修改后都要测试
- **使用testnet**: 在测试网络上验证功能
- **备份代码**: 使用git等版本控制工具
- **文档记录**: 为你的修改编写清晰的注释

## 🌟 成就解锁

🏆 **智能合约新手** - 完成第一个合约项目
🚀 **WES开发者** - 掌握WES开发流程
🎯 **准备就绪** - 可以开始真实项目开发

**下一个目标**: 开发一个解决实际问题的智能合约！

---

**记住**: 每个专家都曾是初学者。继续学习，持续实践，你将成为优秀的区块链开发者！ 💪
