# weisyn-node - WES 区块链节点权威手册

> **定位**：`weisyn-node` 的**官方说明书**，面向运维 / DevOps / SRE / 生产环境，包含所有启动模式、命令行参数、环境策略和生产部署流程。

---

## 📋 目录

- [快速上手（开发/测试）](#快速上手开发测试)
- [启动模式 & 链模式说明](#启动模式--链模式说明)
- [节点级参数总表（flag 参考手册）](#节点级参数总表flag-参考手册)
- [环境与角色推荐（dev/test/prod）](#环境与角色推荐devtestprod)
- [生产打包与部署](#生产打包与部署)
- [与日志/诊断相关的参数](#与日志诊断相关的参数)
- [子命令](#子命令)
- [常见问题](#常见问题)

---

## 🚀 快速上手（开发/测试）

### 方式一：使用 go run（推荐用于开发验证）

**适用场景**：开发、测试、快速验证代码修改。无需编译，直接运行源代码。

```bash
# 本地私链开发（单节点，自动挖矿）
go run ./cmd/node --chain private --config ./configs/chains/dev-private-local.json

# 本地公链开发（单节点，自动挖矿）
go run ./cmd/node --chain public --config ./configs/chains/dev-public-local.json

# 连接公共测试网（使用内嵌配置）
go run ./cmd/node --chain public
```

### 方式二：先编译再运行（推荐用于生产环境）

**适用场景**：正式使用、生产部署、需要重复运行。

```bash
# 1. 编译
make build-node

# 2. 运行
./bin/weisyn-node --chain private --config ./configs/chains/dev-private-local.json
./bin/weisyn-node --chain public --config ./configs/chains/dev-public-local.json
./bin/weisyn-node --chain public  # 公共测试网
```

---

## 🔧 启动模式 & 链模式说明

### `--chain` 参数说明

`weisyn-node` 支持三种链模式，通过 `--chain` 参数指定：

| `--chain` 值 | 链模式 | 是否需要 `--config` | 配置来源 | 说明 |
|-------------|--------|-------------------|---------|------|
| `public` | 公链 | 可选 | 无 `--config`：内嵌 `test-public-demo.json`<br>有 `--config`：用户配置文件 | 公共测试网或自建公链 |
| `mainnet` | 公链（主网别名） | 否 | 内嵌 `test-public-demo.json` | 当前等同于 `--chain public` |
| `consortium` | 联盟链 | **必须** | 用户配置文件 | 多机构共同背书 |
| `private` | 私有链 | **必须** | 用户配置文件 | 单机构/内网账本 |

### 三种启动模式的区别

| 特性 | 官方测试网 | 自建公链 | 联盟链/私链 |
|------|-----------|---------|------------|
| 启动方式 | `--chain public`（无 `--config`） | `--chain public --config <path>` | `--chain consortium/private --config <path>` |
| 配置来源 | 内嵌配置（编译时嵌入） | 用户配置文件 | 用户配置文件 |
| chain_id | 固定为 12001（test-public-demo） | 用户自定义（1000-9999） | 用户自定义（联盟链：20000-29999，私链：10000-19999） |
| 配置修改 | 不允许修改链级配置 | 可以修改链级配置 | 可以修改链级配置 |
| 创建方式 | 无需创建，直接使用 | 通过 BaaS 或手工创建 | 使用 `chain init` 生成模板 |

> 💡 **详细链模式设计**：见 [configs/chains/README.md](../../configs/chains/README.md)

### 内嵌配置说明（`--chain public` 无 `--config`）

当使用 `--chain public` 且不提供 `--config` 时，节点使用**内嵌配置**：

- **内嵌机制**：编译/运行时通过 Go 的 `//go:embed` 指令，把 `configs/chains/test-public-demo.json` 内容嵌入到二进制中（见 `configs/embed.go`）
- **运行时行为**：直接从内存读取，不再从磁盘打开 JSON 文件
- **优势**：二进制自包含，分发时无需携带配置文件
- **节点能力**：节点能力（挖矿、共识投票等）由运行时状态机控制，通过 API 进行管理

---

## 📝 节点级参数总表（flag 参考手册）

### 链选择 / 配置来源

| 参数 | 类型 | 必需 | 说明 | 节点级覆盖 |
|-----|------|------|------|-----------|
| `--chain <mode>` | string | ✅ | 链模式：`public` \| `mainnet` \| `consortium` \| `private` | ❌ |
| `--config <path>` | string | 条件 | 配置文件路径<br>- `public`：可选（无则用内嵌配置）<br>- `consortium/private`：必需 | ❌ |

### 端口相关（节点级覆盖）

| 参数 | 类型 | 默认值 | 说明 | 覆盖的配置字段 |
|-----|------|--------|------|---------------|
| `--http-port <port>` | int | 0（不覆盖） | HTTP API 端口（REST/JSON-RPC/WebSocket） | `api.http_port` |
| `--grpc-port <port>` | int | 0（不覆盖） | gRPC API 端口 | `api.grpc_port` |
| `--diagnostics-port <port>` | int | 0（不覆盖） | 诊断/pprof HTTP 端口 | `node.host.diagnostics_port` |

**使用示例**：

```bash
# 覆盖单个端口
./bin/weisyn-node --chain public --http-port 28700

# 同时覆盖多个端口（适配本机环境）
./bin/weisyn-node --chain public \
  --http-port 28700 \
  --grpc-port 28702 \
  --diagnostics-port 28706
```

> **说明**：
> - 所有端口参数都是**节点级覆盖**，只影响当前设备，不会改变链级配置（chain_id、genesis 等）
> - JSON 配置文件中的端口值作为**默认值**，命令行参数优先级更高
> - 适用于：端口冲突、多节点部署在同一台机器、不同环境使用不同端口等场景

### 存储目录（节点级覆盖）

| 参数 | 类型 | 默认值 | 说明 | 覆盖的配置字段 |
|-----|------|--------|------|---------------|
| `--data-dir <path>` | string | ""（不覆盖） | 数据根目录 | `storage.data_root` |

**使用示例**：

```bash
./bin/weisyn-node --chain public --data-dir /custom/data/path
```

### 全局开关

| 参数 | 类型 | 说明 |
|-----|------|------|
| `--help` | bool | 显示帮助信息 |
| `--version` | bool | 显示版本信息 |

---

## 🌍 环境与角色推荐（dev/test/prod）

### dev 环境（本地开发）

**推荐配置**：

- **配置文件**：`dev-private-local.json` 或 `dev-public-local.json`
- **同步模式**：`from_genesis`（从创世块启动）
- **诊断端口**：默认启用（`diagnostics_enabled=true`，`diagnostics_port: 28686`）
- **节点能力**：挖矿和共识投票通过运行时 API 控制

**启动示例**：

```bash
# 本地私链开发
./bin/weisyn-node --chain private --config ./configs/chains/dev-private-local.json

# 本地公链开发
./bin/weisyn-node --chain public --config ./configs/chains/dev-public-local.json
```

### test 环境（测试网 / 联盟测试）

**推荐配置**：

- **配置文件**：`test-public-demo.json`（内嵌）或 `test-consortium-demo.json`
- **同步模式**：`from_network`（从网络同步）
- **检查点**：共识节点建议配置 `require_trusted_checkpoint=true` + `trusted_checkpoint`
- **节点能力**：挖矿和共识投票通过运行时 API 控制

**启动示例**：

```bash
# 公共测试网
./bin/weisyn-node --chain public

# 联盟链测试
./bin/weisyn-node --chain consortium --config ./configs/chains/test-consortium-demo.json
```

### prod 环境（生产部署）

**推荐配置**：

- **配置文件**：生产环境专用配置（通过 BaaS 或 `chain init` 生成）
- **同步模式**：`from_network`（从网络同步）
- **检查点**：共识节点**必须**配置 `require_trusted_checkpoint=true` + `trusted_checkpoint`
- **诊断端口**：默认禁用（`diagnostics_enabled=false`），仅运维需要时通过配置文件临时启用
- **节点能力**：挖矿和共识投票通过运行时 API 控制
- **安全建议**：
  - 通过防火墙/安全组限制诊断端口访问
  - 使用 systemd / Supervisor / Docker / K8s 管理进程
  - 配置日志轮转和监控告警

**启动示例**：

```bash
# 生产环境
./bin/weisyn-node --chain public --config ./prod-public-config.json
```

> 💡 **详细策略矩阵**：见 [configs/chains/README.md](../../configs/chains/README.md) → "节点角色与同步策略推荐"

---

## 🏭 生产打包与部署

### 构建生产二进制

#### 方式一：使用 Makefile（推荐）

```bash
# 构建节点二进制
make build-node

# 输出：bin/weisyn-node
```

#### 方式二：使用 go build

```bash
# 基础构建
go build -o bin/weisyn-node ./cmd/node

# 交叉编译（Linux）
GOOS=linux GOARCH=amd64 go build -o bin/weisyn-node-linux-amd64 ./cmd/node

# 交叉编译（macOS）
GOOS=darwin GOARCH=amd64 go build -o bin/weisyn-node-darwin-amd64 ./cmd/node

# 交叉编译（Windows）
GOOS=windows GOARCH=amd64 go build -o bin/weisyn-node-windows-amd64.exe ./cmd/node
```

#### 方式三：CI/CD 构建

```yaml
# 示例：GitHub Actions
- name: Build Node Binary
  run: |
    go build -o bin/weisyn-node \
      -ldflags "-X main.version=${{ github.ref_name }}" \
      ./cmd/node
```

### 推荐的运行方式

#### 1. systemd（Linux 生产环境推荐）

创建 systemd 服务文件 `/etc/systemd/system/weisyn-node.service`：

```ini
[Unit]
Description=WES Blockchain Node
After=network.target

[Service]
Type=simple
User=wes
Group=wes
WorkingDirectory=/opt/wes
ExecStart=/opt/wes/bin/weisyn-node --chain public --data-dir /data/wes/node
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

# 安全限制
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/data/wes

[Install]
WantedBy=multi-user.target
```

**管理命令**：

```bash
# 启动服务
sudo systemctl start weisyn-node

# 停止服务
sudo systemctl stop weisyn-node

# 查看状态
sudo systemctl status weisyn-node

# 查看日志
sudo journalctl -u weisyn-node -f

# 开机自启
sudo systemctl enable weisyn-node
```

#### 2. Supervisor（进程管理）

创建 Supervisor 配置文件 `/etc/supervisor/conf.d/weisyn-node.conf`：

```ini
[program:weisyn-node]
command=/opt/wes/bin/weisyn-node --chain public --data-dir /data/wes/node
directory=/opt/wes
user=wes
autostart=true
autorestart=true
stderr_logfile=/var/log/weisyn-node/error.log
stdout_logfile=/var/log/weisyn-node/output.log
```

**管理命令**：

```bash
# 启动
sudo supervisorctl start weisyn-node

# 停止
sudo supervisorctl stop weisyn-node

# 查看状态
sudo supervisorctl status weisyn-node
```

#### 3. Docker

创建 `Dockerfile`：

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -o /build/bin/weisyn-node ./cmd/node

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/bin/weisyn-node /app/weisyn-node
COPY --from=builder /build/configs/chains/test-public-demo.json /app/configs/chains/test-public-demo.json
EXPOSE 28680 28682 28683
ENTRYPOINT ["/app/weisyn-node"]
CMD ["--chain", "public", "--data-dir", "/data"]
```

**构建和运行**：

```bash
# 构建镜像
docker build -t weisyn-node:latest .

# 运行容器
docker run -d \
  --name weisyn-node \
  -p 28680:28680 \
  -p 28682:28682 \
  -p 28683:28683 \
  -v /data/wes:/data \
  weisyn-node:latest \
  --chain public --data-dir /data
```

#### 4. Kubernetes

创建 Deployment 配置 `k8s/node-deployment.yaml`：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: weisyn-node
spec:
  replicas: 1
  selector:
    matchLabels:
      app: weisyn-node
  template:
    metadata:
      labels:
        app: weisyn-node
    spec:
      containers:
      - name: weisyn-node
        image: weisyn-node:latest
        args:
        - --chain
        - public
        - --data-dir
        - /data
        ports:
        - containerPort: 28680
          name: http
        - containerPort: 28682
          name: grpc
        - containerPort: 28683
          name: p2p
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: weisyn-node-data
```

**部署命令**：

```bash
kubectl apply -f k8s/node-deployment.yaml
kubectl get pods -l app=weisyn-node
```

### 滚动升级与平滑重启

**推荐策略**：

1. **滚动升级**（多节点部署）：
   - 逐个节点停止 → 更新二进制 → 重启
   - 确保至少 2/3 节点在线（共识要求）

2. **平滑重启**（单节点）：
   - 发送 `SIGTERM` 信号，等待节点优雅关闭
   - 节点会完成当前区块处理后再退出

**示例脚本**：

```bash
#!/bin/bash
# 平滑重启节点

# 发送 SIGTERM（优雅关闭）
kill -TERM $(pgrep -f weisyn-node)

# 等待进程退出（最多 60 秒）
timeout=60
while [ $timeout -gt 0 ] && pgrep -f weisyn-node > /dev/null; do
  sleep 1
  timeout=$((timeout - 1))
done

# 如果还在运行，强制杀死
if pgrep -f weisyn-node > /dev/null; then
  kill -9 $(pgrep -f weisyn-node)
fi

# 启动新版本
/opt/wes/bin/weisyn-node --chain public --data-dir /data/wes/node
```

> 💡 **详细运维指南**：见 `_dev/06-开发运维指南-guides/04-运行与运维-operations-and-runtime/` 目录下的相关文档

---

## 📊 与日志/诊断相关的参数

### 环境变量

| 环境变量 | 说明 | 使用示例 |
|---------|------|---------|
| `WES_CLI_MODE=true` | 关闭控制台日志输出，只写入文件 | `export WES_CLI_MODE=true`<br>`./bin/weisyn-node --chain public` |

**使用方式**：

```bash
export WES_CLI_MODE=true
./bin/weisyn-node --chain public
```

设置后，所有日志只写入文件，不再在终端刷屏。日志文件位置：`{data_dir}/{env}/{instance}/logs/node-system.log`

### 配置文件中的诊断设置

在链配置文件的 `node.host` 部分可以启用诊断端口：

```json
{
  "node": {
    "host": {
      "diagnostics_enabled": true,
      "diagnostics_port": 28686
    }
  }
}
```

**诊断端口说明**：

- `diagnostics_enabled`：是否启用诊断 HTTP 服务（pprof / P2P diagnostics）
- `diagnostics_port`：诊断 HTTP 端口（默认 28686，可通过 `--diagnostics-port` 覆盖）
- **安全建议**：生产环境默认禁用，仅运维需要时临时启用，并通过防火墙限制访问

**访问诊断端点**：

```bash
# 浏览器访问
http://127.0.0.1:28686/debug/pprof/

# 命令行访问
go tool pprof http://127.0.0.1:28686/debug/pprof/heap
go tool pprof http://127.0.0.1:28686/debug/pprof/goroutine
```

> 💡 **详细诊断指南**：见 `_dev/06-开发运维指南-guides/04-运行与运维-operations-and-runtime/03-NODE_DIAGNOSTICS_PRACTICAL_GUIDE.md`

---

## 🔨 子命令

### chain init

生成联盟链/私链配置文件模板。

**使用 go run**：

```bash
# 交互式（默认）
go run ./cmd/node chain init --mode consortium --out ./my-consortium.json

# 非交互式（CI/CD 场景）
go run ./cmd/node chain init --mode consortium --out ./my-consortium.json --force
```

**使用编译后的二进制**：

```bash
# 交互式（默认）
./bin/weisyn-node chain init --mode consortium --out ./my-consortium.json

# 非交互式（CI/CD 场景）
./bin/weisyn-node chain init --mode consortium --out ./my-consortium.json --force
```

**选项说明**：

- `--mode`：链模式（`consortium` 或 `private`）
- `--out`：输出文件路径（必需）
- `--force` / `--yes`：强制覆盖已存在的文件，跳过交互确认（用于 CI/CD）

> 💡 **生成后必须修改的字段**：`chain_id`、`network_id`、`network_namespace`、`genesis.accounts` 等。  
> 详细字段说明请见 [configs/chains/README.md](../../configs/chains/README.md)。

---

## ❓ 常见问题

### Q: 使用 go run 还是编译后运行？

**A:** 
- **开发验证**：使用 `go run ./cmd/node`，无需编译，修改代码后立即生效
- **生产环境**：先编译（`make build-node`），然后运行 `./bin/weisyn-node`

### Q: 命令在哪里执行？

**A:** 在**终端/命令行**中执行。打开终端，进入项目根目录，然后执行命令。

### Q: 如何知道节点启动成功？

**A:** 看到类似以下输出说明启动成功：

```
🚀 正在启动 weisyn-node
   链模式: public
   运行环境: test
   配置来源: 内嵌公链配置（公共测试网 test-public-demo）
...
```

### Q: 为什么访问 `http://localhost:28680/` 是 `404 page not found`？

**A:** 这是**预期行为**，不是节点没启动，也不是 API 出问题：

- 当前 HTTP 服务器只注册了以下端点：
  - `POST /jsonrpc`（主 JSON‑RPC 协议）及兼容别名 `POST /rpc`
  - `GET /ws`（WebSocket 实时订阅）
  - `GET /api/v1/health/*`（健康检查，如 `live` / `ready`）
  - `GET /api/v1/spv/*`（SPV 轻客户端）
  - `GET /api/v1/txpool/*`（交易池查询）
  - `GET /api/v1/system/memory`（内存监控，取决于是否启用）
- 根路径 `/` 没有注册任何 handler，所以返回 404 是正常的。

**正确的检查方式**：

```bash
# 存活探针（liveness）
curl http://localhost:28680/api/v1/health/live

# 就绪探针（readiness）
curl http://localhost:28680/api/v1/health/ready
```

### Q: 为什么终端会疯狂刷新日志？如何让日志只写入文件？

**A:** 默认情况下，节点会**同时输出到控制台和文件**。

**方式一：使用环境变量关闭控制台输出（推荐）**

```bash
export WES_CLI_MODE=true
./bin/weisyn-node --chain public
```

设置后，所有日志只写入文件，不再在终端刷屏。

**方式二：在配置文件中关闭控制台输出**

如果你使用联盟链/私链模式，可以在配置文件的 `log` 部分设置：

```json
{
  "log": {
    "level": "info",
    "to_console": false
  }
}
```

**查看日志文件**：

```bash
# 日志目录遵循：{data_root}/{env}/{instance_slug}/logs/
# 其中 instance_slug 默认按规则生成：{env}-{chain_mode}-{network.network_name}
#
# 例如：公共测试网（内嵌 test-public-demo）当前默认 network_name= WES_public_testnet_demo_2024，
# 所以日志目录通常是：
#   ./data/test/test-public-WES_public_testnet_demo_2024/logs/
#
# 不确定具体目录时，建议先快速定位：
find ./data -name node-system.log

# 实时查看系统日志（示例）
tail -f ./data/test/test-public-WES_public_testnet_demo_2024/logs/node-system.log

# 实时查看业务日志（示例）
tail -f ./data/test/test-public-WES_public_testnet_demo_2024/logs/node-business.log
```

### Q: 公链模式可以指定配置文件吗？

**A:** 
- **官方测试网**：不可以。使用 `--chain public`（无 `--config`）时，使用内嵌的 `test-public-demo` 配置。
- **自建公链**：可以。使用 `--chain public --config <path>` 时，必须提供配置文件，且 `chain_mode` 必须为 `public`。

### Q: 如何修改运行环境（environment）？

**A:** `environment` 字段必须在配置文件中定义，不能通过命令行参数修改。编辑配置文件中的 `environment` 字段（`dev` | `test` | `prod`）。

### Q: 节点级配置会改变链级配置吗？

**A:** 不会。`--http-port`、`--grpc-port`、`--diagnostics-port`、`--data-dir` 等节点级参数只影响本地节点，不会改变链 ID、genesis、network_namespace 等链级配置。

### Q: 配置文件中的端口被占用了怎么办？

**A:** 使用节点级端口覆盖参数：

```bash
./bin/weisyn-node --chain public --http-port 28700 --grpc-port 28702 --diagnostics-port 28706
```

> **详细说明**：见上方"节点级参数总表" → "端口相关"小节

### Q: 单节点矿工场景下，为什么节点一直显示"系统正在同步中，无法开始挖矿"？

**A:** 这是**单节点矿工 / 首块出块场景**的特殊情况。

**问题现象**：
- 节点状态显示 `Bootstrapping`
- `localHeight=0`，`networkHeight=0`
- 挖矿一直无法开始，日志显示"系统正在同步中，无法开始挖矿"

**原因分析**：
- 在单节点矿工场景下，节点是这条链的**第一个出块节点**
- 不存在上游网络可同步，同步服务也不可能凭空把 `networkHeight` 变成 >0
- 原有的同步检查逻辑假设"一定存在一个上游高度 > 0 的网络可以追"，导致单节点场景被错误拦截

**解决方案**：
- 系统已自动识别并处理此场景：当检测到 `Bootstrapping + localHeight=0 + networkHeight=0` 时，会视为"首个矿工节点"，允许直接开始挖矿
- 节点会自动输出日志：`检测到单节点 Bootstrapping 场景（localHeight=0, networkHeight=0），视为首个矿工节点，允许开始挖矿`
- 后续的高度门闸检查会正确处理首块出块逻辑（链高度=0、lastProcessed=0 允许开始挖）

**适用场景**：
- 本地开发环境启动单节点私链/公链进行挖矿测试
- 新链的初始矿工节点
- 独立运行的测试节点

**多节点场景**：
- 如果存在其他节点（`networkHeight > 0`），则仍按原有逻辑：先同步到网络高度，再开始挖矿

---

## 📖 相关文档

- **[cmd/README.md](../README.md)** - cmd/ 目录总览（任务导航、快速上手）
- **[configs/chains/README.md](../../configs/chains/README.md)** - 配置选型与字段规范
- **[运行环境设计](../../_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/03-ENVIRONMENT_DESIGN.md)** - 运行环境设计文档
- **[链模式设计](../../_dev/02-架构设计-architecture/12-运行与部署架构-runtime-and-deployment/04-CHAINMODE_DESIGN.md)** - 链模式设计文档
- **[节点诊断实战指南](../../_dev/06-开发运维指南-guides/04-运行与运维-operations-and-runtime/03-NODE_DIAGNOSTICS_PRACTICAL_GUIDE.md)** - 诊断与排障完整指南
