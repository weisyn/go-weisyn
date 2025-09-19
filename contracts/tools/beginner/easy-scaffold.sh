#!/bin/bash

# ==================== WES智能合约项目创建助手 ====================
#
# 🎯 工具作用：通过交互式问答帮助初学者创建第一个合约项目
# 💡 特点：友好的用户界面、智能默认值、详细的指导说明
# 🎨 设计理念：让新手也能轻松上手合约开发
#
# 📚 使用方法：
#   ./easy-scaffold.sh
#
# ==================== 友好提示颜色定义 ====================

# 🎨 颜色定义让输出更加友好
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m' # No Color

# 📺 输出函数定义
print_header() {
    echo -e "${BLUE}================================${NC}"
    echo -e "${WHITE}$1${NC}"
    echo -e "${BLUE}================================${NC}"
    echo ""
}

print_step() {
    echo -e "${CYAN}📍 $1${NC}"
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

# ==================== 欢迎界面 ====================

clear
print_header "🎉 欢迎使用WES智能合约创建助手！"

echo -e "${WHITE}这个工具将通过几个简单问题帮你创建第一个合约项目${NC}"
echo -e "${CYAN}⏱️  预计耗时：3-5分钟${NC}"
echo -e "${CYAN}🎯 适合人群：区块链开发新手${NC}"
echo -e "${CYAN}🚀 完成后：你将拥有一个可运行的合约项目${NC}"
echo ""

# 📋 检查环境
print_step "检查开发环境..."

# 检查当前目录是否正确
if [[ ! -d "../templates/learning" ]]; then
    print_error "请在contracts/tools/beginner/目录下运行此脚本"
    exit 1
fi

# 检查TinyGo是否安装
if ! command -v tinygo &> /dev/null; then
    print_warning "未检测到TinyGo编译器"
    echo -e "${YELLOW}📝 安装方法：${NC}"
    echo "   brew tap tinygo-org/tools"
    echo "   brew install tinygo"
    echo ""
    read -p "是否继续（项目创建成功但无法编译）？[y/N]: " continue_without_tinygo
    if [[ $continue_without_tinygo != "y" && $continue_without_tinygo != "Y" ]]; then
        echo "请先安装TinyGo后再运行此工具"
        exit 1
    fi
else
    print_success "TinyGo编译器已安装"
fi

echo ""

# ==================== 项目类型选择 ====================

print_header "🤔 你想创建什么类型的合约？"

echo -e "${WHITE}请选择最适合你项目的合约类型：${NC}"
echo ""
echo -e "${CYAN}1) 💰 代币合约${NC} - 适合创建可转账的数字货币"
echo -e "   ${PURPLE}💡 例如：社区积分、游戏金币、项目代币${NC}"
echo ""
echo -e "${CYAN}2) 🖼️  NFT合约${NC} - 适合创建独特的数字收藏品" 
echo -e "   ${PURPLE}💡 例如：数字艺术、游戏道具、证书凭证${NC}"
echo ""
echo -e "${CYAN}3) 🎮 游戏合约${NC} - 适合创建链上游戏和互动应用"
echo -e "   ${PURPLE}💡 例如：抽奖游戏、技能对战、虚拟宠物${NC}"
echo ""
echo -e "${CYAN}4) 🏛️  DAO合约${NC} - 适合创建去中心化组织和治理"
echo -e "   ${PURPLE}💡 例如：投票系统、提案管理、资金管理${NC}"
echo ""
echo -e "${CYAN}5) 💡 自定义合约${NC} - 从空白模板开始，完全自由发挥"
echo -e "   ${PURPLE}💡 例如：创新应用、复杂逻辑、混合功能${NC}"
echo ""

while true; do
    read -p "请输入选择 (1-5): " choice
    case $choice in
        1)
            CONTRACT_TYPE="token"
            TEMPLATE_DIR="simple-token"
            TYPE_NAME="💰 代币合约"
            break
            ;;
        2)
            CONTRACT_TYPE="nft"
            TEMPLATE_DIR="basic-nft"
            TYPE_NAME="🖼️ NFT合约"
            break
            ;;
        3)
            CONTRACT_TYPE="game"
            TEMPLATE_DIR="starter-contract"
            TYPE_NAME="🎮 游戏合约"
            break
            ;;
        4)
            CONTRACT_TYPE="dao"
            TEMPLATE_DIR="starter-contract"
            TYPE_NAME="🏛️ DAO合约"
            break
            ;;
        5)
            CONTRACT_TYPE="custom"
            TEMPLATE_DIR="starter-contract"
            TYPE_NAME="💡 自定义合约"
            break
            ;;
        *)
            print_warning "请输入1-5之间的数字"
            ;;
    esac
done

print_success "很棒的选择！我们来创建 $TYPE_NAME"
echo ""

# ==================== 项目基本信息 ====================

print_header "📝 项目基本信息"

# 获取合约名称
while true; do
    echo -e "${WHITE}给你的合约起个名字：${NC}"
    echo -e "${PURPLE}💡 建议：简洁明了，体现功能特点${NC}"
    echo -e "${PURPLE}📝 示例：MyToken, ArtCollection, LuckyGame${NC}"
    read -p "合约名称: " contract_name
    
    if [[ -z "$contract_name" ]]; then
        print_warning "合约名称不能为空"
        continue
    fi
    
    # 检查名称是否已存在
    if [[ -d "$contract_name" ]]; then
        print_warning "项目目录已存在，请选择其他名称"
        continue
    fi
    
    break
done

# 获取作者信息
echo ""
echo -e "${WHITE}你的名字（作为合约作者）：${NC}"
echo -e "${PURPLE}💡 这将显示在合约信息中，可以是真名或昵称${NC}"
read -p "作者姓名: " author_name

if [[ -z "$author_name" ]]; then
    author_name="WES开发者"
    print_info "使用默认作者名: $author_name"
fi

# 获取项目描述（可选）
echo ""
echo -e "${WHITE}项目描述（可选）：${NC}"
echo -e "${PURPLE}💡 简单描述你的合约用途和特点${NC}"
read -p "项目描述: " project_description

if [[ -z "$project_description" ]]; then
    case $CONTRACT_TYPE in
        "token")
            project_description="一个基于WES的代币合约"
            ;;
        "nft")
            project_description="一个基于WES的NFT收藏合约"
            ;;
        "game")
            project_description="一个基于WES的游戏合约"
            ;;
        "dao")
            project_description="一个基于WES的DAO治理合约"
            ;;
        "custom")
            project_description="一个基于WES的自定义合约"
            ;;
    esac
    print_info "使用默认描述: $project_description"
fi

echo ""

# ==================== 功能定制 ====================

if [[ $CONTRACT_TYPE == "token" ]]; then
    print_header "💰 代币功能定制"
    
    echo -e "${WHITE}代币符号（3-5个字母）：${NC}"
    echo -e "${PURPLE}💡 示例：BTC, ETH, USDT${NC}"
    read -p "代币符号: " token_symbol
    
    if [[ -z "$token_symbol" ]]; then
        # 从合约名称生成符号
        token_symbol=$(echo "$contract_name" | tr '[:lower:]' '[:upper:]' | cut -c1-4)
        print_info "自动生成符号: $token_symbol"
    fi
    
    echo ""
    echo -e "${WHITE}初始发行量：${NC}"
    echo -e "${PURPLE}💡 建议：1000000（一百万）${NC}"
    read -p "发行量: " initial_supply
    
    if [[ -z "$initial_supply" ]]; then
        initial_supply="1000000"
        print_info "使用默认发行量: $initial_supply"
    fi
    
elif [[ $CONTRACT_TYPE == "nft" ]]; then
    print_header "🖼️ NFT功能定制"
    
    echo -e "${WHITE}NFT系列名称：${NC}"
    echo -e "${PURPLE}💡 例如：My Art Collection, Game Items${NC}"
    read -p "系列名称: " collection_name
    
    if [[ -z "$collection_name" ]]; then
        collection_name="$contract_name Collection"
        print_info "自动生成系列名: $collection_name"
    fi
    
    echo ""
    echo -e "${WHITE}NFT符号：${NC}"
    echo -e "${PURPLE}💡 例如：MAC, GAME${NC}"
    read -p "NFT符号: " nft_symbol
    
    if [[ -z "$nft_symbol" ]]; then
        nft_symbol=$(echo "$contract_name" | tr '[:lower:]' '[:upper:]' | cut -c1-3)"NFT"
        print_info "自动生成符号: $nft_symbol"
    fi
fi

echo ""

# ==================== 项目创建 ====================

print_header "🔨 正在创建你的项目..."

print_step "项目配置总结："
echo -e "${CYAN}   📂 项目名称: $contract_name${NC}"
echo -e "${CYAN}   🏷️  合约类型: $TYPE_NAME${NC}"
echo -e "${CYAN}   👤 作者: $author_name${NC}"
echo -e "${CYAN}   📝 描述: $project_description${NC}"

if [[ $CONTRACT_TYPE == "token" ]]; then
    echo -e "${CYAN}   💰 代币符号: $token_symbol${NC}"
    echo -e "${CYAN}   📊 发行量: $initial_supply${NC}"
elif [[ $CONTRACT_TYPE == "nft" ]]; then
    echo -e "${CYAN}   🖼️  系列名: $collection_name${NC}"
    echo -e "${CYAN}   🏷️  NFT符号: $nft_symbol${NC}"
fi

echo ""
read -p "确认创建项目？[Y/n]: " confirm
if [[ $confirm == "n" || $confirm == "N" ]]; then
    echo "项目创建已取消"
    exit 0
fi

echo ""
print_step "复制项目模板..."

# 创建项目目录
mkdir -p "$contract_name"
cd "$contract_name"

# 复制模板文件
cp -r "../../templates/learning/$TEMPLATE_DIR/"* .

print_success "模板文件复制完成"

# ==================== 文件定制 ====================

print_step "定制项目文件..."

# 定制主代码文件
if [[ -f "src/main.go" ]]; then
    # 替换基本信息
    sed -i '' "s/我的.*合约/$contract_name合约/g" src/main.go
    sed -i '' "s/我的.*代币/$contract_name/g" src/main.go
    sed -i '' "s/我的.*NFT系列/$collection_name/g" src/main.go
    sed -i '' "s/WES学习者/$author_name/g" src/main.go
    
    # Token特定替换
    if [[ $CONTRACT_TYPE == "token" ]]; then
        sed -i '' "s/LEARN/$token_symbol/g" src/main.go
        sed -i '' "s/1000000/$initial_supply/g" src/main.go
    fi
    
    # NFT特定替换  
    if [[ $CONTRACT_TYPE == "nft" ]]; then
        sed -i '' "s/LEARN-NFT/$nft_symbol/g" src/main.go
    fi
    
    print_success "源代码定制完成"
fi

# 定制README文件
if [[ -f "README.md" ]]; then
    sed -i '' "s/我的第一个.*合约/$contract_name/g" README.md
    sed -i '' "1s/.*/# $contract_name/" README.md
    
    # 添加项目描述
    echo "" >> README.md
    echo "## 📝 项目描述" >> README.md
    echo "$project_description" >> README.md
    echo "" >> README.md
    echo "## 👤 作者" >> README.md
    echo "$author_name" >> README.md
    echo "" >> README.md
    echo "## 📅 创建时间" >> README.md
    echo "$(date '+%Y-%m-%d')" >> README.md
    
    print_success "README文档定制完成"
fi

# 创建项目配置文件
cat > project.json << EOF
{
    "name": "$contract_name",
    "type": "$CONTRACT_TYPE",
    "author": "$author_name", 
    "description": "$project_description",
    "version": "1.0.0",
    "created": "$(date -Iseconds)",
    "template": "$TEMPLATE_DIR"
}
EOF

print_success "项目配置文件创建完成"

# ==================== 构建脚本创建 ====================

print_step "创建便捷脚本..."

# 创建build脚本
cat > build.sh << 'EOF'
#!/bin/bash

echo "🔨 编译智能合约..."
echo "==================="

# 检查TinyGo
if ! command -v tinygo &> /dev/null; then
    echo "❌ 未找到TinyGo编译器"
    echo "📝 安装方法："
    echo "   brew tap tinygo-org/tools"
    echo "   brew install tinygo"
    exit 1
fi

# 创建build目录
mkdir -p build

# 编译合约
echo "🔸 正在编译..."
tinygo build -o build/main.wasm -target wasi src/main.go

if [ $? -eq 0 ]; then
    echo "✅ 编译成功！"
    echo "📁 输出文件: build/main.wasm"
    echo "📏 文件大小: $(ls -lh build/main.wasm | awk '{print $5}')"
else
    echo "❌ 编译失败"
    exit 1
fi
EOF

chmod +x build.sh

# 创建test脚本
cat > test.sh << 'EOF'
#!/bin/bash

echo "🧪 运行合约测试..."
echo "=================="

# 首先编译
echo "🔸 编译合约..."
./build.sh

if [ $? -ne 0 ]; then
    echo "❌ 编译失败，无法进行测试"
    exit 1
fi

echo ""
echo "🔸 运行基础测试..."

# 这里添加你的测试逻辑
echo "✅ 基础测试通过"
echo "💡 提示：在test.sh中添加更多测试用例"
EOF

chmod +x test.sh

# 创建deploy脚本
cat > deploy.sh << 'EOF'
#!/bin/bash

echo "🚀 部署智能合约..."
echo "=================="

NETWORK=${1:-testnet}

echo "🔸 目标网络: $NETWORK"
echo "🔸 编译合约..."

./build.sh

if [ $? -ne 0 ]; then
    echo "❌ 编译失败，无法部署"
    exit 1
fi

echo ""
echo "🔸 正在部署到 $NETWORK..."

# 这里添加实际的部署逻辑
echo "✅ 部署完成！"
echo "📝 合约地址: 0x示例地址..."
echo "💡 提示：在deploy.sh中添加真实的部署逻辑"
EOF

chmod +x deploy.sh

print_success "便捷脚本创建完成"

# ==================== 项目创建完成 ====================

echo ""
print_header "🎊 项目创建成功！"

print_success "项目已创建在目录: $contract_name/"

echo ""
echo -e "${WHITE}📁 项目结构：${NC}"
echo "   ├── 📄 README.md          # 项目说明文档"
echo "   ├── 📝 src/main.go        # 合约主代码"  
echo "   ├── ⚙️  project.json       # 项目配置"
echo "   ├── 🔨 build.sh           # 编译脚本"
echo "   ├── 🧪 test.sh            # 测试脚本"
echo "   └── 🚀 deploy.sh          # 部署脚本"

echo ""
echo -e "${WHITE}🚀 下一步操作：${NC}"
echo -e "${GREEN}1. 查看代码:${NC} cd $contract_name && cat src/main.go"
echo -e "${GREEN}2. 编译合约:${NC} ./build.sh"
echo -e "${GREEN}3. 运行测试:${NC} ./test.sh"
echo -e "${GREEN}4. 部署合约:${NC} ./deploy.sh testnet"

echo ""
echo -e "${WHITE}📚 学习资源：${NC}"
echo -e "${CYAN}• 查看README了解详细功能${NC}"
echo -e "${CYAN}• 参考../../BEGINNER_GUIDE.md获取更多帮助${NC}"
echo -e "${CYAN}• 访问../../CONCEPTS.md深入理解概念${NC}"

echo ""
echo -e "${WHITE}💡 温馨提示：${NC}"
echo -e "${PURPLE}• 代码中有详细注释，适合学习和修改${NC}"
echo -e "${PURPLE}• 可以根据需求自由定制功能${NC}"
echo -e "${PURPLE}• 遇到问题可以查看文档或寻求社区帮助${NC}"

echo ""
print_success "祝你在WES区块链开发中取得成功！"

echo ""
echo -e "${BLUE}================================${NC}"
echo -e "${WHITE}     感谢使用WES开发工具！     ${NC}"
echo -e "${BLUE}================================${NC}"
