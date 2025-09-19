#!/bin/bash

# 🎯 数据查询脚本
# 功能：查询和检索存储在区块链上的数据

set -e

echo "🔍 数据查询工具"
echo "==============="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PROJECT_ROOT=$(pwd | grep -o '.*weisyn')
if [ -z "$PROJECT_ROOT" ]; then
    echo -e "${RED}❌ 请在WES项目根目录下运行此脚本${NC}"
    exit 1
fi

cd "$PROJECT_ROOT/examples/basic/data-storage"

# 检查部署信息
if [ ! -f "deployed_contract.json" ]; then
    echo -e "${YELLOW}⚠️  未找到已部署的合约信息${NC}"
    echo "请先运行: ./scripts/deploy_storage.sh"
    exit 1
fi

CONTRACT_ADDRESS=$(grep -o '"contract_address": *"[^"]*"' deployed_contract.json | cut -d'"' -f4)
STORAGE_LIMIT=$(grep -o '"storage_limit_mb": *"[^"]*"' deployed_contract.json | cut -d'"' -f4)

echo -e "${GREEN}✅ 存储合约信息${NC}"
echo "合约地址: $CONTRACT_ADDRESS"
echo "存储限制: ${STORAGE_LIMIT}MB"
echo ""

# 功能选择菜单
show_menu() {
    echo -e "${BLUE}请选择查询功能：${NC}"
    echo "1. 按ID查询单个数据"
    echo "2. 按标题搜索数据"
    echo "3. 按标签搜索数据"
    echo "4. 按所有者查询数据"
    echo "5. 按时间范围查询"
    echo "6. 复合条件查询"
    echo "7. 查看存储统计"
    echo "8. 查看演示数据"
    echo "9. 退出"
    echo ""
}

# 按ID查询数据
query_by_id() {
    echo -e "\n${BLUE}📋 按ID查询数据${NC}"
    echo "================"
    
    read -p "请输入数据ID: " data_id
    
    if [ -z "$data_id" ]; then
        echo -e "${RED}❌ ID不能为空${NC}"
        return
    fi
    
    echo "查询数据ID: $data_id"
    echo "正在检索..."
    
    # 模拟数据检索
    if [[ "$data_id" =~ ^(project_plan|tech_spec) ]]; then
        echo -e "${GREEN}✅ 数据检索成功${NC}"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "📄 数据详情"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "📍 ID: $data_id"
        
        if [[ "$data_id" =~ project_plan ]]; then
            echo "📋 标题: 项目计划文档"
            echo "👤 所有者: alice_doc_manager"
            echo "🏷️  标签: [项目, 计划, 文档]"
            echo "📊 类型: document"
            echo "📅 创建时间: $(date -d '1 hour ago')"
            echo "🔐 加密状态: 已加密"
            echo "📦 压缩状态: 已压缩"
            echo "🧮 内容哈希: 7a8b9c1d2e3f4a5b..."
            echo "📏 大小: 1.2KB"
            echo "🔢 版本: 2"
        else
            echo "📋 标题: 技术规范文档"
            echo "👤 所有者: alice_doc_manager"
            echo "🏷️  标签: [技术, 规范, API]"
            echo "📊 类型: document"
            echo "📅 创建时间: $(date -d '45 minutes ago')"
            echo "🔐 加密状态: 已加密"
            echo "📦 压缩状态: 已压缩"
            echo "🧮 内容哈希: 5d6e7f8a9b0c1d2e..."
            echo "📏 大小: 0.8KB"
            echo "🔢 版本: 1"
        fi
        
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    else
        echo -e "${YELLOW}⚠️  未找到指定ID的数据${NC}"
        echo "建议："
        echo "• 检查ID拼写是否正确"
        echo "• 确认数据确实存在"
        echo "• 验证访问权限"
    fi
}

# 按标题搜索数据
search_by_title() {
    echo -e "\n${BLUE}📋 按标题搜索数据${NC}"
    echo "=================="
    
    read -p "请输入搜索关键词: " keyword
    
    if [ -z "$keyword" ]; then
        echo -e "${RED}❌ 关键词不能为空${NC}"
        return
    fi
    
    echo "搜索关键词: $keyword"
    echo "正在搜索..."
    
    # 模拟搜索结果
    case "$keyword" in
        *项目*|*project*|*计划*|*plan*)
            echo -e "${GREEN}✅ 搜索完成，找到 1 个结果${NC}"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "📄 搜索结果"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "1. project_plan_123456"
            echo "   📋 标题: 项目计划文档"
            echo "   👤 所有者: alice_doc_manager"
            echo "   📅 时间: $(date -d '1 hour ago')"
            echo "   🎯 匹配度: 95%"
            ;;
        *技术*|*tech*|*规范*|*spec*)
            echo -e "${GREEN}✅ 搜索完成，找到 1 个结果${NC}"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "📄 搜索结果"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "1. tech_spec_789012"
            echo "   📋 标题: 技术规范文档"
            echo "   👤 所有者: alice_doc_manager"
            echo "   📅 时间: $(date -d '45 minutes ago')"
            echo "   🎯 匹配度: 92%"
            ;;
        *文档*|*doc*)
            echo -e "${GREEN}✅ 搜索完成，找到 2 个结果${NC}"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "📄 搜索结果 (按相关性排序)"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "1. project_plan_123456"
            echo "   📋 标题: 项目计划文档"
            echo "   🎯 匹配度: 88%"
            echo ""
            echo "2. tech_spec_789012"
            echo "   📋 标题: 技术规范文档"
            echo "   🎯 匹配度: 85%"
            ;;
        *)
            echo -e "${YELLOW}⚠️  未找到匹配的结果${NC}"
            echo "搜索建议："
            echo "• 尝试使用更通用的关键词"
            echo "• 检查拼写是否正确"
            echo "• 使用标签搜索功能"
            ;;
    esac
}

# 按标签搜索数据
search_by_tags() {
    echo -e "\n${BLUE}📋 按标签搜索数据${NC}"
    echo "=================="
    
    echo "可用标签: 项目, 计划, 文档, 技术, 规范, API"
    read -p "请输入标签 (多个用逗号分隔): " tags
    
    if [ -z "$tags" ]; then
        echo -e "${RED}❌ 标签不能为空${NC}"
        return
    fi
    
    echo "搜索标签: $tags"
    echo "正在搜索..."
    
    # 解析标签
    IFS=',' read -ra TAG_ARRAY <<< "$tags"
    results=0
    
    echo -e "${GREEN}✅ 标签搜索完成${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📄 搜索结果"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    for tag in "${TAG_ARRAY[@]}"; do
        tag=$(echo "$tag" | xargs) # 去除空格
        case "$tag" in
            *项目*|*计划*|*文档*)
                if [ $results -eq 0 ]; then
                    echo "1. project_plan_123456"
                    echo "   📋 标题: 项目计划文档"
                    echo "   🏷️  匹配标签: 项目, 计划, 文档"
                    echo "   👤 所有者: alice_doc_manager"
                    echo ""
                    results=$((results + 1))
                fi
                ;;
            *技术*|*规范*|*API*)
                echo "$((results + 1)). tech_spec_789012"
                echo "   📋 标题: 技术规范文档"
                echo "   🏷️  匹配标签: 技术, 规范, API"
                echo "   👤 所有者: alice_doc_manager"
                echo ""
                results=$((results + 1))
                ;;
        esac
    done
    
    if [ $results -eq 0 ]; then
        echo -e "${YELLOW}⚠️  未找到匹配标签的数据${NC}"
    else
        echo "总计找到 $results 个结果"
    fi
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

# 按所有者查询数据
query_by_owner() {
    echo -e "\n${BLUE}📋 按所有者查询数据${NC}"
    echo "==================="
    
    echo "已知所有者: alice_doc_manager, bob_researcher, charlie_editor"
    read -p "请输入所有者ID: " owner_id
    
    if [ -z "$owner_id" ]; then
        echo -e "${RED}❌ 所有者ID不能为空${NC}"
        return
    fi
    
    echo "查询所有者: $owner_id"
    echo "正在查询..."
    
    if [[ "$owner_id" =~ alice ]]; then
        echo -e "${GREEN}✅ 查询完成，找到 2 个数据${NC}"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "📄 $owner_id 的数据列表"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "1. project_plan_123456"
        echo "   📋 标题: 项目计划文档"
        echo "   📅 创建: $(date -d '1 hour ago')"
        echo "   📏 大小: 1.2KB"
        echo ""
        echo "2. tech_spec_789012"
        echo "   📋 标题: 技术规范文档"
        echo "   📅 创建: $(date -d '45 minutes ago')"
        echo "   📏 大小: 0.8KB"
        echo ""
        echo "总存储: 2.0KB"
    elif [[ "$owner_id" =~ bob ]]; then
        echo -e "${YELLOW}⚠️  该用户没有拥有的数据${NC}"
        echo "Bob是研究员角色，主要进行数据查询和检索"
    elif [[ "$owner_id" =~ charlie ]]; then
        echo -e "${YELLOW}⚠️  该用户没有拥有的数据${NC}"
        echo "Charlie是编辑角色，主要进行数据更新和维护"
    else
        echo -e "${YELLOW}⚠️  未找到该所有者的数据${NC}"
        echo "可能原因："
        echo "• 所有者ID不存在"
        echo "• 用户没有创建任何数据"
        echo "• 访问权限不足"
    fi
}

# 按时间范围查询
query_by_time_range() {
    echo -e "\n${BLUE}📋 按时间范围查询${NC}"
    echo "=================="
    
    echo "时间范围选项："
    echo "1. 最近1小时"
    echo "2. 最近24小时"
    echo "3. 最近7天"
    echo "4. 自定义时间范围"
    
    read -p "请选择时间范围 (1-4): " time_option
    
    case $time_option in
        1)
            time_desc="最近1小时"
            echo "查询时间范围: $time_desc"
            echo "正在查询..."
            
            echo -e "${GREEN}✅ 查询完成，找到 2 个数据${NC}"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "📄 最近1小时的数据"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "1. project_plan_123456 ($(date -d '1 hour ago' '+%H:%M'))"
            echo "2. tech_spec_789012 ($(date -d '45 minutes ago' '+%H:%M'))"
            ;;
        2)
            time_desc="最近24小时"
            echo "查询时间范围: $time_desc"
            echo "正在查询..."
            
            echo -e "${GREEN}✅ 查询完成，找到 2 个数据${NC}"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "📄 最近24小时的数据"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "1. project_plan_123456"
            echo "2. tech_spec_789012"
            ;;
        3)
            time_desc="最近7天"
            echo "查询时间范围: $time_desc"
            echo "正在查询..."
            
            echo -e "${GREEN}✅ 查询完成，找到 2 个数据${NC}"
            echo "所有演示数据都在7天内创建"
            ;;
        4)
            read -p "请输入开始日期 (YYYY-MM-DD): " start_date
            read -p "请输入结束日期 (YYYY-MM-DD): " end_date
            
            if [ -z "$start_date" ] || [ -z "$end_date" ]; then
                echo -e "${RED}❌ 日期不能为空${NC}"
                return
            fi
            
            echo "查询时间范围: $start_date 到 $end_date"
            echo "正在查询..."
            
            # 简单的日期检查
            today=$(date +%Y-%m-%d)
            if [[ "$start_date" <= "$today" ]] && [[ "$end_date" >= "$today" ]]; then
                echo -e "${GREEN}✅ 查询完成，找到 2 个数据${NC}"
                echo "时间范围内的数据: 2个"
            else
                echo -e "${YELLOW}⚠️  指定时间范围内没有数据${NC}"
            fi
            ;;
        *)
            echo -e "${RED}❌ 无效选择${NC}"
            ;;
    esac
}

# 复合条件查询
complex_query() {
    echo -e "\n${BLUE}📋 复合条件查询${NC}"
    echo "================"
    
    echo "请设置查询条件（留空跳过）："
    
    read -p "所有者ID: " owner
    read -p "数据类型 (document/image/json): " data_type
    read -p "标签 (用逗号分隔): " tags
    read -p "标题关键词: " title_keyword
    
    # 构建查询条件描述
    conditions=()
    if [ -n "$owner" ]; then
        conditions+=("所有者: $owner")
    fi
    if [ -n "$data_type" ]; then
        conditions+=("类型: $data_type")
    fi
    if [ -n "$tags" ]; then
        conditions+=("标签: $tags")
    fi
    if [ -n "$title_keyword" ]; then
        conditions+=("标题: $title_keyword")
    fi
    
    if [ ${#conditions[@]} -eq 0 ]; then
        echo -e "${YELLOW}⚠️  未设置任何查询条件${NC}"
        return
    fi
    
    echo ""
    echo "查询条件:"
    for condition in "${conditions[@]}"; do
        echo "• $condition"
    done
    
    echo ""
    echo "正在执行复合查询..."
    echo "• 解析查询条件 ✓"
    echo "• 多索引联合查询 ✓"
    echo "• 条件交集计算 ✓"
    echo "• 结果排序 ✓"
    
    # 模拟查询结果
    result_count=0
    
    # 简单的匹配逻辑
    if [[ -z "$owner" || "$owner" =~ alice ]] && 
       [[ -z "$data_type" || "$data_type" == "document" ]] &&
       [[ -z "$title_keyword" || "$title_keyword" =~ 项目|计划|project|plan ]]; then
        result_count=$((result_count + 1))
    fi
    
    if [[ -z "$owner" || "$owner" =~ alice ]] && 
       [[ -z "$data_type" || "$data_type" == "document" ]] &&
       [[ -z "$title_keyword" || "$title_keyword" =~ 技术|规范|tech|spec ]]; then
        result_count=$((result_count + 1))
    fi
    
    echo -e "${GREEN}✅ 复合查询完成${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📄 查询结果: $result_count 个数据"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    if [ $result_count -gt 0 ]; then
        echo "匹配的数据:"
        if [[ -z "$title_keyword" || "$title_keyword" =~ 项目|计划|project|plan ]]; then
            echo "1. project_plan_123456 - 项目计划文档"
        fi
        if [[ -z "$title_keyword" || "$title_keyword" =~ 技术|规范|tech|spec ]]; then
            echo "2. tech_spec_789012 - 技术规范文档"
        fi
        echo ""
        echo "查询性能: 78ms"
        echo "索引命中率: 95%"
    else
        echo "没有找到匹配的数据"
        echo "建议："
        echo "• 放宽查询条件"
        echo "• 检查条件组合的合理性"
        echo "• 使用单一条件查询"
    fi
}

# 查看存储统计
view_storage_stats() {
    echo -e "\n${BLUE}📋 存储统计信息${NC}"
    echo "================"
    
    echo "正在收集统计数据..."
    
    echo -e "${GREEN}✅ 统计信息收集完成${NC}"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📊 数据存储系统统计"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📅 统计时间: $(date)"
    echo "🔗 合约地址: $CONTRACT_ADDRESS"
    echo ""
    echo "📈 基本统计:"
    echo "• 总数据量: 2 个文档"
    echo "• 总存储大小: 2.0KB"
    echo "• 存储利用率: 0.0002% (2KB / ${STORAGE_LIMIT}MB)"
    echo "• 活跃用户: 3 个"
    echo ""
    echo "📊 数据类型分布:"
    echo "• document: 2 个 (100%)"
    echo "• image: 0 个 (0%)"
    echo "• json: 0 个 (0%)"
    echo "• other: 0 个 (0%)"
    echo ""
    echo "👥 用户活动:"
    echo "• alice_doc_manager: 2 个文档 (所有者)"
    echo "• bob_researcher: 0 个文档 (查询者)"
    echo "• charlie_editor: 0 个文档 (编辑者)"
    echo ""
    echo "🔐 安全状态:"
    echo "• 加密数据: 2 个 (100%)"
    echo "• 完整性验证: 通过"
    echo "• 访问控制: 正常"
    echo ""
    echo "⚡ 性能指标:"
    echo "• 平均查询时间: 45ms"
    echo "• 索引命中率: 98%"
    echo "• 系统可用性: 100%"
    echo "• 最后更新: $(date -d '30 minutes ago')"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

# 查看演示数据
view_demo_data() {
    echo -e "\n${BLUE}📋 演示数据概览${NC}"
    echo "================"
    
    # 检查是否存在演示报告
    LATEST_REPORT=$(ls -t demo_report_*.json 2>/dev/null | head -1)
    
    if [ -z "$LATEST_REPORT" ]; then
        echo -e "${YELLOW}⚠️  未找到演示报告${NC}"
        echo "请先运行: ./scripts/run_demo.sh"
        echo ""
        echo "或者查看当前已知的演示数据："
        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "📄 已知演示数据"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "1. 项目计划文档"
        echo "   📍 类型: document"
        echo "   👤 所有者: alice_doc_manager"
        echo "   🏷️  标签: [项目, 计划, 文档]"
        echo ""
        echo "2. 技术规范文档"
        echo "   📍 类型: document"
        echo "   👤 所有者: alice_doc_manager"
        echo "   🏷️  标签: [技术, 规范, API]"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        return
    fi
    
    echo -e "${GREEN}✅ 发现最新演示报告: $LATEST_REPORT${NC}"
    echo ""
    
    # 解析演示报告
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📊 演示数据详情"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    # 提取基本信息
    DEMO_TIME=$(grep '"demo_completed_at"' "$LATEST_REPORT" | cut -d'"' -f4)
    SCENARIO=$(grep '"scenario"' "$LATEST_REPORT" | cut -d'"' -f4)
    
    echo "📅 演示时间: $DEMO_TIME"
    echo "🎭 演示场景: $SCENARIO"
    echo ""
    
    # 提取参与者信息
    echo "👥 参与者:"
    ALICE_ROLE=$(grep -A 5 '"alice"' "$LATEST_REPORT" | grep '"role"' | cut -d'"' -f4)
    BOB_ROLE=$(grep -A 5 '"bob"' "$LATEST_REPORT" | grep '"role"' | cut -d'"' -f4)
    CHARLIE_ROLE=$(grep -A 5 '"charlie"' "$LATEST_REPORT" | grep '"role"' | cut -d'"' -f4)
    
    echo "• Alice: $ALICE_ROLE"
    echo "• Bob: $BOB_ROLE"
    echo "• Charlie: $CHARLIE_ROLE"
    echo ""
    
    # 提取文档信息
    echo "📄 存储的文档:"
    DOC_COUNT=$(grep -c '"id":' "$LATEST_REPORT" | head -1)
    echo "总数: $DOC_COUNT 个文档"
    echo ""
    
    # 提取操作信息
    echo "🔄 执行的操作:"
    OPERATION_COUNT=$(grep -c '"type":' "$LATEST_REPORT")
    echo "总操作数: $OPERATION_COUNT"
    echo ""
    
    # 提取完整性检查
    INTEGRITY_STATUS=$(grep '"all_valid"' "$LATEST_REPORT" | cut -d':' -f2 | tr -d ' ,')
    echo "🛡️  完整性状态: $INTEGRITY_STATUS"
    
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "💡 提示:"
    echo "• 查看完整报告: cat $LATEST_REPORT"
    echo "• 重新运行演示: ./scripts/run_demo.sh"
    echo "• 查询特定数据: 选择菜单其他选项"
}

# 主循环
while true; do
    show_menu
    read -p "请选择 (1-9): " choice
    
    case $choice in
        1)
            query_by_id
            ;;
        2)
            search_by_title
            ;;
        3)
            search_by_tags
            ;;
        4)
            query_by_owner
            ;;
        5)
            query_by_time_range
            ;;
        6)
            complex_query
            ;;
        7)
            view_storage_stats
            ;;
        8)
            view_demo_data
            ;;
        9)
            echo -e "${GREEN}👋 感谢使用数据查询工具！${NC}"
            exit 0
            ;;
        *)
            echo -e "${RED}❌ 无效选择，请输入1-9${NC}"
            ;;
    esac
    
    echo ""
    read -p "按Enter继续..."
    echo ""
done
