# APIRelay

轻量级 API 调度中心，支持多渠道统一管理和 OpenAI / Anthropic / Gemini 兼容转发。

## 特性

- **多渠道管理**：统一管理 OpenAI、Claude、Gemini、DeepSeek 等多个 API 渠道
- **智能调度**：支持优先级、加权随机、轮询等多种调度策略
- **协议转换**：自动转换 OpenAI、Anthropic、Gemini 请求/响应格式
- **模型路由**：支持模型别名、重定向、模型组等高级路由功能
- **健康检查**：自动检测渠道健康状态，失败自动切换
- **流式响应**：完整支持 SSE 流式输出
- **可视化管理**：优雅的 Web 管理界面，支持拖拽排序
- **请求日志**：记录所有请求详情，便于调试和监控
- **前后端一体**：单文件二进制，开箱即用

## 快速开始

### 下载并运行

1. 从 [Releases](https://github.com/TheFloodDragon/APIRelay/releases) 下载对应平台的二进制文件

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
   ```
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

打开管理后台，进入"渠道管理"页面：

1. 点击"添加渠道"
2. 选择协议类型（OpenAI / Anthropic / Gemini）
3. 填写渠道名称和 API Key
4. 点击"获取模型"自动拉取可用模型列表
5. 设置优先级和权重
6. 保存

### 2. 兼容 API 调用

#### OpenAI 格式

```bash
curl -X POST http://localhost:15722/v1/chat/completions \
  -H "Authorization: Bearer YOUR_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

#### Anthropic 格式

```bash
curl -X POST http://localhost:15722/v1/messages \
  -H "Authorization: Bearer YOUR_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-opus-20240229",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 1024
  }'
```

#### Gemini 格式

```bash
curl -X POST http://localhost:15722/v1beta/models/gemini-pro:generateContent \
  -H "Authorization: Bearer YOUR_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{"role": "user", "parts": [{"text": "Hello"}]}]
  }'
```

#### OpenAI Responses API

```bash
curl -X POST http://localhost:15722/v1/responses \
  -H "Authorization: Bearer YOUR_ADMIN_KEY" \
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
  -H "Authorization: Bearer YOUR_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Count to 5"}],
    "stream": true
  }'
```

### 3. 模型路由功能

#### 模型别名

在管理后台"模型列表"页面设置显示名称，将上游模型名映射为对外调用名：

- `gpt-4-0613` → `gpt-4`
- `claude-3-opus-20240229` → `claude-3-opus`

#### 模型重定向

配置模型重定向，自动将请求转到其他模型：

```bash
curl -X POST http://localhost:15722/api/routes/redirects \
  -H "Authorization: Bearer YOUR_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "gpt-4",
    "target": "claude-3-opus"
  }'
```

#### 模型组

配置模型组，一个请求可以同时尝试多个模型：

```bash
curl -X POST http://localhost:15722/api/routes/groups \
  -H "Authorization: Bearer YOUR_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "group": "premium",
    "models": ["gpt-4", "claude-3-opus", "gemini-pro"]
  }'
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
  require_login: false  # 是否需要登录

scheduler:
  strategy: priority  # priority | weighted | round_robin
  health_check_interval: 60  # 健康检查间隔（秒）
  unhealthy_threshold: 3  # 失败次数阈值

logging:
  level: info  # debug | info | warn | error
  output: file  # stdout | file
  file: ./logs/apirelay.log
  request_log: true  # 是否记录请求日志

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

## 架构设计

```
┌─────────────────────────────────────────────────────────┐
│                       Client                            │
│  (OpenAI SDK / Anthropic SDK / Gemini SDK / curl)      │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│                    APIRelay                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │ API Router  │  │  Protocol   │  │  Scheduler  │    │
│  │             │→ │   Adaptor   │→ │             │    │
│  └─────────────┘  └─────────────┘  └─────────────┘    │
│         │               │                   │           │
│         ▼               ▼                   ▼           │
│  ┌──────────────────────────────────────────────────┐  │
│  │         Channel Pool & Health Check              │  │
│  └──────────────────────────────────────────────────┘  │
└────────────────────┬────────────────────────────────────┘
                     │
         ┌───────────┼───────────┐
         ▼           ▼           ▼
    ┌────────┐  ┌────────┐  ┌────────┐
    │ OpenAI │  │ Claude │  │ Gemini │
    │   API  │  │   API  │  │   API  │
    └────────┘  └────────┘  └────────┘
```

### 核心模块

- **API Router**：处理兼容 API 请求，支持 OpenAI / Anthropic / Gemini 格式
- **Protocol Adaptor**：协议转换，统一请求/响应格式
- **Scheduler**：渠道调度，支持优先级、加权、轮询等策略
- **Health Check**：定期检测渠道健康状态，自动切换
- **Model Router**：模型路由，支持别名、重定向、模型组

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

### 测试

```bash
# 运行测试
go test ./...

# 格式化代码
go fmt ./...
gofmt -w .
```

## API 文档

### 管理 API

所有管理接口需要在请求头中携带管理密钥：

```
Authorization: Bearer YOUR_ADMIN_KEY
```

#### 渠道管理

- `GET /api/channels` - 获取渠道列表
- `POST /api/channels` - 创建渠道
- `GET /api/channels/:id` - 获取渠道详情
- `PUT /api/channels/:id` - 更新渠道
- `DELETE /api/channels/:id` - 删除渠道
- `POST /api/channels/:id/test` - 测试渠道连接
- `POST /api/channels/:id/models` - 获取渠道模型列表
- `PUT /api/channels/reorder` - 批量调整渠道优先级

#### 模型管理

- `GET /api/models` - 获取模型列表
- `GET /api/models/available` - 获取可用模型
- `PUT /api/models/:id` - 更新模型元数据

#### 路由管理

- `GET /api/routes` - 获取所有路由配置
- `POST /api/routes/aliases` - 设置模型别名
- `DELETE /api/routes/aliases/:alias` - 删除模型别名
- `POST /api/routes/redirects` - 设置模型重定向
- `DELETE /api/routes/redirects/:source` - 删除模型重定向
- `POST /api/routes/groups` - 设置模型组
- `DELETE /api/routes/groups/:group` - 删除模型组
- `POST /api/routes/reload` - 重新加载路由配置

#### API 密钥管理

- `GET /api/keys` - 获取密钥列表
- `POST /api/keys` - 创建密钥
- `DELETE /api/keys/:id` - 删除密钥

#### 日志查询

- `GET /api/logs` - 获取请求日志

#### 系统信息

- `GET /api/system/health` - 健康检查（无需认证）
- `GET /api/system/info` - 系统信息

### 兼容 API

所有兼容接口需要在请求头中携带 API Key：

```
Authorization: Bearer YOUR_API_KEY
```

或使用特定协议的认证方式：

```
# Anthropic
x-api-key: YOUR_API_KEY

# Gemini
x-goog-api-key: YOUR_API_KEY
```

#### OpenAI 兼容

- `GET /v1/models` - 获取可用模型列表
- `GET /v1/models/:model` - 获取单个模型信息
- `POST /v1/chat/completions` - 聊天补全
- `POST /v1/completions` - 文本补全
- `POST /v1/embeddings` - 文本嵌入
- `POST /v1/responses` - Responses API

#### Anthropic 兼容

- `POST /v1/messages` - Claude Messages API

#### Gemini 兼容

- `GET /v1beta/models` - 获取 Gemini 模型列表
- `GET /v1beta/models/:model` - 获取单个模型信息
- `POST /v1beta/models/:model:generateContent` - 生成内容
- `POST /v1beta/models/:model:streamGenerateContent` - 流式生成内容

## 常见问题

### 1. 如何使用管理密钥？

管理密钥用于访问管理接口和作为调试用的 API Key。首次启动时请立即修改 `config.yml` 中的 `auth.admin_key`。

### 2. 如何创建用于客户端的 API Key？

在管理后台"API 密钥管理"页面创建，支持设置允许的模型和 IP 白名单。

### 3. 渠道优先级如何工作？

- **priority 模式**：按优先级从高到低依次尝试
- **weighted 模式**：按权重加权随机选择
- **round_robin 模式**：轮询选择所有渠道

### 4. 如何处理上游 API 失败？

APIRelay 会自动重试其他可用渠道。可在 `config.yml` 中配置健康检查参数：

```yaml
scheduler:
  health_check_interval: 60
  unhealthy_threshold: 3
```

### 5. 支持哪些协议转换？

- OpenAI → Anthropic
- OpenAI → Gemini
- Anthropic → OpenAI
- Anthropic → Gemini
- Gemini → OpenAI
- Gemini → Anthropic

### 6. 如何查看请求日志？

在管理后台"请求日志"页面查看，或通过 API 查询：

```bash
curl http://localhost:15722/api/logs \
  -H "Authorization: Bearer YOUR_ADMIN_KEY"
```

## 贡献指南

欢迎提交 Issue 和 Pull Request！

开发流程：

1. Fork 本仓库
2. 创建特性分支：`git checkout -b feature/xxx`
3. 提交代码：`git commit -m "feat: add xxx"`
4. 推送分支：`git push origin feature/xxx`
5. 提交 Pull Request

## 许可证

[MIT License](LICENSE)

## 致谢

本项目参考了以下优秀项目的设计：

- [NewAPI](https://github.com/Calcium-Ion/new-api) - API 网关功能
- [CCSwitch](https://github.com/farion1231/cc-switch) - 管理界面设计

## 联系方式

如有问题或建议，请通过 [GitHub Issues](https://github.com/TheFloodDragon/APIRelay/issues) 联系。
