# APIRelay

APIRelay 是一个轻量级 API 中转与管理服务，支持 OpenAI / Anthropic / Gemini 等协议入口的统一接入、协议转换、全局渠道路由、故障转移与熔断控制。

## 特性

- **多渠道管理**：统一管理 OpenAI 兼容、Anthropic、Gemini 等上游渠道。
- **全局代理架构**：所有协议入口共享同一套代理配置、故障转移队列和熔断器状态，不使用 app 隔离。
- **全局故障转移**：按全局队列顺序选择渠道；关闭自动故障转移时仅选择最高优先级渠道。
- **全局熔断器**：熔断 key 只使用 `channel_id`，不包含 app/protocol 维度，可在管理台手动重置。
- **协议转换**：支持 OpenAI、Anthropic、Gemini 请求/响应格式转换。
- **流式响应**：支持 SSE 流式透传/转换；流式首包失败可切换下一个渠道，开始输出后不再切换。
- **响应预检**：非流式响应完整读取后再标记成功；流式响应首包预读成功后再开始写客户端。
- **可视化管理**：Web 管理台支持渠道、模型、请求日志和全局代理配置管理。
- **请求日志**：记录请求渠道、模型、协议、状态码、延迟与错误信息。
- **前后端一体**：可通过单文件二进制提供后端与管理台。

## 快速开始

### 下载并运行

1. 从 [Releases](https://github.com/TheFloodDragon/APIRelay/releases) 下载对应平台的二进制文件。

2. 首次启动会自动生成默认配置文件：

   ```bash
   ./apirelay
   ```

3. 修改配置文件中的管理密钥：

   ```bash
   vim config.yml  # 修改 auth.admin_key
   ```

4. 重启服务：

   ```bash
   ./apirelay
   ```

5. 访问管理后台：

   ```text
   http://localhost:15722
   ```

### 使用 Docker

```bash
docker run -d \
  -p 15722:15722 \
  -v $(pwd)/config.yml:/app/config.yml \
  -v $(pwd)/data:/app/data \
  --name apirelay \
  theflooddragon/apirelay:latest
```

## 使用指南

### 1. 添加渠道

打开管理后台，进入“渠道管理”页面：

1. 点击“添加渠道”。
2. 选择协议类型（OpenAI 兼容 / Anthropic / Gemini）。
3. 填写渠道名称、API Key、Base URL。
4. 填写或获取上游真实模型列表。
5. 设置优先级、权重、超时、重试次数。
6. 保存。

### 2. 配置全局代理

进入管理后台 `/proxy` 页面，可以配置所有协议入口共享的全局代理策略：

- 代理开关。
- 自动故障转移开关。
- 最大重试次数。
- 非流式超时。
- 流式首包超时。
- 流式静默超时。
- 熔断失败阈值。
- 熔断恢复阈值。
- 熔断打开时间。
- 全局故障转移队列拖拽排序。
- 每个渠道的熔断状态查看与手动重置。

说明：`/proxy` 页面只展示全局配置，不显示 app 类型选择或 app tab。故障转移队列和熔断器均以 `channel_id` 为唯一维度。

### 3. 兼容 API 调用

兼容 API 可以使用管理密钥调试，也可以使用“API 密钥管理”页面创建的 API Key。

#### OpenAI Chat Completions

```bash
curl -X POST http://localhost:15722/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

#### Anthropic Messages

```bash
curl -X POST http://localhost:15722/v1/messages \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 1024
  }'
```

也可以使用 Anthropic 常见认证头：

```text
x-api-key: YOUR_API_KEY
```

#### Gemini Native

```bash
curl -X POST http://localhost:15722/v1beta/models/gemini-pro:generateContent \
  -H "x-goog-api-key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{"role": "user", "parts": [{"text": "Hello"}]}]
  }'
```

#### OpenAI Responses API

```bash
curl -X POST http://localhost:15722/v1/responses \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "instructions": "You are a helpful assistant",
    "input": "Hello"
  }'
```

#### 流式响应

```bash
curl -X POST http://localhost:15722/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Count to 5"}],
    "stream": true
  }'
```

## 全局代理架构

APIRelay 已移除 app 隔离概念。所有协议入口进入同一套全局路由、转发、熔断和响应处理体系：

```text
Client / SDK
    │
    ▼
兼容协议入口（OpenAI / Anthropic / Gemini / Codex）
    │  仅设置 RelayKind / 协议模式，不设置 app
    ▼
RequestContext
    │
    ▼
ProviderRouter
    │  读取全局 ProxyConfig
    │  读取全局 FailoverQueue
    │  按 channel_id 检查 CircuitBreaker
    ▼
Forwarder
    │  构建上游请求
    │  上游失败、预检失败、首包超时后按全局队列切换渠道
    ▼
ResponseProcessor
    │  非流式：完整读取并转换后记录成功
    │  流式：首包预读，开始输出后不再切换渠道或写 JSON 错误
    ▼
Client
```

### 核心模块

- **ProviderRouter**：读取全局代理配置和全局故障转移队列，选择可用渠道并跳过打开状态的熔断器。
- **Forwarder**：统一执行渠道选择、上游请求、预检和失败重试。
- **CircuitBreaker**：按 `channel_id` 维护全局熔断状态，支持 closed / open / half_open。
- **ResponseProcessor**：负责响应预检、hop-by-hop header 剥离、非流式完整读取、SSE 透传/转换辅助。
- **Protocol Adaptor**：在不同协议格式之间转换请求和响应。

## 兼容路由矩阵

以下兼容路径会由后端 API 路由处理，不会落到 SPA HTML 或 Gin 默认 404：

```text
GET  /models
GET  /v1/models
POST /v1/messages
POST /claude/v1/messages
POST /chat/completions
POST /v1/chat/completions
POST /v1/v1/chat/completions
POST /codex/v1/chat/completions
POST /responses
POST /v1/responses
POST /v1/v1/responses
POST /codex/v1/responses
POST /responses/compact
POST /v1/responses/compact
ANY  /v1beta/*path
ANY  /gemini/v1beta/*path
ANY  /gemini/v1/*path
GET  /health
GET  /status
```

## 配置说明

首次启动会自动生成 `config.yml`，主要配置项：

```yaml
server:
  host: 127.0.0.1
  port: 15722
  mode: release  # debug | release

auth:
  admin_key: change-me-in-production  # 管理密钥（必须修改）
  require_login: false

logging:
  level: info  # debug | info | warn | error
  output: file # stdout | file
  file: ./logs/apirelay.log
  request_log: true

cors:
  enabled: true
  allow_origins:
    - "*"
  allow_methods:
    - GET
    - POST
    - PUT
    - DELETE
    - OPTIONS
```

代理运行时配置存储在数据库的全局代理配置表中，可通过 `/proxy` 页面或 `/api/proxy/config` 读写。

## API 文档

### 管理 API

所有管理接口需要在请求头中携带管理密钥：

```text
Authorization: Bearer YOUR_ADMIN_KEY
```

#### 渠道管理

- `GET /api/channels` - 获取渠道列表。
- `POST /api/channels` - 创建渠道。
- `GET /api/channels/:id` - 获取渠道详情。
- `PUT /api/channels/:id` - 更新渠道。
- `DELETE /api/channels/:id` - 删除渠道。
- `POST /api/channels/:id/models` - 获取渠道模型列表。
- `PUT /api/channels/reorder` - 批量调整渠道优先级。

#### 模型管理

- `GET /api/models` - 获取模型列表。
- `GET /api/models/available` - 获取可用模型。
- `PUT /api/models/:id` - 更新模型显示名/启用状态。
- `DELETE /api/models/:id` - 删除模型。

#### 全局代理管理

- `GET /api/proxy/status` - 获取全局代理配置、全局故障转移队列和熔断状态。
- `GET /api/proxy/config` - 获取全局代理配置。
- `PUT /api/proxy/config` - 更新全局代理配置。
- `GET /api/proxy/failover-queue` - 获取全局故障转移队列。
- `PUT /api/proxy/failover-queue` - 更新全局故障转移队列。
- `GET /api/proxy/circuits` - 获取每个渠道的全局熔断状态。
- `POST /api/proxy/circuits/:channel_id/reset` - 重置指定渠道的熔断器。

#### API 密钥管理

- `GET /api/keys` - 获取密钥列表。
- `POST /api/keys` - 创建密钥。
- `DELETE /api/keys/:id` - 删除密钥。

#### 日志查询

- `GET /api/logs` - 获取请求日志。

#### 系统信息

- `GET /api/system/health` - 健康检查（无需认证）。
- `GET /api/system/info` - 系统信息。

### 兼容 API

兼容接口支持以下认证方式：

```text
Authorization: Bearer YOUR_API_KEY
x-api-key: YOUR_API_KEY          # Anthropic 常用
x-goog-api-key: YOUR_API_KEY     # Gemini 常用
?key=YOUR_API_KEY                # Gemini 常用
```

## 常见问题

### 1. APIRelay 是否按 app 类型隔离代理配置？

不隔离。当前实现使用全局代理配置、全局故障转移队列和全局熔断器。协议入口只标记请求类型和协议格式，不参与配置隔离。

### 2. 渠道优先级和故障转移如何工作？

- 开启自动故障转移时，按 `/proxy` 页面配置的全局队列顺序尝试渠道。
- 关闭自动故障转移时，只选择最高优先级的可用渠道。
- 熔断状态为 open 的渠道会被跳过。
- 上游失败、预检失败、流式首包超时等发生在写客户端前的问题可触发下一个渠道。
- 流式响应一旦开始写客户端，就不再切换渠道，也不会再写 JSON 错误。

### 3. 如何查看和重置熔断器？

进入 `/proxy` 页面查看每个渠道的熔断状态，也可以调用：

```bash
curl http://localhost:15722/api/proxy/circuits \
  -H "Authorization: Bearer YOUR_ADMIN_KEY"

curl -X POST http://localhost:15722/api/proxy/circuits/1/reset \
  -H "Authorization: Bearer YOUR_ADMIN_KEY"
```

### 4. 如何查看请求日志？

在管理后台“请求日志”页面查看，或通过 API 查询：

```bash
curl http://localhost:15722/api/logs \
  -H "Authorization: Bearer YOUR_ADMIN_KEY"
```

## 本地开发

### 环境要求

- Go 1.21+
- Node.js 18+
- SQLite 3

### 后端开发

```bash
# 安装依赖
go mod download

# 运行
go run cmd/server/main.go

# 构建
go build -o apirelay cmd/server/main.go

# 测试
go test ./...
```

### 前端开发

```bash
cd web

# 安装依赖
npm install

# 开发模式（热更新）
npm run dev

# 构建生产版本
npm run build
```

## 贡献指南

欢迎提交 Issue 和 Pull Request。

## 许可证

[MIT License](LICENSE)

## 致谢

本项目参考了以下优秀项目的设计：

- [NewAPI](https://github.com/Calcium-Ion/new-api) - API 网关功能
- [CCSwitch](https://github.com/farion1231/cc-switch) - 故障转移与代理管理思路

## 联系方式

如有问题或建议，请通过 [GitHub Issues](https://github.com/TheFloodDragon/APIRelay/issues) 联系。
