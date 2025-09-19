#!/bin/bash

# 历史清理脚本：WES/Weisyn 项目名称标准化
# 清理项目中的历史遗留名称（tvc, bpfs等）统一为weisyn/WES

set -e

echo "🔄 开始项目名称标准化清理..."

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 替换映射规则
# 1. 版本号：0.0.1 -> 0.0.1
# 2. 大写场景：WES -> WES  
# 3. 小写场景：weisyn -> weisyn
# 4. URL和仓库：github.com/weisyn -> github.com/weisyn
# 5. import路径：github.com/weisyn/v1 -> github.com/weisyn/v1

echo "📋 替换映射规则："
echo "  版本: 0.0.1 -> 0.0.1"
echo "  大写: WES -> WES"
echo "  小写: weisyn -> weisyn" 
echo "  仓库: github.com/weisyn -> github.com/weisyn"
echo "  模块: github.com/weisyn/v1 -> github.com/weisyn/v1"

# 创建替换函数
replace_in_files() {
    local pattern="$1"
    local replacement="$2"
    local description="$3"
    
    echo "🔧 $description"
    
    # 查找所有相关文件，排除二进制文件和特定目录
    find . -type f \( \
        -name "*.go" -o \
        -name "*.md" -o \
        -name "*.json" -o \
        -name "*.yaml" -o \
        -name "*.yml" -o \
        -name "*.sh" -o \
        -name "*.py" -o \
        -name "*.proto" -o \
        -name "*.txt" -o \
        -name "Makefile" -o \
        -name "*.toml" \
    \) \
    ! -path "./vendor/*" \
    ! -path "./.git/*" \
    ! -path "./bin/*" \
    ! -path "./data/logs/*" \
    ! -path "./server.log" \
    ! -path "./config-temp/*" \
    ! -path "*/node_modules/*" \
    -print0 | xargs -0 grep -l "$pattern" 2>/dev/null | while read -r file; do
        echo "  📝 更新: $file"
        # 使用临时文件进行替换，避免sed在macOS上的问题
        sed "s|$pattern|$replacement|g" "$file" > "$file.tmp" && mv "$file.tmp" "$file"
    done
}

# 执行替换，按优先级和安全性排序

echo "📦 第一阶段：版本信息替换"
replace_in_files "4\.0\.0" "0.0.1" "版本号 0.0.1 -> 0.0.1"

echo "📦 第二阶段：模块和import路径替换"  
replace_in_files "github\.com/weisyn/v4" "github.com/weisyn/v1" "Go模块路径 github.com/weisyn/v1 -> github.com/weisyn/v1"

echo "📦 第三阶段：GitHub仓库URL替换"
replace_in_files "github\.com/weisyn/weisyn4\.0\.0" "github.com/weisyn/weisyn" "GitHub仓库URL"
replace_in_files "github\.com/weisyn" "github.com/weisyn" "GitHub组织路径"

echo "📦 第四阶段：项目名称替换"
# 大写WES -> WES（适用于标题、常量、文档标题等）
replace_in_files "WES" "WES" "大写项目名称 WES -> WES"

# 小写weisyn -> weisyn（适用于命名空间、文件名、变量名等）
replace_in_files "weisyn" "weisyn" "小写项目名称 weisyn -> weisyn"

echo "📦 第五阶段：特殊情况处理"

# 处理中文描述中的WES
replace_in_files "WES (Weisyn Chain)" "WES (Weisyn Chain)" "项目全称描述"
replace_in_files "WES —— 微迅链可信可控的企业级数字基础设施" "WES —— 微迅链可信可控的企业级数字基础设施" "中文项目描述"

# 处理文档中的详细描述（先处理特殊的，再处理一般的）
replace_in_files "让企业的数据、AI模型、业务逻辑可以在微迅链分布式网络上自主可控运行，同时获得区块链级的可信保障。" \
"让企业的数据、AI模型、业务逻辑可以在微迅链分布式网络上自主可控运行，同时获得区块链级的可信保障。" \
"业务描述更新"

# 处理网络命名空间
replace_in_files '"weisyn"' '"weisyn"' "网络命名空间已正确"

echo "📦 第六阶段：文档标题和链接更新" 
# 更新markdown文档链接
replace_in_files "\[WES Logo\]" "[Weisyn Logo]" "Logo引用更新"
replace_in_files "docs/assets/logo\.png" "docs/assets/weisyn-logo.png" "Logo文件路径"

# 特殊文件路径处理
if [ -f "tmp/weisyn-development-config-2668070175.json" ]; then
    echo "  📝 重命名临时配置文件"
    mv "tmp/weisyn-development-config-2668070175.json" "tmp/weisyn-development-config-2668070175.json" 2>/dev/null || true
fi

echo "📦 第七阶段：验证关键文件"

# 验证关键文件的替换结果
echo "🔍 验证替换结果..."

if grep -q "github.com/weisyn/v1" go.mod 2>/dev/null; then
    echo "  ✅ go.mod 模块路径已更新"
else
    echo "  ❌ go.mod 模块路径更新失败"
fi

if grep -q "WES.*微迅链" README.md 2>/dev/null; then
    echo "  ✅ README.md 项目描述已更新"
else
    echo "  ❌ README.md 项目描述更新可能有问题"
fi

if grep -q "0.0.1" internal/cli/version/info.go 2>/dev/null; then
    echo "  ✅ 版本信息已更新"
else
    echo "  ❌ 版本信息更新失败"
fi

echo ""
echo "🎉 替换完成！"
echo ""
echo "📋 下一步操作建议："
echo "  1. 执行: go mod tidy  # 更新依赖"
echo "  2. 执行: go build ./...  # 验证编译"
echo "  3. 执行: go test ./...   # 运行测试"
echo "  4. 手动检查可能需要特殊处理的文件"
echo ""
echo "⚠️  需要手动检查的文件类型："
echo "  - 二进制文件和日志文件可能仍包含旧路径"
echo "  - 图片和资产文件的文件名"
echo "  - 外部服务配置中的URL引用"
echo ""

# 显示统计信息
echo "📊 替换统计："
echo "  - 搜索到的文件总数: $(find . -type f \( -name "*.go" -o -name "*.md" -o -name "*.json" \) ! -path "./vendor/*" ! -path "./.git/*" ! -path "./data/logs/*" | wc -l)"
echo "  - 当前仍包含'WES'的文件: $(find . -type f -name "*.go" -o -name "*.md" | xargs grep -l "WES" 2>/dev/null | wc -l)"
echo "  - 当前仍包含'github.com/weisyn'的文件: $(find . -type f -name "*.go" | xargs grep -l "github.com/weisyn" 2>/dev/null | wc -l)"

echo ""
echo "替换脚本执行完毕 ✨"
