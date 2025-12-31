#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# WES 发布脚本
# ═══════════════════════════════════════════════════════════════════════════════
#
# 用法：
#   ./scripts/release.sh --sync              # 仅同步代码（不打版）
#   ./scripts/release.sh --version 1.0.0     # 发布指定版本
#   ./scripts/release.sh --dry-run           # 模拟运行（不实际复制）
#   ./scripts/release.sh --help              # 显示帮助
#
# 配置文件：design/publishing/release.config.yml
#   - 定义排除规则、发布目录等配置
#   - 脚本会从配置文件读取 exclude 列表
#
# ═══════════════════════════════════════════════════════════════════════════════

set -euo pipefail

# ───────────────────────────────────────────────────────────────────────────────
# 配置变量
# ───────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG_FILE="$PROJECT_ROOT/design/publishing/release.config.yml"

# WES 工作区根目录
WES_ROOT="$(cd "$PROJECT_ROOT/.." && pwd)"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ───────────────────────────────────────────────────────────────────────────────
# 辅助函数
# ───────────────────────────────────────────────────────────────────────────────

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# ───────────────────────────────────────────────────────────────────────────────
# 从配置文件读取
# ───────────────────────────────────────────────────────────────────────────────

# 读取 pub_path
get_pub_path() {
    if [[ -f "$CONFIG_FILE" ]]; then
        grep "^pub_path:" "$CONFIG_FILE" | sed 's/pub_path:[[:space:]]*"\{0,1\}\([^"]*\)"\{0,1\}/\1/' | tr -d ' '
    else
        echo "_weisyn/go-weisyn"
    fi
}

# 从 release.config.yml 读取排除规则并生成 rsync --exclude 参数
# 配置文件格式：
#   exclude:
#     - ".git"
#     - "_dev"
#     - "*.log"
get_exclude_args_from_config() {
    if [[ ! -f "$CONFIG_FILE" ]]; then
        log_warning "配置文件不存在: $CONFIG_FILE，使用默认排除规则"
        get_default_exclude_args
        return
    fi

    # 解析 YAML 的 exclude 列表
    # 使用 awk 提取 exclude: 下的所有 - "xxx" 行
    local in_exclude=0
    local exclude_args=""
    
    while IFS= read -r line; do
        # 检测 exclude: 开始
        if [[ "$line" =~ ^exclude: ]]; then
            in_exclude=1
            continue
        fi
        
        # 如果遇到其他顶级键，停止
        if [[ $in_exclude -eq 1 && "$line" =~ ^[a-z_]+: && ! "$line" =~ ^[[:space:]] ]]; then
            in_exclude=0
            continue
        fi
        
        # 在 exclude 块内，提取排除项
        if [[ $in_exclude -eq 1 && "$line" =~ ^[[:space:]]*-[[:space:]]+ ]]; then
            # 提取引号内或无引号的值
            local value
            value=$(echo "$line" | sed 's/^[[:space:]]*-[[:space:]]*"\{0,1\}\([^"#]*\)"\{0,1\}.*/\1/' | sed 's/[[:space:]]*$//')
            if [[ -n "$value" ]]; then
                exclude_args+="--exclude=$value"$'\n'
            fi
        fi
    done < "$CONFIG_FILE"
    
    if [[ -z "$exclude_args" ]]; then
        log_warning "配置文件中未找到排除规则，使用默认规则"
        get_default_exclude_args
        return
    fi
    
    echo "$exclude_args"
}

# 默认排除规则（当配置文件不存在或解析失败时使用）
get_default_exclude_args() {
    cat << 'EOF'
--exclude=.git
--exclude=.idea
--exclude=.vscode
--exclude=.cursor
--exclude=.DS_Store
--exclude=Thumbs.db
--exclude=*.swp
--exclude=*.swo
--exclude=*~
--exclude=_dev
--exclude=_docs
--exclude=design
--exclude=reports
--exclude=bin
--exclude=data
--exclude=config-temp
--exclude=tmp
--exclude=cache
--exclude=coverage
--exclude=vendor
--exclude=test
--exclude=*.test
--exclude=*.out
--exclude=*.log
--exclude=*.ndjson
--exclude=.cache
--exclude=*.a
--exclude=*.o
--exclude=*.wasm
--exclude=node_modules
--exclude=dist
EOF
}

show_help() {
    local pub_path
    pub_path=$(get_pub_path)
    
    cat << EOF
WES 发布脚本

用法：
    ./scripts/release.sh [选项]

选项：
    --sync              仅同步代码到发布目录（不打版）
    --version <ver>     发布指定版本（如 1.0.0）
    --dry-run           模拟运行，显示将要执行的操作但不实际执行
    --clean             清理发布目录后再同步
    --check             仅执行发布前检查，不同步代码
    --show-excludes     显示当前排除规则
    --help              显示此帮助信息

示例：
    ./scripts/release.sh --sync              # 同步代码到发布目录
    ./scripts/release.sh --version 1.0.0     # 发布 v1.0.0 版本
    ./scripts/release.sh --sync --dry-run    # 模拟同步，查看将复制哪些文件
    ./scripts/release.sh --sync --clean      # 清理后同步
    ./scripts/release.sh --show-excludes     # 查看排除规则

目录结构：
    开发仓库: $PROJECT_ROOT
    发布目录: $WES_ROOT/$pub_path

配置文件: $CONFIG_FILE
    排除规则在配置文件的 exclude: 部分定义
EOF
}

# 显示排除规则
show_excludes() {
    log_info "排除规则来源: $CONFIG_FILE"
    echo ""
    echo "当前排除规则："
    echo "═══════════════════════════════════════════════════════════════"
    get_exclude_args_from_config | sed 's/--exclude=/  - /'
    echo "═══════════════════════════════════════════════════════════════"
    echo ""
    log_info "如需修改排除规则，请编辑: $CONFIG_FILE"
}

# ───────────────────────────────────────────────────────────────────────────────
# 发布前检查
# ───────────────────────────────────────────────────────────────────────────────

run_pre_checks() {
    log_info "执行发布前检查..."
    
    cd "$PROJECT_ROOT"
    
    # 1. 检查 go.mod 存在
    if [[ ! -f "go.mod" ]]; then
        log_error "go.mod 文件不存在"
        return 1
    fi
    
    # 2. go mod tidy 检查
    log_info "检查依赖完整性 (go mod tidy)..."
    if ! go mod tidy 2>&1; then
        log_warning "go mod tidy 有警告，请检查"
    fi
    
    # 3. go vet 检查
    log_info "静态代码分析 (go vet)..."
    if ! go vet ./... 2>&1; then
        log_warning "go vet 发现问题，请检查"
    fi
    
    # 4. 编译检查
    log_info "编译检查 (go build)..."
    if ! go build -o /dev/null ./cmd/node 2>&1; then
        log_error "编译失败"
        return 1
    fi
    
    log_success "发布前检查完成"
    return 0
}

# ───────────────────────────────────────────────────────────────────────────────
# 同步代码
# ───────────────────────────────────────────────────────────────────────────────

sync_code() {
    local dry_run=$1
    local clean=$2
    local pub_path
    pub_path=$(get_pub_path)
    local pub_dir="$WES_ROOT/$pub_path"
    
    log_info "同步代码到发布目录..."
    log_info "源目录: $PROJECT_ROOT"
    log_info "目标目录: $pub_dir"
    log_info "配置文件: $CONFIG_FILE"
    
    # 确保发布目录存在
    if [[ "$dry_run" != "true" ]]; then
        mkdir -p "$pub_dir"
    fi
    
    # 清理选项
    if [[ "$clean" == "true" ]]; then
        log_warning "将清理目标目录..."
        if [[ "$dry_run" != "true" ]]; then
            # 保留 .git 目录（如果存在）
            if [[ -d "$pub_dir/.git" ]]; then
                log_info "保留 .git 目录"
                mv "$pub_dir/.git" "$pub_dir/../.git_backup" 2>/dev/null || true
            fi
            rm -rf "$pub_dir"/*
            if [[ -d "$pub_dir/../.git_backup" ]]; then
                mv "$pub_dir/../.git_backup" "$pub_dir/.git"
            fi
        fi
    fi
    
    # 构建 rsync 命令
    local rsync_opts="-av --delete"
    if [[ "$dry_run" == "true" ]]; then
        rsync_opts="$rsync_opts --dry-run"
        log_info "[DRY-RUN] 以下是将要复制的文件："
    fi
    
    # 从配置文件读取排除规则
    local exclude_args
    exclude_args=$(get_exclude_args_from_config)
    
    # 执行 rsync
    cd "$PROJECT_ROOT"
    
    # shellcheck disable=SC2086
    echo "$exclude_args" | xargs rsync $rsync_opts "$PROJECT_ROOT/" "$pub_dir/"
    
    if [[ "$dry_run" != "true" ]]; then
        log_success "代码同步完成"
        
        # 显示同步统计
        log_info "发布目录内容："
        ls -la "$pub_dir/" | head -20
        
        # 计算文件数量
        local file_count
        file_count=$(find "$pub_dir" -type f 2>/dev/null | wc -l | tr -d ' ')
        log_info "共同步 $file_count 个文件"
    fi
}

# ───────────────────────────────────────────────────────────────────────────────
# 处理 go.mod（移除 replace 指令）
# ───────────────────────────────────────────────────────────────────────────────

process_go_mod() {
    local pub_path
    pub_path=$(get_pub_path)
    local pub_dir="$WES_ROOT/$pub_path"
    local go_mod="$pub_dir/go.mod"
    
    if [[ ! -f "$go_mod" ]]; then
        log_warning "go.mod 文件不存在于发布目录"
        return 0
    fi
    
    log_info "处理 go.mod（移除 replace 指令）..."
    
    # 创建临时文件
    local tmp_file
    tmp_file=$(mktemp)
    
    # 移除 replace 块
    awk '
        /^replace \(/ { in_replace=1; next }
        /^\)/ && in_replace { in_replace=0; next }
        in_replace { next }
        /^replace [^(]/ { next }
        { print }
    ' "$go_mod" > "$tmp_file"
    
    mv "$tmp_file" "$go_mod"
    
    log_success "go.mod 处理完成"
}

# ───────────────────────────────────────────────────────────────────────────────
# 创建同步记录
# ───────────────────────────────────────────────────────────────────────────────

create_sync_record() {
    local timestamp
    timestamp=$(date +"%Y-%m-%d-%H%M%S")
    local sync_dir="$PROJECT_ROOT/design/publishing/syncs/$timestamp"
    
    mkdir -p "$sync_dir"
    
    cat > "$sync_dir/sync.md" << EOF
# 代码同步记录

## 基本信息

- **同步时间**: $(date +"%Y-%m-%d %H:%M:%S")
- **同步类型**: 代码同步（不打版）
- **操作人员**: $(whoami)

## 同步原因

<!-- 请填写本次同步的原因 -->

## 变更摘要

<!-- 请填写主要变更内容 -->

## 影响范围

<!-- 请填写影响的功能模块 -->

## 备注

<!-- 其他需要说明的事项 -->
EOF

    log_info "同步记录已创建: $sync_dir/sync.md"
    log_info "请编辑同步记录文件，填写同步信息"
}

# ───────────────────────────────────────────────────────────────────────────────
# 版本发布
# ───────────────────────────────────────────────────────────────────────────────

release_version() {
    local version=$1
    local dry_run=$2
    
    log_info "准备发布版本 v$version..."
    
    # 检查版本目录是否存在
    local version_dir="$PROJECT_ROOT/design/publishing/versions/v$version"
    if [[ ! -d "$version_dir" ]]; then
        log_error "版本目录不存在: $version_dir"
        log_info "请先创建版本目录和 release.md 文档："
        log_info "  mkdir -p $version_dir"
        log_info "  cp design/publishing/templates/release-template.md $version_dir/release.md"
        return 1
    fi
    
    # 检查 release.md 是否存在
    if [[ ! -f "$version_dir/release.md" ]]; then
        log_error "release.md 不存在: $version_dir/release.md"
        return 1
    fi
    
    # 执行发布前检查
    if ! run_pre_checks; then
        log_error "发布前检查失败"
        return 1
    fi
    
    # 同步代码
    sync_code "$dry_run" "true"  # 版本发布时强制清理
    
    # 处理 go.mod
    if [[ "$dry_run" != "true" ]]; then
        process_go_mod
    fi
    
    local pub_path
    pub_path=$(get_pub_path)
    
    log_success "版本 v$version 发布准备完成"
    log_info "下一步："
    log_info "  1. cd $WES_ROOT/$pub_path"
    log_info "  2. git add ."
    log_info "  3. git commit -m 'Release v$version'"
    log_info "  4. git tag v$version"
    log_info "  5. git push origin main --tags"
}

# ───────────────────────────────────────────────────────────────────────────────
# 主函数
# ───────────────────────────────────────────────────────────────────────────────

main() {
    local action=""
    local version=""
    local dry_run="false"
    local clean="false"
    
    # 解析参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --sync)
                action="sync"
                shift
                ;;
            --version)
                action="version"
                version="$2"
                shift 2
                ;;
            --dry-run)
                dry_run="true"
                shift
                ;;
            --clean)
                clean="true"
                shift
                ;;
            --check)
                action="check"
                shift
                ;;
            --show-excludes)
                action="show-excludes"
                shift
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                log_error "未知选项: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    # 检查配置文件
    if [[ ! -f "$CONFIG_FILE" ]]; then
        log_warning "配置文件不存在: $CONFIG_FILE"
        log_info "将使用默认排除规则"
    fi
    
    # 执行操作
    case $action in
        sync)
            log_info "执行代码同步..."
            sync_code "$dry_run" "$clean"
            if [[ "$dry_run" != "true" ]]; then
                process_go_mod
                create_sync_record
            fi
            ;;
        version)
            if [[ -z "$version" ]]; then
                log_error "请指定版本号"
                exit 1
            fi
            release_version "$version" "$dry_run"
            ;;
        check)
            run_pre_checks
            ;;
        show-excludes)
            show_excludes
            ;;
        *)
            log_error "请指定操作: --sync, --version <ver>, --check, 或 --show-excludes"
            show_help
            exit 1
            ;;
    esac
}

main "$@"
