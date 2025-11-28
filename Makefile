# WES 项目 Makefile
# 支持环境专用二进制文件构建

.PHONY: help build-all build-dev build-test build-prod build-legacy clean clean-all clean-data clean-data-preview clean-data-force test lint lint-fix install-deps install-lint-tools run-dev run-test run-prod

# 默认目标
.DEFAULT_GOAL := help

# 构建变量
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GO_VERSION := $(shell go version | awk '{print $$3}')

# LDFLAGS for embedding build information
BASE_LDFLAGS := -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT) -X main.GoVersion=$(GO_VERSION)

##@ Help

help: ## 显示帮助信息
	@echo "🔧 WES 项目构建工具"
	@echo ""
	@echo "📋 可用命令:"
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Environment-Specific Build (环境专用构建)

build-all: ## 构建所有环境的二进制文件
	@echo "🏗️  构建所有环境二进制文件..."
	@./scripts/build/build-all-envs.sh

build-dev: ## 仅构建开发环境二进制文件
	@echo "🔧 构建开发环境二进制文件..."
	@bash scripts/build/ensure_onnx_libs.sh
	@mkdir -p bin
	@go build -ldflags "$(BASE_LDFLAGS)" -o bin/weisyn-development ./cmd/weisyn
	@chmod +x bin/weisyn-development
	@echo "✅ bin/weisyn-development 构建完成（使用 --env development 指定环境）"

build-test: ## 仅构建测试环境二进制文件
	@echo "🧪 构建测试环境二进制文件..."
	@bash scripts/build/ensure_onnx_libs.sh
	@mkdir -p bin
	@go build -ldflags "$(BASE_LDFLAGS)" -o bin/weisyn-testing ./cmd/weisyn
	@chmod +x bin/weisyn-testing
	@echo "✅ bin/weisyn-testing 构建完成（使用 --env testing 指定环境）"

build-prod: ## 仅构建生产环境二进制文件
	@echo "🚀 构建生产环境二进制文件..."
	@bash scripts/build/ensure_onnx_libs.sh
	@mkdir -p bin
	@go build -ldflags "$(BASE_LDFLAGS)" -o bin/weisyn-production ./cmd/weisyn
	@chmod +x bin/weisyn-production
	@echo "✅ bin/weisyn-production 构建完成（使用 --env production 指定环境）"


##@ Development (开发相关)

run-dev: build-dev ## 构建并运行开发环境
	@echo "🔧 启动开发环境..."
	@./bin/weisyn-development --env development

run-test: build-test ## 构建并运行测试环境
	@echo "🧪 启动测试环境..."
	@./bin/weisyn-testing --env testing --daemon

run-prod: build-prod ## 构建并运行生产环境（仅API模式）
	@echo "🚀 启动生产环境（API模式）..."
	@./bin/weisyn-production --env production --daemon

##@ Quality Assurance (质量保证)

test: ## 运行测试
	@echo "🧪 运行测试套件..."
	@go test ./...

test-verbose: ## 运行详细测试
	@echo "🧪 运行详细测试套件..."
	@go test -v ./...

lint: ## 运行代码检查（使用golangci-lint，如果未安装则使用go vet/fmt）
	@echo "🔍 运行代码检查..."
	@GOLANGCI_LINT=""; \
	if [ -f "./bin/golangci-lint" ]; then \
		GOLANGCI_LINT="./bin/golangci-lint"; \
	elif command -v golangci-lint >/dev/null 2>&1; then \
		GOLANGCI_LINT="golangci-lint"; \
	fi; \
	if [ -n "$$GOLANGCI_LINT" ]; then \
		echo "✅ 使用 golangci-lint 进行代码检查..."; \
		if [ -d "/tmp/go-x86_64/go" ] && [ "$$(uname -m)" = "x86_64" ]; then \
			export PATH="/tmp/go-x86_64/go/bin:$$PATH"; \
			export GOROOT="/tmp/go-x86_64/go"; \
		fi; \
		$$GOLANGCI_LINT run; \
	else \
		echo "⚠️  golangci-lint 未安装，使用 go vet/fmt 进行基础检查"; \
		echo "💡 运行 'make install-lint-tools' 安装 golangci-lint 以获得更全面的检查"; \
		go vet ./...; \
		go fmt -l .; \
	fi

lint-fix: ## 自动修复代码问题（仅golangci-lint）
	@echo "🔧 自动修复代码问题..."
	@GOLANGCI_LINT=""; \
	if [ -f "./bin/golangci-lint" ]; then \
		GOLANGCI_LINT="./bin/golangci-lint"; \
	elif command -v golangci-lint >/dev/null 2>&1; then \
		GOLANGCI_LINT="golangci-lint"; \
	fi; \
	if [ -n "$$GOLANGCI_LINT" ]; then \
		$$GOLANGCI_LINT run --fix; \
	else \
		echo "❌ golangci-lint 未安装，请先运行 'make install-lint-tools'"; \
		exit 1; \
	fi

lint-check: ## 运行检查并生成修复友好的报告（一次性，包含代码上下文）
	@echo "🔍 运行代码质量检查并生成报告..."
	@./scripts/lint/check-and-report.sh

lint-extract: lint-check ## 提取所有 lint 问题列表（已废弃，使用 lint-check）
	@echo "⚠️  已废弃，请使用: make lint-check"
	@./scripts/lint/check-and-report.sh

lint-update: ## 增量更新报告（只检查修改的文件）
	@./scripts/lint/update-report.sh

lint-stats: ## 显示 lint 问题统计报告
	@if [ ! -f ".lint-report.json" ]; then \
		echo "❌ 报告不存在，请先运行: make lint-check"; \
		exit 1; \
	fi
	@./scripts/lint/stats-issues.sh

lint-query: ## 查询 lint 问题（用法: make lint-query LINTER=errcheck 或 make lint-query FILE=internal/core）
	@if [ ! -f ".lint-report.json" ]; then \
		echo "❌ 报告不存在，请先运行: make lint-check"; \
		exit 1; \
	fi
	@if [ -n "$(LINTER)" ]; then \
		./scripts/lint/query-issues.sh -l "$(LINTER)"; \
	elif [ -n "$(FILE)" ]; then \
		./scripts/lint/query-issues.sh -f "$(FILE)"; \
	else \
		./scripts/lint/query-issues.sh -s; \
	fi

lint-verify: ## 验证修复情况（用法: make lint-verify FILE=path/to/file.go）
	@if [ -n "$$FILE" ]; then \
		./scripts/lint/verify-fix.sh -f $$FILE; \
	else \
		./scripts/lint/verify-fix.sh -a; \
	fi

##@ Dependencies (依赖管理)

install-deps: ## 安装依赖
	@echo "📦 安装Go依赖..."
	@go mod download
	@go mod tidy

install-lint-tools: ## 安装代码检查工具（golangci-lint）
	@echo "📦 安装 golangci-lint..."
	@if [ -f "./bin/golangci-lint" ]; then \
		echo "✅ golangci-lint 已安装在项目 bin 目录: $$(./bin/golangci-lint --version)"; \
	elif command -v golangci-lint >/dev/null 2>&1; then \
		echo "✅ golangci-lint 已安装: $$(golangci-lint --version)"; \
	else \
		echo "正在下载并安装 golangci-lint 到项目 bin 目录..."; \
		mkdir -p bin; \
		VERSION=$$(curl -s https://api.github.com/repos/golangci/golangci-lint/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'); \
		echo "下载版本: $$VERSION"; \
		ARCH=$$(uname -m); \
		if [ "$$ARCH" = "x86_64" ] || [ "$$ARCH" = "amd64" ]; then \
			ARCH="amd64"; \
		elif [ "$$ARCH" = "arm64" ] || [ "$$ARCH" = "aarch64" ]; then \
			ARCH="arm64"; \
		fi; \
		OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
		curl -sSfL "https://github.com/golangci/golangci-lint/releases/download/$$VERSION/golangci-lint-$${VERSION#v}-$$OS-$$ARCH.tar.gz" -o /tmp/golangci-lint.tar.gz; \
		tar -xzf /tmp/golangci-lint.tar.gz -C /tmp/; \
		cp /tmp/golangci-lint-*/golangci-lint bin/; \
		chmod +x bin/golangci-lint; \
		rm -rf /tmp/golangci-lint*; \
		echo "✅ golangci-lint 安装完成: $$(./bin/golangci-lint --version)"; \
	fi

deps-update: ## 更新依赖
	@echo "🔄 更新Go依赖..."
	@go get -u ./...
	@go mod tidy

##@ Utilities (工具命令)

clean: ## 清理构建产物
	@echo "🧹 清理构建产物..."
	@rm -f bin/weisyn-development bin/weisyn-testing bin/weisyn-production bin/development bin/testing bin/production
	@echo "✅ 清理完成"

clean-all: clean ## 清理所有生成文件
	@echo "🧹 清理所有生成文件..."
	@rm -rf bin/
	@go clean -cache -testcache -modcache
	@echo "✅ 深度清理完成"

clean-data: ## 清理区块链数据（交互式）
	@echo "🗑️  清理区块链数据..."
	@go run ./cmd/cleanup

clean-data-preview: ## 预览要删除的数据文件
	@echo "🔍 预览数据清理..."
	@go run ./cmd/cleanup --dry-run

clean-data-force: ## 强制清理数据（无确认）
	@echo "⚠️ 强制清理区块链数据..."
	@go run ./cmd/cleanup --yes

version: ## 显示版本信息
	@echo "WES 构建信息:"
	@echo "  构建时间: $(BUILD_TIME)"
	@echo "  Git提交: $(GIT_COMMIT)"
	@echo "  Go版本: $(GO_VERSION)"

check-env: ## 检查构建环境
	@echo "🔍 检查构建环境..."
	@echo "Go版本: $(shell go version)"
	@echo "Git版本: $(shell git --version 2>/dev/null || echo 'Git未安装')"
	@echo "项目根目录: $(shell pwd)"
	@echo "配置文件:"
	@ls -la configs/*/config.json 2>/dev/null || echo "  配置文件未找到"

##@ Installation (安装)

install-all: build-all ## 构建并安装所有二进制文件到系统路径
	@echo "📦 安装二进制文件到系统路径..."
	@sudo cp bin/weisyn-development /usr/local/bin/weisyn-development 2>/dev/null || true
	@sudo cp bin/weisyn-testing /usr/local/bin/weisyn-testing 2>/dev/null || true
	@sudo cp bin/weisyn-production /usr/local/bin/weisyn-production 2>/dev/null || true
	@echo "✅ 安装完成"
	@echo "   系统命令: weisyn-development, weisyn-testing, weisyn-production"

uninstall: ## 卸载系统中的二进制文件
	@echo "🗑️  卸载系统二进制文件..."
	@sudo rm -f /usr/local/bin/weisyn-development /usr/local/bin/weisyn-testing /usr/local/bin/weisyn-production
	@echo "✅ 卸载完成"

##@ Docker (容器化)

docker-build: ## 构建Docker镜像
	@echo "🐳 构建Docker镜像..."
	@docker build -t weisyn:latest .

docker-run: docker-build ## 运行Docker容器
	@echo "🐳 运行Docker容器..."
	@docker run -p 8080:8080 weisyn:latest

##@ Examples (示例)

example-dev: ## 运行开发环境示例
	@echo "📚 开发环境使用示例:"
	@echo "  完整功能: ./bin/development"
	@echo "  仅API:   ./bin/development --api-only"
	@echo "  仅CLI:   ./bin/development --cli-only"

example-prod: ## 显示生产环境示例
	@echo "📚 生产环境使用示例:"
	@echo "  推荐方式: ./bin/production --api-only"
	@echo "  完整功能: ./bin/production"
	@echo "  调试模式: ./bin/production --cli-only"
