# APIRelay 开发计划

## 项目概述

**APIRelay** 是一个轻量级 API 调度中心，参考 NewAPI 网关能力和 CCSwitch 管理体验，提供：

- 多渠道统一管理（OpenAI、Claude、Gemini、DeepSeek 等）
- 优先级调度与失败重试
- OpenAI 兼容接口转发
- 拖拽式优先级调整
- 健康检查与请求日志

**技术栈**：Go 1.21 + Gin + GORM + SQLite | Vue 3 + TypeScript + Element Plus

**当前状态**：MVP 骨架已完成并提交（commit `2480efd`），待编译验证和集成测试。

---

## 阶段一：核心验证与修复（1-2 天）

**目标**：确保基础架构可编译、可运行、可用。

### 1.1 编译验证

- [ ] 在有 Go 环境的机器上运行：
  ```bash
  go mod download
  go build -o apirelay ./cmd/server
  ```
- [ ] 修复编译错误（如有）
- [ ] 前端构建验证：
  ```bash
  cd web
  npm install
  npm run build
  ```
- [ ] 修复 TypeScript/Vite 构建错误（如有）

### 1.2 本地启动测试

- [ ] 启动后端：`./apirelay`（首次启动会自动生成 `config.yml`）
- [ ] 确认服务监听 `http://localhost:15722`
- [ ] 访问健康检查：`GET /api/system/health`（应返回 200 OK）
- [ ] 前端开发模式：`cd web && npm run dev`
- [ ] 访问前端：`http://localhost:5173`

### 1.3 管理 API 集成测试

使用 `curl` 或 Postman 测试：

**环境变量**：
```bash
export ADMIN_KEY="change-me-in-production"
```

**测试用例**：

1. **创建渠道**：
   ```bash
   curl -X POST http://localhost:15722/api/channels \
     -H "Authorization: Bearer $ADMIN_KEY" \
     -H "Content-Type: application/json" \
     -d '{
       "name": "OpenAI Test",
       "type": "openai",
       "api_key": "sk-test-key",
       "base_url": "https://api.openai.com/v1",
       "models": ["gpt-4o", "gpt-3.5-turbo"],
       "priority": 10,
       "weight": 1,
       "enabled": true
     }'
   ```

2. **获取渠道列表**：
   ```bash
   curl http://localhost:15722/api/channels \
     -H "Authorization: Bearer $ADMIN_KEY"
   ```

3. **测试渠道连接**（需要真实 API Key）：
   ```bash
   curl -X POST http://localhost:15722/api/channels/1/test \
     -H "Authorization: Bearer $ADMIN_KEY"
   ```

4. **自动获取模型**：
   ```bash
   curl -X POST http://localhost:15722/api/channels/1/models \
     -H "Authorization: Bearer $ADMIN_KEY"
   ```

5. **批量调整优先级**：
   ```bash
   curl -X PUT http://localhost:15722/api/channels/reorder \
     -H "Authorization: Bearer $ADMIN_KEY" \
     -H "Content-Type: application/json" \
     -d '{
       "orders": [
         {"id": 1, "priority": 10},
         {"id": 2, "priority": 8}
       ]
     }'
   ```

### 1.4 OpenAI 兼容接口测试

**前置条件**：至少有一个启用的渠道和模型。

1. **获取可用模型**：
   ```bash
   curl http://localhost:15722/v1/models \
     -H "Authorization: Bearer $ADMIN_KEY"
   ```

2. **聊天补全**（需要真实上游 API）：
   ```bash
   curl -X POST http://localhost:15722/v1/chat/completions \
     -H "Authorization: Bearer $ADMIN_KEY" \
     -H "Content-Type: application/json" \
     -d '{
       "model": "gpt-3.5-turbo",
       "messages": [{"role": "user", "content": "Hello"}]
     }'
   ```

3. **检查请求日志**：
   ```bash
   curl http://localhost:15722/api/logs \
     -H "Authorization: Bearer $ADMIN_KEY"
   ```

### 1.5 前端集成测试

- [ ] 打开渠道管理页面
- [ ] 在顶部输入管理密钥并保存
- [ ] 添加一个测试渠道
- [ ] 点击"获取模型"
- [ ] 拖拽调整渠道顺序
- [ ] 启用/禁用渠道开关
- [ ] 编辑渠道信息
- [ ] 删除渠道

**验收标准**：所有操作无报错，渠道列表实时更新。

---

## 阶段二：核心功能增强（已完成）

**目标**：补齐 MVP 缺失的关键能力。

**完成状态**：已支持流式响应、高级调度策略、定时健康检查和 OpenAI / Anthropic / Gemini 协议适配，并补充了调度与健康检查单元测试。

### 2.1 流式响应支持（已完成）

**状态**：`/v1/chat/completions`、Responses 桥接和跨协议流式转发已支持 `stream: true` / SSE。

**实现**：

1. 检测请求 body 中的 `stream` 字段
2. 若 `stream=true`，设置响应头：
   ```
   Content-Type: text/event-stream
   Cache-Control: no-cache
   Connection: keep-alive
   ```
3. 转发到上游渠道，逐行读取 SSE 响应并透传
4. 处理上游连接断开和重试逻辑

**验证**：
```bash
curl -X POST http://localhost:15722/v1/chat/completions \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Count to 5"}],
    "stream": true
  }'
```

应实时输出 `data: {...}` 格式的流。

### 2.2 高级调度策略（已完成）

**当前**：已支持 `priority`、`weighted`、`round_robin`，并为加权/轮询核心逻辑补充单元测试。

**新增**：

- **weighted**：按权重加权随机
  - 实现公式：`rand.Intn(totalWeight)` 映射到渠道
- **round_robin**：轮询选择
  - 使用内存计数器记录上次选择的渠道索引

**配置**：
```yaml
scheduler:
  strategy: weighted  # priority | weighted | round_robin
```

**验证**：添加多个权重不同的渠道，发送多次请求，观察日志分布是否符合权重比例。

### 2.3 定时健康检查（已完成）

**实现**：

1. 在 `main.go` 启动时启动 goroutine
2. 每隔 `scheduler.health_check_interval` 秒（默认 60s）
3. 遍历所有启用渠道，调用 `/models` 接口
4. 成功：`health_status = healthy`
5. 失败：累计失败次数，达到 `unhealthy_threshold` 后标记为 `unhealthy`
6. 更新 `last_check` 时间戳

**优化**：不健康的渠道不参与调度，但仍继续检查，恢复后自动重新启用。

### 2.4 协议适配器（Anthropic/Gemini）（已完成）

**当前**：已支持 OpenAI、Anthropic、Gemini 调用格式与上游渠道类型之间的请求/响应转换。

**目标**：自动转换请求/响应格式。

#### Anthropic Claude 适配

**请求转换**：
```json
// OpenAI
{
  "model": "gpt-4",
  "messages": [{"role": "user", "content": "Hello"}]
}

// 转为 Anthropic
{
  "model": "claude-3-opus-20240229",
  "messages": [{"role": "user", "content": "Hello"}],
  "max_tokens": 1024
}
```

**响应转换**：
```json
// Anthropic
{
  "id": "msg_xxx",
  "type": "message",
  "content": [{"type": "text", "text": "Hi"}]
}

// 转为 OpenAI
{
  "id": "msg_xxx",
  "object": "chat.completion",
  "choices": [{"message": {"role": "assistant", "content": "Hi"}}]
}
```

**实现位置**：`internal/adapter/` 下新增 `anthropic_adapter.go` 和 `gemini_adapter.go`。

**验证**：添加 Claude 渠道，使用 OpenAI 客户端请求，应正常返回。

---

## 阶段三：前端完善（2-3 天）

**目标**：补齐管理后台缺失的页面。

### 3.1 仪表盘（Dashboard）

**展示内容**：

- 总请求数、成功率、平均延迟
- 各渠道请求分布（饼图）
- 最近 24 小时请求趋势（折线图）
- 渠道健康状态卡片

**技术栈**：ECharts + 后端统计 API

**后端新增接口**：
```
GET /api/stats/overview
GET /api/stats/timeline?start=xxx&end=xxx
GET /api/stats/channels
```

### 3.2 日志查询页面

**功能**：

- 表格展示最近请求日志
- 筛选：时间范围、渠道、模型、状态码
- 分页加载
- 点击查看详情（请求参数、响应、错误信息）

**后端已有接口**：
```
GET /api/logs?limit=50&offset=0
```

### 3.3 API 密钥管理页面

**功能**：

- 列表展示所有密钥（隐藏完整 Key）
- 创建密钥：自动生成 `sk-ar-xxx`
- 设置允许模型、IP 白名单
- 删除密钥

**后端已有接口**：
```
GET    /api/keys
POST   /api/keys
DELETE /api/keys/:id
```

### 3.4 模型管理页面

**功能**：

- 展示所有模型及其所属渠道
- 设置模型别名（如 `gpt-4` → `gpt-4o`）
- 模型重定向（如请求 `gpt-4` 自动转到 `claude-3-opus`）

**后端已有接口**：
```
GET /api/models
GET /api/models/available
```

**需新增**：
```
POST /api/models/alias
POST /api/models/redirect
```

---

## 阶段四：生产就绪（2-3 天）

**目标**：优化性能、安全、监控，可用于生产环境。

### 4.1 性能优化

- [ ] 启用 Gin 的 `ReleaseMode`
- [ ] 添加数据库连接池配置
- [ ] 对高频查询添加索引（如 `request_logs.created_at`、`channels.priority`）

### 4.2 安全加固

- [ ] 强制修改默认 `admin_key`（启动时检测并警告）
- [ ] API Key 存储使用 bcrypt 哈希
- [ ] 请求日志脱敏（不记录完整 API Key）
- [ ] CORS 配置生产环境白名单
- [ ] 添加请求体大小限制（防止 DoS）

### 4.4 监控与日志

- [ ] 集成 Prometheus metrics（可选）
  ```
  /metrics
  ```
  暴露：
  - `apirelay_requests_total`
  - `apirelay_request_duration_seconds`
  - `apirelay_channel_errors_total`

- [ ] 结构化日志（JSON 格式）
- [ ] 日志轮转（使用 `lumberjack`）

### 4.6 文档编写

**README.md**：

```markdown
# APIRelay

API 调度中心，支持多渠道统一管理和 OpenAI 兼容转发。

## 快速开始

### 本地二进制部署（推荐）

1. 下载对应平台的构建产物并解压。

2. 首次启动生成默认配置：
   ```bash
   ./apirelay
   ```

3. 修改配置后重启：
   ```bash
   vim config.yml  # 修改 admin_key
   ./apirelay --config config.yml
   ```

4. 访问管理后台：
   ```
   http://localhost:15722
   ```

### 本地开发

**后端**：
```bash
go run cmd/server/main.go
```

**前端**：
```bash
cd web
npm install
npm run dev
```

## 使用指南

### 添加渠道

1. 打开渠道管理页面
2. 点击"添加渠道"
3. 填写渠道信息和 API Key
4. 点击"获取模型"自动拉取模型列表
5. 保存

### OpenAI 兼容调用

```bash
curl -X POST http://localhost:15722/v1/chat/completions \
  -H "Authorization: Bearer YOUR_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

## 配置说明

首次启动会自动生成 `config.yml`；默认模板维护在 `pkg/config/default_config.yml`。

## API 文档

见 [docs/api.md](docs/api.md)。

## License

MIT
```

**API 文档**（`docs/api.md`）：

详细列出所有管理接口和 OpenAI 兼容接口的请求/响应格式。

---

## 阶段五：扩展功能（可选）

**这些功能不影响核心使用，可根据需求选择实现。**

### 5.1 WebSocket 实时监控

- 后端：`/api/logs/stream`（WebSocket）
- 前端：日志页面订阅，实时显示新请求

### 5.2 用户体系（多租户）

- 添加 `users` 表
- 每个用户独立管理渠道和 API Key
- JWT 登录认证

### 5.3 计费与额度

- 添加 `quotas` 表
- 记录每个 API Key 的 Token 消耗
- 超额自动禁用

### 5.4 渠道预设模板

参考 CCSwitch，预设 50+ 常见渠道模板（OpenAI、Claude、Gemini、DeepSeek、讯飞星火等），一键添加。

---

## 阶段六：对齐 CCSwitch 的模型测试与供应商编辑体验（新增规划）

**目标**：在保留 APIRelay 作为服务端 API Relay 的定位下，吸收 CCSwitch 在“供应商编辑”“模型拉取”“模型检查 / Stream Check”“端点测速”上的成熟交互，让渠道管理从当前的基础 CRUD 升级为可诊断、可验证、可快速配置的供应商工作台。

**调研来源**：已对照 `farion1231/cc-switch` 源码与文档，重点参考：

- `docs/user-manual/zh/2-providers/2.3-edit.md`
- `docs/user-manual/zh/4-proxy/4.5-model-test.md`
- `src/lib/api/model-fetch.ts`
- `src/lib/api/model-test.ts`
- `src/components/providers/forms/ProviderAdvancedConfig.tsx`
- `src/components/providers/forms/shared/ModelInputWithFetch.tsx`
- `src/components/providers/forms/EndpointSpeedTest.tsx`
- `src-tauri/src/services/stream_check.rs`

### 6.1 供应商编辑方向

当前 APIRelay 的“渠道”应逐步升级为更接近 CCSwitch 的“供应商”概念，但后端命名可继续沿用 `Channel`，前端展示改为“供应商 / 渠道”。

#### 6.1.1 基础信息增强

在 `channels` 表和前端编辑面板中补充：

- `website_url`：供应商官网、控制台或文档地址
- `notes`：备注信息
- `icon`：供应商图标标识
- `icon_color`：图标颜色
- `category`：供应商分类
  - `official`：官方
  - `aggregator`：聚合服务
  - `third_party`：第三方中转
  - `custom`：自定义
- `provider_type`：供应商类型标识，如 `openai`、`anthropic`、`gemini`、`deepseek`、`openrouter`、`newapi`

**前端表现**：

- 卡片顶部展示图标、名称、分类、健康状态
- 编辑弹窗拆分为：基础信息、认证与端点、模型配置、高级测试配置、调度配置
- API Key 输入支持显示/隐藏
- 网站地址可一键打开
- 备注在卡片或详情中展示

#### 6.1.2 供应商预设模板

参考 CCSwitch 的预设选择器，新增 APIRelay 的供应商模板系统。

模板字段建议：

```json
{
  "id": "deepseek",
  "name": "DeepSeek",
  "category": "official",
  "provider_type": "deepseek",
  "protocol": "openai_compatible",
  "base_url": "https://api.deepseek.com/v1",
  "models_url": "https://api.deepseek.com/v1/models",
  "website_url": "https://platform.deepseek.com",
  "icon": "deepseek",
  "icon_color": "#4D6BFE",
  "default_test_model": "deepseek-chat",
  "default_models": ["deepseek-chat", "deepseek-reasoner"]
}
```

落地方式：

- 前端先内置 `web/src/config/providerPresets.ts`
- 后端后续可暴露 `GET /api/provider-presets`
- 添加供应商时选择预设，自动填充协议、Base URL、模型获取地址、测试模型、官网地址
- 保留“自定义供应商”入口

#### 6.1.3 端点管理与测速

参考 CCSwitch 的 `EndpointSpeedTest`，为每个供应商支持多个候选端点。

新增数据结构：

```go
type ChannelEndpoint struct {
  ID        uint      `json:"id"`
  ChannelID uint      `json:"channel_id"`
  URL       string    `json:"url"`
  IsDefault bool      `json:"is_default"`
  IsCustom  bool      `json:"is_custom"`
  Latency   *int      `json:"latency"`
  Status    *int      `json:"status"`
  Error     string    `json:"error"`
  CreatedAt time.Time `json:"created_at"`
  UpdatedAt time.Time `json:"updated_at"`
}
```

新增接口：

```http
GET    /api/channels/:id/endpoints
POST   /api/channels/:id/endpoints
DELETE /api/channels/:id/endpoints/:endpoint_id
POST   /api/channels/:id/endpoints/test
POST   /api/channels/:id/endpoints/select-fastest
```

功能要求：

- 支持手动添加 / 删除端点
- 支持测速多个端点，展示延迟、HTTP 状态、错误
- 支持“自动选择最快端点”
- Base URL 保存前统一去除尾部 `/`
- 端点 URL 校验只允许 `http://localhost` / `http://127.0.0.1` / `https://`，避免误填危险地址

### 6.2 模型拉取方向

当前 `POST /api/channels/:id/models` 只是按协议简单拉取。应对齐 CCSwitch 的模型拉取体验：前端预检、候选 models URL、错误分类、分组下拉选择。

#### 6.2.1 模型拉取参数

新增可选字段：

- `models_url`：精确指定模型列表接口
- `is_full_url`：Base URL 是否已经是完整接口地址，不再拼接 `/v1` 或 `/models`
- `model_fetch_mode`：模型获取策略
  - `auto`：根据协议自动推导候选 URL
  - `custom`：只使用 `models_url`
  - `static`：不请求远端，仅使用手动模型列表

`Channel` 建议扩展：

```go
ModelsURL      string `json:"models_url"`
IsFullURL      bool   `json:"is_full_url"`
ModelFetchMode string `json:"model_fetch_mode"`
```

#### 6.2.2 候选 URL 策略

OpenAI 兼容供应商：

1. 如果存在 `models_url`，优先使用
2. 如果 `base_url` 已以 `/v1` 结尾，尝试 `{base_url}/models`
3. 如果 `base_url` 不含 `/v1`，尝试 `{base_url}/v1/models` 和 `{base_url}/models`
4. 对包含 `/anthropic`、`/openai` 等兼容子路径的中转地址，允许剥离子路径后重试

Gemini：

- 默认尝试 `{base_url}/models`
- 兼容官方 `v1beta/models`

Anthropic：

- 官方 API 无稳定模型列表时，使用内置静态模型目录
- 第三方 Anthropic 兼容接口可尝试 `models_url` 或 OpenAI 兼容 `/models`

#### 6.2.3 前端模型输入体验

参考 CCSwitch `ModelInputWithFetch`：

- 在模型输入框旁显示“获取模型”按钮
- 获取中显示 loading
- 成功后展示分组下拉菜单
- 分组维度：`owned_by` / provider / 模型名前缀
- 点击模型自动填入测试模型或模型列表
- 错误提示按类型区分：
  - 缺少 API Key
  - 缺少端点地址
  - 401 / 403：认证失败
  - 404 / 405：模型接口不存在
  - timeout：请求超时
  - parse error：响应不兼容

### 6.3 模型测试 / Stream Check 方向

当前 `POST /api/channels/:id/test` 主要通过拉取模型列表判断连通性，不足以验证“指定模型能否真实完成请求”。应升级为类似 CCSwitch 的模型检查：发送真实短请求，检测响应状态、延迟、首字节时间和错误分类。

#### 6.3.1 健康状态扩展

将渠道健康状态扩展为：

- `unknown`：未测试
- `healthy`：请求成功且延迟正常
- `degraded`：请求成功但超过降级阈值
- `unhealthy`：认证失败、模型不存在、超时或服务错误

保留现有 `healthy/unhealthy/unknown` 兼容展示，前端新增黄色“降级”状态。

#### 6.3.2 全局模型测试配置

新增系统配置项，可存入 `system_configs`：

```json
{
  "timeout_secs": 45,
  "max_retries": 2,
  "degraded_threshold_ms": 6000,
  "test_prompt": "Who are you?",
  "max_tokens": 20,
  "stream": true,
  "default_models": {
    "openai_compatible": "gpt-4o-mini",
    "anthropic": "claude-3-5-haiku-latest",
    "gemini": "gemini-1.5-flash"
  }
}
```

新增接口：

```http
GET /api/model-test/config
PUT /api/model-test/config
```

前端新增“模型测试设置”面板，包含：

- 默认测试模型
- 测试 prompt
- 超时时间
- 最大重试次数
- 降级阈值
- 是否使用流式测试
- 最大输出 token

#### 6.3.3 供应商单独测试配置

参考 CCSwitch 的 `ProviderTestConfig`，允许每个供应商覆盖全局测试配置。

建议保存到 `Channel.Config`：

```json
{
  "test_config": {
    "enabled": true,
    "test_model": "deepseek-chat",
    "timeout_secs": 30,
    "test_prompt": "Who are you?",
    "degraded_threshold_ms": 5000,
    "max_retries": 1,
    "stream": true
  }
}
```

前端放在供应商编辑弹窗的“高级测试配置”折叠面板中：

- 开关：使用单独配置
- 测试模型
- 超时时间
- 测试 prompt
- 降级阈值
- 最大重试次数

#### 6.3.4 模型测试接口

新增接口：

```http
POST /api/channels/:id/model-test
POST /api/channels/model-test/batch
GET  /api/channels/:id/model-test/logs?limit=20
```

单个测试响应：

```json
{
  "success": true,
  "status": "healthy",
  "message": "模型测试通过",
  "response_time_ms": 842,
  "ttfb_ms": 210,
  "http_status": 200,
  "model_used": "gpt-4o-mini",
  "tested_at": "2026-06-08T00:00:00Z",
  "retry_count": 0,
  "error_category": ""
}
```

错误分类：

- `auth_failed`：401 / 403
- `model_not_found`：模型不存在或 404 且响应体包含模型错误
- `rate_limited`：429
- `timeout`：请求超时
- `network_error`：DNS / 连接失败
- `server_error`：5xx
- `invalid_response`：响应格式无法解析
- `unsupported_protocol`：当前协议暂不支持真实模型测试

#### 6.3.5 流式测试细节

测试请求要求：

- prompt 尽量短，默认 `Who are you?`
- `max_tokens` 控制在 10-50
- 默认优先 stream，用于计算首字节时间 `ttfb_ms`
- 不支持 stream 的供应商自动降级为非流式测试
- 测试消耗少量额度，前端首次使用时显示提示

协议适配：

- OpenAI Chat Completions：`POST /chat/completions`
- OpenAI Responses：`POST /responses`
- Anthropic Messages：`POST /messages`
- Gemini Native：`POST /models/{model}:generateContent`

### 6.4 模型测试日志

新增表：

```go
type ModelTestLog struct {
  ID             uint      `json:"id" gorm:"primaryKey"`
  ChannelID      uint      `json:"channel_id" gorm:"index"`
  ChannelName    string    `json:"channel_name"`
  Status         string    `json:"status" gorm:"size:20;index"`
  Success        bool      `json:"success"`
  Message        string    `json:"message"`
  ResponseTimeMS *int      `json:"response_time_ms"`
  TTFBMS         *int      `json:"ttfb_ms"`
  HTTPStatus     *int      `json:"http_status"`
  ModelUsed      string    `json:"model_used"`
  RetryCount     int       `json:"retry_count"`
  ErrorCategory  string    `json:"error_category" gorm:"size:50;index"`
  TestedAt       time.Time `json:"tested_at" gorm:"index"`
}
```

要求：

- 每次手动测试和定时健康检查都可写入日志
- 默认保留最近 7 天或最近 1000 条
- 渠道卡片展示最近一次测试结果
- 供应商详情页展示测试历史

### 6.5 后台健康检查与调度联动

将现有健康检查从“拉取模型列表”升级为“真实模型测试”。

调度规则建议：

- `healthy`：正常参与调度
- `degraded`：仍参与调度，但排序低于 healthy；如启用 weighted，可降低临时权重
- `unhealthy`：不参与调度，继续后台探测，恢复后自动进入调度
- 认证失败 / 模型不存在：不重试过多，避免浪费额度
- 网络错误 / 超时：按 `max_retries` 重试

### 6.6 前端页面调整

#### 渠道 / 供应商卡片

新增展示：

- 图标 + 供应商名称 + 分类
- Base URL / 当前默认端点
- 健康状态：健康 / 降级 / 异常 / 未知
- 最近测试延迟、TTFB、模型名
- 操作按钮：测试、获取模型、端点测速、编辑、删除

#### 编辑弹窗结构

建议拆分为：

1. 预设选择（仅新增时）
2. 基础信息
3. 认证与端点
4. 模型与协议
5. 调度策略
6. 高级测试配置
7. 备注与图标

#### 设置页新增模型测试配置

当前前端暂无独立设置页，可先在渠道页增加“模型测试设置”按钮，后续迁移到系统设置页。

### 6.7 实施顺序

#### 6.7.1 第一批：最小模型测试闭环（开始实施）

**批次目标**：不引入端点表、不重构供应商编辑弹窗，先把“测试渠道连接”从 `/models` 探活升级为“指定模型真实短请求测试”，并提供全局测试配置、状态回写和前端结果展示。

##### A. 后端数据结构

- [ ] 在 `internal/model/models.go` 中新增 `ModelTestLog`：
  - `channel_id`、`channel_name`
  - `status`：`healthy` / `degraded` / `unhealthy`
  - `success`、`message`
  - `response_time_ms`
  - `ttfb_ms`（第一批可为空；后续流式解析补齐）
  - `http_status`
  - `model_used`
  - `retry_count`
  - `error_category`
  - `tested_at`
- [ ] 将 `ModelTestLog` 加入 `internal/model/db.go` 的 `AutoMigrate`
- [ ] 暂不新增 `Channel` 字段，第一批测试配置先存入 `system_configs` 与 `channel.config.test_config`

##### B. 后端配置能力

- [ ] 新增模型测试配置结构：

  ```go
  type ModelTestConfig struct {
    TimeoutSecs         int               `json:"timeout_secs"`
    MaxRetries          int               `json:"max_retries"`
    DegradedThresholdMS int               `json:"degraded_threshold_ms"`
    TestPrompt          string            `json:"test_prompt"`
    MaxTokens           int               `json:"max_tokens"`
    Stream              bool              `json:"stream"`
    DefaultModels        map[string]string `json:"default_models"`
  }
  ```

- [ ] 默认值：
  - `timeout_secs = 45`
  - `max_retries = 2`
  - `degraded_threshold_ms = 6000`
  - `test_prompt = "Who are you?"`
  - `max_tokens = 20`
  - `stream = false`（第一批先走非流式，避免 SSE 解析扩大范围）
  - `default_models.openai_compatible = "gpt-4o-mini"`
  - `default_models.openai = "gpt-4o-mini"`
  - `default_models.deepseek = "deepseek-chat"`
  - `default_models.anthropic = "claude-3-5-haiku-latest"`
  - `default_models.gemini = "gemini-1.5-flash"`
- [ ] 新增 `SystemConfigRepository` 或在现有 service 内封装 `system_configs` 读写
- [ ] 新增接口：
  - `GET /api/model-test/config`
  - `PUT /api/model-test/config`

##### C. 后端模型测试服务

- [ ] 新增 `internal/service/model_test_service.go`
- [ ] 新增核心方法：
  - `GetConfig() (ModelTestConfig, error)`
  - `SaveConfig(ModelTestConfig) error`
  - `TestChannel(channelID uint) (*ModelTestResult, error)`
  - `resolveTestModel(channel, config)`：优先使用 `channel.config.test_config.test_model`，其次全局默认模型，最后使用渠道模型列表第一个
- [ ] 第一批支持的真实测试协议：
  - `openai` / `openai_compatible` / `newapi` / `oneapi` / `deepseek` / `openrouter` / `custom`：`POST {base_url}/chat/completions`
- [ ] 第一批对 `anthropic` / `gemini` 的处理策略：
  - 如果协议已有可靠适配器可复用，则实现真实短请求
  - 若风险较大，先返回 `unsupported_protocol`，但保留配置默认模型与前端提示；后续批次补齐
- [ ] 测试请求体：

  ```json
  {
    "model": "<model_used>",
    "messages": [{"role": "user", "content": "Who are you?"}],
    "max_tokens": 20,
    "stream": false
  }
  ```

- [ ] 结果判定：
  - 2xx 且响应可解析：`healthy` 或 `degraded`
  - 响应耗时 `> degraded_threshold_ms`：`degraded`
  - 401 / 403：`auth_failed`
  - 404 或响应体包含模型不存在特征：`model_not_found`
  - 429：`rate_limited`
  - 5xx：`server_error`
  - 超时：`timeout`
  - 网络错误：`network_error`
  - 响应解析失败：`invalid_response`
- [ ] 每次测试后：
  - 写入 `model_test_logs`
  - 更新 `channels.health_status`
  - 更新 `channels.last_check`

##### D. 后端 HTTP 接口

- [ ] `POST /api/channels/:id/model-test`
  - 返回 `ModelTestResult`
- [ ] 保留旧接口 `POST /api/channels/:id/test`
  - 内部改为调用 `ModelTestService.TestChannel`
  - 响应保持旧格式兼容：`success/message`
- [ ] 第一批可暂不实现批量测试和日志查询接口，避免一次改动过大；但数据表先准备好

##### E. 前端 API 与类型

- [ ] 更新 `web/src/api/channels.ts`
  - `health_status` 类型兼容 `degraded`
  - 新增 `ModelTestResult`
  - 新增 `modelTestChannel(id)`
  - `testChannel(id)` 可改用 `/channels/:id/model-test` 或保留旧接口但使用新响应
- [ ] 新增 `web/src/api/model-test.ts`
  - `getModelTestConfig()`
  - `saveModelTestConfig(config)`

##### F. 前端渠道页最小更新

- [ ] `ChannelCard.vue`
  - 健康状态新增 `degraded` 黄色“降级”
  - 卡片展示最近测试摘要：模型、延迟、错误分类（第一批可从本次测试结果临时提示，持久展示后续通过日志接口补齐）
  - “测试”按钮 loading 状态由父组件传入，避免重复点击
- [ ] `Channels.vue`
  - `handleTest` 改用模型测试接口
  - 成功时显示：`健康/降级 + 延迟 + 模型`
  - 失败时显示错误分类与 message
  - 测试结束后重新加载渠道列表
- [ ] 在渠道页顶部增加“模型测试设置”按钮，打开一个简易弹窗：
  - timeout
  - max retries
  - degraded threshold
  - test prompt
  - max tokens
  - 默认模型 map 先用 JSON 文本编辑或按协议分组输入

##### G. 第一批验收用例

- [ ] `GET /api/model-test/config` 在无配置时返回默认值
- [ ] `PUT /api/model-test/config` 保存后再次读取一致
- [ ] 对 OpenAI 兼容渠道执行 `POST /api/channels/:id/model-test`：
  - API Key 错误返回 `auth_failed`
  - 模型不存在返回 `model_not_found` 或 `invalid_response`（根据上游响应）
  - 正常模型返回 `healthy` 或 `degraded`
- [ ] 执行测试后，渠道列表 `health_status` 和 `last_check` 更新
- [ ] 前端点击“测试”不会再只拉取 `/models`，而是展示真实模型测试结果
- [ ] 旧接口 `/api/channels/:id/test` 仍可被旧前端/脚本调用

#### 6.7.2 第二批：供应商编辑体验

- [ ] 增加供应商基础元数据字段：官网、备注、分类、图标
- [ ] 添加前端供应商预设选择器
- [ ] 编辑弹窗拆分为多个区域
- [ ] API Key 显示/隐藏和官网快捷链接
- [ ] 高级测试配置折叠面板

#### 6.7.3 第三批：模型拉取增强

- [ ] 支持 `models_url`、`is_full_url`、`model_fetch_mode`
- [ ] 后端实现候选 models URL 策略
- [ ] 前端模型输入框支持“获取模型 + 分组下拉选择”
- [ ] 模型拉取错误分类提示

#### 6.7.4 第四批：端点测速与自动选择

- [ ] 新增 `channel_endpoints` 表
- [ ] 增加端点管理接口
- [ ] 前端端点测速面板
- [ ] 自动选择最快端点
- [ ] 调度和模型测试使用默认端点

#### 6.7.5 第五批：真实健康检查替换

- [ ] 定时健康检查切换为模型测试
- [ ] 写入模型测试日志
- [ ] `degraded` 调度降权
- [ ] 批量测试全部供应商
- [ ] 测试日志保留策略

### 6.8 验收标准

- 新增供应商时，选择预设后能自动填入 Base URL、协议、默认测试模型和官网地址
- 模型拉取失败时能明确提示认证失败、接口不存在、超时或响应不兼容
- 单个供应商点击“测试”会发送真实模型请求，而不是只请求 `/models`
- 测试结果包含：状态、延迟、TTFB、模型名、HTTP 状态、错误分类、重试次数
- 延迟超过阈值时显示“降级”，但不直接标记不可用
- 供应商可单独覆盖测试模型、超时、prompt 和重试次数
- 批量测试不会被单个供应商失败阻塞
- 健康检查和调度能识别 `healthy/degraded/unhealthy`

---

## 发布计划

### v0.1.0 MVP（当前）
- ✅ 基础渠道管理
- ✅ OpenAI 兼容转发
- ✅ 优先级调度
- ✅ 前端原型

### v0.2.0（阶段一、二完成后）
- ✅ 流式响应
- ✅ 高级调度策略
- ✅ 健康检查
- ✅ 协议适配器

### v0.3.0（阶段三完成后）
- ✅ 完整管理后台
- ✅ 仪表盘、日志、密钥、模型管理

### v1.0.0（阶段四完成后）
- ✅ 生产就绪
- ✅ 性能优化
- ✅ 安全加固
- ✅ 完整文档

---

## 技术债务与已知问题

1. **前后端一体构建需要验证**：
   - 当前 GitHub Actions 会先构建前端，再将 `web/dist` 嵌入 Go 二进制
   - 需要通过 CI 验证单文件二进制能直接服务管理后台
   - 后续可继续优化二进制体积和构建缓存策略

2. **请求日志表可能快速膨胀**：
   - 高并发场景下 `request_logs` 表会很大
   - 建议定期归档或使用时序数据库（InfluxDB）

3. **流式响应错误处理不完善**：
   - 上游渠道中途断开时，需要优雅地关闭客户端连接

4. **没有单元测试**：
   - 当前只有手动集成测试
   - 建议为 Repository、Service、Scheduler 层添加单元测试

5. **模型名匹配逻辑简陋**：
   - 使用 SQL `LIKE` 模糊匹配 JSON 数组
   - 高频查询应建立单独的 `channel_models` 关联表

---

## 参考资源

- [NewAPI GitHub](https://github.com/QuantumNous/new-api)
- [CCSwitch GitHub](https://github.com/farion1231/cc-switch)
- [OpenAI API 文档](https://platform.openai.com/docs/api-reference)
- [Anthropic API 文档](https://docs.anthropic.com/claude/reference)
- [Gin 框架文档](https://gin-gonic.com/docs/)
- [Vue 3 文档](https://vuejs.org/)
- [Element Plus 文档](https://element-plus.org/)

---

## 贡献指南

欢迎提交 Issue 和 Pull Request。

开发前请：

1. Fork 本仓库
2. 创建特性分支：`git checkout -b feature/xxx`
3. 提交代码：`git commit -m "feat: add xxx"`
4. 推送分支：`git push origin feature/xxx`
5. 提交 PR

---

## 联系方式

如有问题或建议，请通过 GitHub Issues 联系。
