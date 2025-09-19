#!/bin/bash

# ==================== WES智能合约开发帮助中心 ====================
#
# 🎯 工具作用：提供交互式帮助系统，解答初学者常见问题
# 💡 特点：分类问题解答、实用示例、故障排除指南
# 🎨 设计理念：像私人导师一样，随时为你答疑解惑
#
# 📚 使用方法：
#   ./help.sh [主题]
#   主题：
#     getting-started    新手入门指南
#     templates          模板使用帮助
#     tools              工具使用帮助
#     troubleshooting    问题排除指南
#     examples           示例和最佳实践
#     concepts           核心概念解释
#
# ==================== 颜色和样式定义 ====================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m'

# 输出函数
print_header() {
    echo -e "${BLUE}================================${NC}"
    echo -e "${WHITE}$1${NC}"
    echo -e "${BLUE}================================${NC}"
    echo ""
}

print_section() {
    echo -e "${CYAN}📋 $1${NC}"
    echo -e "${CYAN}$(printf '%.0s-' {1..40})${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${PURPLE}💡 $1${NC}"
}

print_tip() {
    echo -e "${CYAN}💡 技巧: $1${NC}"
}

print_example() {
    echo -e "${GREEN}📝 示例: $1${NC}"
}

# ==================== 主菜单 ====================

show_main_menu() {
    clear
    print_header "🤝 WES智能合约开发帮助中心"
    
    echo -e "${WHITE}欢迎来到WES智能合约开发帮助中心！${NC}"
    echo -e "${CYAN}这里是你学习和解决问题的好帮手 😊${NC}"
    echo ""
    
    echo -e "${WHITE}📚 请选择你需要帮助的主题：${NC}"
    echo ""
    echo -e "${CYAN}1) 🚀 新手入门${NC} - 完全零基础？从这里开始！"
    echo -e "${CYAN}2) 📋 模板使用${NC} - 如何使用和定制合约模板"
    echo -e "${CYAN}3) 🛠️  工具使用${NC} - 编译、部署、测试工具指南"
    echo -e "${CYAN}4) 🔧 问题排除${NC} - 遇到错误？快速解决方案"
    echo -e "${CYAN}5) 📚 示例学习${NC} - 实用代码示例和最佳实践"
    echo -e "${CYAN}6) 💡 核心概念${NC} - 深入理解区块链和WES"
    echo -e "${CYAN}7) 🌐 社区资源${NC} - 获取更多帮助和支持"
    echo -e "${CYAN}8) 🔍 搜索帮助${NC} - 搜索特定问题"
    echo ""
    echo -e "${YELLOW}q) 退出帮助系统${NC}"
    echo ""
}

# ==================== 新手入门 ====================

show_getting_started() {
    clear
    print_header "🚀 新手入门指南"
    
    echo -e "${WHITE}欢迎来到WES智能合约开发！让我们从零开始 🌟${NC}"
    echo ""
    
    print_section "第一步：环境准备"
    echo "在开始之前，确保你已经安装了必要的工具："
    echo ""
    echo -e "${GREEN}✅ TinyGo编译器${NC} - 用于编译智能合约"
    echo "   安装方法: brew install tinygo"
    echo ""
    echo -e "${GREEN}✅ Go语言${NC} - TinyGo的依赖"
    echo "   安装方法: https://golang.org/dl/"
    echo ""
    echo -e "${GREEN}✅ WES节点${NC} - 本地开发环境"
    echo "   获取方法: 联系WES团队"
    echo ""
    
    print_section "第二步：理解目录结构"
    echo "contracts/目录是你的开发工作台："
    echo ""
    echo "📁 templates/learning/     # 学习版模板（推荐新手使用）"
    echo "  ├── simple-token/       # 代币合约模板"
    echo "  ├── basic-nft/          # NFT合约模板"
    echo "  └── starter-contract/   # 自定义合约模板"
    echo ""
    echo "🛠️  tools/beginner/        # 初学者友好工具"
    echo "  ├── easy-scaffold.sh    # 项目创建助手"
    echo "  ├── simple-build.sh     # 简化编译工具"
    echo "  ├── quick-deploy.sh     # 快速部署工具"
    echo "  └── help.sh             # 帮助系统（你在用的）"
    echo ""
    
    print_section "第三步：创建第一个项目"
    echo "使用我们的项目创建助手："
    echo ""
    print_example "./easy-scaffold.sh"
    echo ""
    echo "按照提示选择合约类型和输入项目信息，工具会为你生成完整的项目结构。"
    echo ""
    
    print_section "第四步：学习路径"
    echo "推荐的学习顺序："
    echo ""
    echo "1️⃣  先完成 examples/basic/hello-world（如果还没有）"
    echo "2️⃣  使用 templates/learning/simple-token 学习代币开发"
    echo "3️⃣  尝试 templates/learning/basic-nft 了解NFT"
    echo "4️⃣  使用 templates/learning/starter-contract 开发自定义合约"
    echo ""
    
    print_tip "从简单开始，逐步进阶。每个模板都有详细的注释和说明文档！"
    
    echo ""
    read -p "按回车键返回主菜单..."
}

# ==================== 模板使用 ====================

show_templates_help() {
    clear
    print_header "📋 模板使用指南"
    
    echo -e "${WHITE}WES提供了多种模板帮你快速开发合约 🎯${NC}"
    echo ""
    
    print_section "模板分类"
    echo ""
    echo -e "${GREEN}🎓 学习版模板${NC} (templates/learning/)"
    echo "• 详细注释，概念解释"
    echo "• 适合初学者理解和学习"
    echo "• 包含生活化类比和示例"
    echo ""
    echo -e "${BLUE}📋 标准版模板${NC} (templates/standard/)"
    echo "• 生产就绪，最佳实践"
    echo "• 功能完整，性能优化"
    echo "• 适合实际项目开发"
    echo ""
    echo -e "${PURPLE}🏢 企业版模板${NC} (templates/production/)"
    echo "• 企业级功能，安全优化"
    echo "• 高级特性，复杂场景"
    echo "• 适合大型项目"
    echo ""
    
    print_section "如何选择模板"
    echo ""
    echo "根据你的需求选择："
    echo ""
    echo -e "${CYAN}💰 代币相关${NC}"
    echo "• 社区积分 → simple-token"
    echo "• 项目代币 → standard/token"
    echo "• 企业代币 → production/enterprise-token"
    echo ""
    echo -e "${CYAN}🖼️  NFT相关${NC}"
    echo "• 数字收藏 → basic-nft"
    echo "• 艺术作品 → standard/nft"
    echo "• 游戏道具 → production/game-nft"
    echo ""
    echo -e "${CYAN}🎮 游戏/DAO/自定义${NC}"
    echo "• 学习用途 → starter-contract"
    echo "• 复杂功能 → 组合多个模板"
    echo ""
    
    print_section "模板使用步骤"
    echo ""
    echo "1️⃣  选择合适的模板"
    print_example "./easy-scaffold.sh  # 交互式选择"
    echo ""
    echo "2️⃣  定制模板内容"
    echo "• 修改合约名称和符号"
    echo "• 调整初始参数"
    echo "• 添加自定义功能"
    echo ""
    echo "3️⃣  测试和部署"
    print_example "./build.sh && ./test.sh && ./deploy.sh testnet"
    echo ""
    
    print_section "常见定制需求"
    echo ""
    echo -e "${YELLOW}代币模板定制：${NC}"
    echo "• 修改代币名称和符号"
    echo "• 调整发行量和小数位"
    echo "• 添加转账手续费"
    echo "• 实现暂停功能"
    echo ""
    echo -e "${YELLOW}NFT模板定制：${NC}"
    echo "• 设置系列名称和符号"
    echo "• 配置元数据URI"
    echo "• 添加版税机制"
    echo "• 实现白名单铸造"
    echo ""
    
    print_tip "每个模板的README.md都有详细的定制指南，一定要仔细阅读！"
    
    echo ""
    read -p "按回车键返回主菜单..."
}

# ==================== 工具使用 ====================

show_tools_help() {
    clear
    print_header "🛠️ 工具使用指南"
    
    echo -e "${WHITE}WES提供了完整的开发工具链，让开发变得简单 🚀${NC}"
    echo ""
    
    print_section "初学者工具 (tools/beginner/)"
    echo ""
    echo -e "${GREEN}🏗️  easy-scaffold.sh${NC} - 项目创建助手"
    echo "• 交互式项目创建"
    echo "• 5种合约类型选择"
    echo "• 自动代码定制"
    print_example "./easy-scaffold.sh"
    echo ""
    
    echo -e "${GREEN}🔨 simple-build.sh${NC} - 简化编译工具"
    echo "• 友好的编译体验"
    echo "• 详细的错误提示"
    echo "• 跨平台支持"
    print_example "./simple-build.sh --optimize --verbose"
    echo ""
    
    echo -e "${GREEN}🚀 quick-deploy.sh${NC} - 快速部署工具"
    echo "• 多网络支持"
    echo "• 安全检查"
    echo "• 部署状态跟踪"
    print_example "./quick-deploy.sh testnet --verify"
    echo ""
    
    echo -e "${GREEN}🤝 help.sh${NC} - 帮助系统"
    echo "• 交互式帮助"
    echo "• 问题排除指南"
    echo "• 最佳实践分享"
    print_example "./help.sh"
    echo ""
    
    print_section "标准工具 (tools/standard/)"
    echo ""
    echo -e "${BLUE}⚙️  scaffold${NC} - 高级项目脚手架"
    echo -e "${BLUE}🔧 compiler${NC} - 专业编译器"
    echo -e "${BLUE}🛡️  verifier${NC} - 安全验证器"
    echo -e "${BLUE}📦 deployer${NC} - 部署管理器"
    echo ""
    
    print_section "工具使用技巧"
    echo ""
    echo -e "${YELLOW}开发阶段：${NC}"
    echo "• 使用beginner工具快速上手"
    echo "• 频繁编译测试: ./simple-build.sh"
    echo "• 使用testnet部署: ./quick-deploy.sh testnet"
    echo ""
    echo -e "${YELLOW}生产阶段：${NC}"
    echo "• 使用standard工具进行精细控制"
    echo "• 启用优化编译: --optimize"
    echo "• 谨慎部署到mainnet"
    echo ""
    
    print_section "常用命令组合"
    echo ""
    print_example "# 完整开发流程"
    echo "./easy-scaffold.sh          # 创建项目"
    echo "cd MyProject"
    echo "./simple-build.sh           # 编译合约"
    echo "./quick-deploy.sh testnet   # 部署测试"
    echo ""
    print_example "# 生产部署流程"
    echo "./simple-build.sh --optimize"
    echo "./quick-deploy.sh testnet --verify"
    echo "./quick-deploy.sh mainnet   # 确认无误后"
    echo ""
    
    print_tip "善用 --help 选项查看每个工具的详细用法！"
    
    echo ""
    read -p "按回车键返回主菜单..."
}

# ==================== 问题排除 ====================

show_troubleshooting() {
    clear
    print_header "🔧 问题排除指南"
    
    echo -e "${WHITE}遇到问题不要慌，这里有常见问题的解决方案 🩺${NC}"
    echo ""
    
    print_section "编译问题"
    echo ""
    echo -e "${RED}❌ 错误: TinyGo编译器未安装${NC}"
    echo -e "${GREEN}✅ 解决: brew install tinygo${NC}"
    echo ""
    
    echo -e "${RED}❌ 错误: undefined: framework.xxx${NC}"
    echo -e "${GREEN}✅ 解决: 检查import路径${NC}"
    print_example 'import "github.com/weisyn/v1/contracts/sdk/go/framework"'
    echo ""
    
    echo -e "${RED}❌ 错误: syntax error${NC}"
    echo -e "${GREEN}✅ 解决: 检查语法错误${NC}"
    echo "• 缺少分号或括号"
    echo "• 变量名拼写错误"
    echo "• 函数参数不匹配"
    echo ""
    
    echo -e "${RED}❌ 错误: 编译后文件过大${NC}"
    echo -e "${GREEN}✅ 解决: 启用优化编译${NC}"
    print_example "./simple-build.sh --optimize"
    echo ""
    
    print_section "部署问题"
    echo ""
    echo -e "${RED}❌ 错误: 网络连接失败${NC}"
    echo -e "${GREEN}✅ 解决: 检查网络设置${NC}"
    echo "• 确认网络地址正确"
    echo "• 检查防火墙设置"
    echo "• 尝试使用VPN"
    echo ""
    
    echo -e "${RED}❌ 错误: 执行费用不足${NC}"
    echo -e "${GREEN}✅ 解决: 增加执行费用限制${NC}"
    print_example "./quick-deploy.sh testnet --fee-limit 2000000"
    echo ""
    
    echo -e "${RED}❌ 错误: 账户余额不足${NC}"
    echo -e "${GREEN}✅ 解决: 获取测试代币${NC}"
    echo "• 测试网：使用水龙头获取"
    echo "• 主网：购买或转入代币"
    echo ""
    
    print_section "运行时问题"
    echo ""
    echo -e "${RED}❌ 错误: 合约调用失败${NC}"
    echo -e "${GREEN}✅ 解决: 检查参数格式${NC}"
    print_example 'weisyn call <address> Transfer \'{"to":"0x...", "amount":"100"}\''
    echo ""
    
    echo -e "${RED}❌ 错误: 权限不足${NC}"
    echo -e "${GREEN}✅ 解决: 确认调用者权限${NC}"
    echo "• 检查是否为合约owner"
    echo "• 验证访问控制逻辑"
    echo ""
    
    print_section "开发环境问题"
    echo ""
    echo -e "${RED}❌ 错误: 项目结构混乱${NC}"
    echo -e "${GREEN}✅ 解决: 重新创建项目${NC}"
    print_example "./easy-scaffold.sh  # 重新开始"
    echo ""
    
    echo -e "${RED}❌ 错误: 版本不兼容${NC}"
    echo -e "${GREEN}✅ 解决: 更新工具版本${NC}"
    echo "• 更新TinyGo: brew upgrade tinygo"
    echo "• 更新Go: 从官网下载最新版"
    echo ""
    
    print_section "获取更多帮助"
    echo ""
    echo -e "${CYAN}🔍 诊断步骤：${NC}"
    echo "1. 使用 --verbose 选项查看详细信息"
    echo "2. 检查错误日志的具体信息"
    echo "3. 搜索社区是否有类似问题"
    echo "4. 联系WES技术支持"
    echo ""
    
    print_tip "遇到新问题？记录错误信息，它们是解决问题的关键线索！"
    
    echo ""
    read -p "按回车键返回主菜单..."
}

# ==================== 示例学习 ====================

show_examples_help() {
    clear
    print_header "📚 示例学习"
    
    echo -e "${WHITE}通过实用示例快速掌握开发技巧 💡${NC}"
    echo ""
    
    print_section "基础示例"
    echo ""
    echo -e "${GREEN}💰 代币转账示例${NC}"
    echo 'func Transfer() uint32 {'
    echo '    params := framework.GetContractParams()'
    echo '    to := params.ParseJSON("to")'
    echo '    amount := params.ParseJSON("amount")'
    echo '    '
    echo '    // 执行转账逻辑'
    echo '    err := framework.TransferUTXO(from, toAddr, amt, tokenID)'
    echo '    return framework.SUCCESS'
    echo '}'
    echo ""
    
    echo -e "${GREEN}🖼️  NFT铸造示例${NC}"
    echo 'func MintNFT() uint32 {'
    echo '    params := framework.GetContractParams()'
    echo '    to := params.ParseJSON("to")'
    echo '    tokenURI := params.ParseJSON("tokenURI")'
    echo '    '
    echo '    // 创建NFT UTXO'
    echo '    err := framework.CreateUTXO(toAddr, amount, nftTokenID)'
    echo '    return framework.SUCCESS'
    echo '}'
    echo ""
    
    print_section "进阶示例"
    echo ""
    echo -e "${BLUE}🗳️  投票功能示例${NC}"
    echo 'func Vote() uint32 {'
    echo '    proposalID := params.ParseJSON("proposalID")'
    echo '    choice := params.ParseJSON("choice")'
    echo '    voter := framework.GetCaller()'
    echo '    '
    echo '    // 验证投票权限和记录投票'
    echo '    // ...'
    echo '    return framework.SUCCESS'
    echo '}'
    echo ""
    
    echo -e "${BLUE}⏰ 时间锁示例${NC}"
    echo 'func LockAsset() uint32 {'
    echo '    amount := params.ParseJSON("amount")'
    echo '    duration := params.ParseJSON("duration")'
    echo '    unlockTime := framework.GetTimestamp() + duration'
    echo '    '
    echo '    // 创建锁定UTXO'
    echo '    // ...'
    echo '    return framework.SUCCESS'
    echo '}'
    echo ""
    
    print_section "最佳实践"
    echo ""
    echo -e "${YELLOW}✅ 参数验证${NC}"
    echo 'if to == "" || amount == "" {'
    echo '    return framework.ERROR_INVALID_PARAMS'
    echo '}'
    echo ""
    
    echo -e "${YELLOW}✅ 权限检查${NC}"
    echo 'caller := framework.GetCaller()'
    echo 'if !isAuthorized(caller) {'
    echo '    return framework.ERROR_UNAUTHORIZED'
    echo '}'
    echo ""
    
    echo -e "${YELLOW}✅ 事件发出${NC}"
    echo 'event := framework.NewEvent("Transfer")'
    echo 'event.AddAddressField("from", from)'
    echo 'event.AddStringField("to", to)'
    echo 'framework.EmitEvent(event)'
    echo ""
    
    print_section "调试技巧"
    echo ""
    echo -e "${CYAN}🔍 日志输出${NC}"
    echo "• 在关键步骤添加日志"
    echo "• 使用事件记录状态变化"
    echo "• 返回详细的错误信息"
    echo ""
    
    echo -e "${CYAN}🧪 测试方法${NC}"
    echo "• 编写单元测试"
    echo "• 使用testnet验证功能"
    echo "• 模拟各种边界条件"
    echo ""
    
    print_tip "多看examples/目录下的完整示例，它们是最好的学习资料！"
    
    echo ""
    read -p "按回车键返回主菜单..."
}

# ==================== 核心概念 ====================

show_concepts_help() {
    clear
    print_header "💡 核心概念解释"
    
    echo -e "${WHITE}深入理解区块链和WES的核心概念 🧠${NC}"
    echo ""
    
    print_section "智能合约基础"
    echo ""
    echo -e "${GREEN}🤔 什么是智能合约？${NC}"
    echo "智能合约就像自动执行的程序："
    echo "• 部署到区块链上"
    echo "• 按照预定规则执行"
    echo "• 不可篡改，公开透明"
    echo "• 无需中介，自动执行"
    echo ""
    
    echo -e "${GREEN}🎯 智能合约的作用${NC}"
    echo "• 代币发行和管理"
    echo "• NFT创建和交易"
    echo "• 去中心化金融(DeFi)"
    echo "• DAO治理和投票"
    echo "• 游戏和娱乐应用"
    echo ""
    
    print_section "WES特色：UTXO模型"
    echo ""
    echo -e "${BLUE}🧩 什么是UTXO？${NC}"
    echo "UTXO (Unspent Transaction Output) = 未花费交易输出"
    echo ""
    echo "🏦 传统账户模型 vs 🪙 UTXO模型："
    echo "账户模型: Alice余额=100, Bob余额=50"
    echo "UTXO模型: Alice拥有[80,20], Bob拥有[30,20]"
    echo ""
    
    echo -e "${BLUE}✨ UTXO的优势${NC}"
    echo "• 🔒 更安全: 并发处理，防双花"
    echo "• ⚡ 更高效: 并行验证交易"
    echo "• 🔍 更透明: 每个UTXO有明确历史"
    echo "• 💪 更灵活: 支持复杂合约逻辑"
    echo ""
    
    print_section "代币 vs NFT"
    echo ""
    echo -e "${YELLOW}💰 代币 (Fungible Token)${NC}"
    echo "• 可互换: 每个代币都相同"
    echo "• 可分割: 可以有0.5个代币"
    echo "• 用途: 货币、积分、股权"
    echo "• 例子: BTC, ETH, USDT"
    echo ""
    
    echo -e "${YELLOW}🖼️  NFT (Non-Fungible Token)${NC}"
    echo "• 不可互换: 每个都独一无二"
    echo "• 不可分割: 只能整个转移"
    echo "• 用途: 艺术品、收藏品、证书"
    echo "• 例子: 数字艺术、游戏道具"
    echo ""
    
    print_section "区块链基础概念"
    echo ""
    echo -e "${PURPLE}🔗 区块链${NC}"
    echo "• 分布式账本技术"
    echo "• 数据不可篡改"
    echo "• 去中心化网络"
    echo "• 共识机制验证"
    echo ""
    
    echo -e "${PURPLE}⛽ 执行费用费用${NC}"
    echo "• 执行操作的燃料"
    echo "• 防止恶意攻击"
    echo "• 激励矿工打包"
    echo "• 计算资源定价"
    echo ""
    
    echo -e "${PURPLE}📝 交易${NC}"
    echo "• 状态变更请求"
    echo "• 需要签名验证"
    echo "• 打包进入区块"
    echo "• 全网络广播"
    echo ""
    
    print_section "开发相关概念"
    echo ""
    echo -e "${CYAN}🔧 ABI (Application Binary Interface)${NC}"
    echo "• 合约接口定义"
    echo "• 函数调用规范"
    echo "• 参数编码格式"
    echo "• 前端交互必需"
    echo ""
    
    echo -e "${CYAN}📊 事件 (Events)${NC}"
    echo "• 记录重要操作"
    echo "• 便于监听和查询"
    echo "• 减少存储成本"
    echo "• 提供操作历史"
    echo ""
    
    print_tip "理解这些概念是成为优秀区块链开发者的基础！"
    
    echo ""
    read -p "按回车键返回主菜单..."
}

# ==================== 社区资源 ====================

show_community_help() {
    clear
    print_header "🌐 社区资源"
    
    echo -e "${WHITE}加入WES社区，与其他开发者一起成长 🤝${NC}"
    echo ""
    
    print_section "官方资源"
    echo ""
    echo -e "${GREEN}📖 官方文档${NC}"
    echo "• 完整的技术文档"
    echo "• API参考手册"
    echo "• 最佳实践指南"
    echo "• 网址: https://docs.weisyn.io"
    echo ""
    
    echo -e "${GREEN}🌟 GitHub仓库${NC}"
    echo "• 源代码开源"
    echo "• Issue跟踪系统"
    echo "• 贡献指南"
    echo "• 网址: https://github.com/weisyn/weisyn"
    echo ""
    
    echo -e "${GREEN}🎯 官方网站${NC}"
    echo "• 项目介绍和愿景"
    echo "• 最新公告和更新"
    echo "• 团队和路线图"
    echo "• 网址: https://weisyn.io"
    echo ""
    
    print_section "开发者社区"
    echo ""
    echo -e "${BLUE}💬 Discord服务器${NC}"
    echo "• 实时技术讨论"
    echo "• 问题快速解答"
    echo "• 开发者交流"
    echo "• 邀请链接: [联系获取]"
    echo ""
    
    echo -e "${BLUE}📱 Telegram群组${NC}"
    echo "• 中文技术交流"
    echo "• 项目更新通知"
    echo "• 社区活动组织"
    echo "• 群组链接: [联系获取]"
    echo ""
    
    echo -e "${BLUE}🐦 Twitter/X${NC}"
    echo "• 官方动态发布"
    echo "• 技术文章分享"
    echo "• 社区精彩内容"
    echo "• 账号: @WES_official"
    echo ""
    
    print_section "学习资源"
    echo ""
    echo -e "${YELLOW}📚 教程文章${NC}"
    echo "• 从入门到精通系列"
    echo "• 实战项目分析"
    echo "• 常见问题解答"
    echo "• 最佳实践分享"
    echo ""
    
    echo -e "${YELLOW}🎥 视频教程${NC}"
    echo "• 开发环境搭建"
    echo "• 合约开发实战"
    echo "• 工具使用演示"
    echo "• 概念深度讲解"
    echo ""
    
    echo -e "${YELLOW}🎪 在线研讨会${NC}"
    echo "• 定期技术分享"
    echo "• 专家答疑解惑"
    echo "• 新功能介绍"
    echo "• 社区互动交流"
    echo ""
    
    print_section "获得帮助的最佳方式"
    echo ""
    echo -e "${CYAN}🆘 问问题前的准备${NC}"
    echo "1. 详细描述问题现象"
    echo "2. 提供错误信息截图"
    echo "3. 说明你已经尝试的解决方法"
    echo "4. 包含相关的代码片段"
    echo ""
    
    echo -e "${CYAN}💡 提问技巧${NC}"
    echo "• 标题简洁明了"
    echo "• 描述完整准确"
    echo "• 提供最小重现案例"
    echo "• 保持礼貌和耐心"
    echo ""
    
    print_section "贡献社区"
    echo ""
    echo -e "${PURPLE}🎁 如何贡献${NC}"
    echo "• 报告bug和问题"
    echo "• 提交代码改进"
    echo "• 编写文档和教程"
    echo "• 帮助其他开发者"
    echo "• 分享项目和经验"
    echo ""
    
    echo -e "${PURPLE}🏆 贡献奖励${NC}"
    echo "• 社区声誉和认可"
    echo "• 优先技术支持"
    echo "• 参与核心决策"
    echo "• 代币奖励(如适用)"
    echo ""
    
    print_tip "活跃的社区参与是快速学习和解决问题的最佳途径！"
    
    echo ""
    read -p "按回车键返回主菜单..."
}

# ==================== 搜索帮助 ====================

search_help() {
    clear
    print_header "🔍 搜索帮助"
    
    echo -e "${WHITE}输入关键词搜索相关帮助信息 🔎${NC}"
    echo ""
    
    read -p "请输入搜索关键词 (按q返回): " keyword
    
    if [[ "$keyword" == "q" ]]; then
        return
    fi
    
    echo ""
    echo -e "${CYAN}搜索结果: \"$keyword\"${NC}"
    echo -e "${CYAN}$(printf '%.0s-' {1..40})${NC}"
    
    # 简化的搜索逻辑
    case $keyword in
        *编译*|*build*|*compile*)
            echo "📋 相关帮助: 工具使用 → 编译问题"
            echo "• 使用 ./simple-build.sh 进行编译"
            echo "• 添加 --optimize 进行优化编译"
            echo "• 使用 --verbose 查看详细信息"
            ;;
        *部署*|*deploy*)
            echo "📋 相关帮助: 工具使用 → 部署问题"
            echo "• 使用 ./quick-deploy.sh testnet 部署到测试网"
            echo "• 添加 --verify 进行部署验证"
            echo "• 使用 --dry-run 进行模拟部署"
            ;;
        *代币*|*token*)
            echo "📋 相关帮助: 模板使用 → 代币模板"
            echo "• 使用 templates/learning/simple-token"
            echo "• 学习代币的基本概念和实现"
            echo "• 了解UTXO代币管理机制"
            ;;
        *NFT*|*nft*)
            echo "📋 相关帮助: 模板使用 → NFT模板"
            echo "• 使用 templates/learning/basic-nft"
            echo "• 理解NFT与代币的区别"
            echo "• 学习元数据管理"
            ;;
        *错误*|*error*|*问题*)
            echo "📋 相关帮助: 问题排除"
            echo "• 查看详细的错误排除指南"
            echo "• 常见编译和部署问题解决"
            echo "• 获取社区支持"
            ;;
        *UTXO*|*utxo*)
            echo "📋 相关帮助: 核心概念 → UTXO模型"
            echo "• 理解WES的UTXO机制"
            echo "• 学习与账户模型的区别"
            echo "• 掌握UTXO的优势"
            ;;
        *)
            echo "未找到匹配的帮助信息"
            echo ""
            echo "建议搜索关键词："
            echo "• 编译、部署、代币、NFT、错误、UTXO"
            echo "• build、deploy、token、nft、error"
            ;;
    esac
    
    echo ""
    read -p "按回车键继续搜索，或输入q返回主菜单: " next
    
    if [[ "$next" != "q" ]]; then
        search_help
    fi
}

# ==================== 主程序 ====================

main() {
    # 检查是否有命令行参数
    if [[ $# -gt 0 ]]; then
        case $1 in
            getting-started)
                show_getting_started
                ;;
            templates)
                show_templates_help
                ;;
            tools)
                show_tools_help
                ;;
            troubleshooting)
                show_troubleshooting
                ;;
            examples)
                show_examples_help
                ;;
            concepts)
                show_concepts_help
                ;;
            *)
                echo "未知主题: $1"
                echo "可用主题: getting-started, templates, tools, troubleshooting, examples, concepts"
                exit 1
                ;;
        esac
        return
    fi
    
    # 交互式模式
    while true; do
        show_main_menu
        read -p "请选择 (1-8/q): " choice
        
        case $choice in
            1)
                show_getting_started
                ;;
            2)
                show_templates_help
                ;;
            3)
                show_tools_help
                ;;
            4)
                show_troubleshooting
                ;;
            5)
                show_examples_help
                ;;
            6)
                show_concepts_help
                ;;
            7)
                show_community_help
                ;;
            8)
                search_help
                ;;
            q|Q)
                clear
                print_header "👋 感谢使用WES帮助系统！"
                echo -e "${WHITE}希望这些信息对你的开发有帮助！${NC}"
                echo -e "${CYAN}记住：优秀的开发者都是从提问开始的 😊${NC}"
                echo ""
                echo -e "${GREEN}祝你在WES生态中开发出精彩的应用！${NC}"
                echo ""
                exit 0
                ;;
            *)
                echo -e "${RED}无效选择，请输入1-8或q${NC}"
                read -p "按回车键继续..."
                ;;
        esac
    done
}

# 运行主程序
main "$@"
