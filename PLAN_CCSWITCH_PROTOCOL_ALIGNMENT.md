# APIRelay 对齐 CC-Switch 本地路由协议模式计划

## 目标

根据 `ccswitch_本地路由分析.md`，将 APIRelay 当前的协议入口、协议模式、转发语义、故障转移和响应处理逐步调整到与 CC-Switch 本地路由一致。

本计划采用批次推进：每一批都应能独立验收，避免一次性大规模重构导致不可回归。

---

## 当前差距摘要

APIRelay 当前已经具备：

- OpenAI Chat Completions：`POST /v1/chat/completions`
- OpenAI Responses 桥接：`POST /v1/responses`
- Anthropic Messages：`POST /v1/messages`
- Gemini Native 基础入口：`/v1beta/models/*`
- OpenAI / Anthropic / Gemini 基础协议转换
- 多渠道调度与失败重试
- 请求日志

与 CC-Switch 相比，主要缺口包括：

1. 路由矩阵不完整：缺少 `/claude/*`、`/codex/*`、`/gemini/*`、`/v1/v1/*`、根级 `/responses` 和 `/chat/completions` 等兼容路径。
2. 协议模式缺少 app 维度：当前只有 `RelayMode + RelayFormat + APIType`，缺少类似 CC-Switch `AppType` 的 `claude / codex / gemini / claude_desktop` 语义。
3. Responses API 仍以 Chat 桥接为主，尚未成为 Codex 一等协议模式。
4. 流式响应缺少首包预检，一旦向客户端写入响应头就无法安全切换渠道。
5. 协议转换仍是最小文本模型，tool call、media、thinking、复杂 content block、usage 等容易丢失。
6. 缺少按 `app_type + channel/provider` 隔离的熔断器。
7. 响应处理、SSE 解析、usage 统计、解压、错误规范化仍较分散。
8. Gemini Native 路径支持不完整，缺少 `/gemini/v1beta/*`、`/gemini/v1/*`、`countTokens`、GET 透传等。
9. Claude Desktop 3P Gateway 尚未实现。

---

## 批次总览

| 批次 | 名称 | 目标 | 优先级 |
|---|---|---|---|
| 第一批 | 路由矩阵与协议模式命名对齐 | 先让 CC-Switch 兼容路径进入正确 handler，并在日志/上下文中区分 app 类型 | P0 |
| 第二批 | RequestContext 与 Forwarder 抽象 | 统一 JSON / Stream / Responses / Gemini 的转发入口，减少重复逻辑 | P0 |
| 第三批 | Codex Responses 一等协议支持 | Responses native/chat bridge 策略、非流式 SSE 聚合、Codex 错误格式 | P0 |
| 第四批 | Gemini Native 完整路径支持 | 对齐 Gemini CLI 路由、URL 构建、GET/models/countTokens/alt=sse | P1 |
| 第五批 | 流式首包预检与故障转移 | 流式首包失败可切换渠道，开始输出后不再错误切换 | P1 |
| 第六批 | app 维度熔断器 | 按 `relay_app:channel_id` 维护 Closed/Open/HalfOpen 状态 | P1 |
| 第七批 | 富协议转换 | 支持 tool calls、media、thinking、复杂 content blocks、usage 保真 | P2 |
| 第八批 | 统一响应处理与 UsageLog | 解压、SSE parser、usage 解析、成本统计基础 | P2 |
| 第九批 | Claude Desktop Gateway | 增加 `/claude-desktop/v1/*` 独立命名空间和 token 认证 | P3 |
| 第十批 | 高级整流与优化 | media fallback、thinking rectifier、413 特化、Copilot/Bedrock 优化 | P3 |

---

# 第一批：路由矩阵与协议模式命名对齐

## 批次目标

第一批只做“入口与语义对齐”，不进行大规模转发重构。

完成后应满足：

1. CC-Switch 文档中核心本地路由路径在 APIRelay 中不再返回 404。
2. 每个入口能明确映射到 `RelayApp + RelayMode + RelayFormat`。
3. 请求日志能记录 `relay_app`，为后续熔断器、usage、统计按 app 隔离打基础。
4. 原有 `/v1/*` 兼容 API 行为不被破坏。

## 第一批范围

### 纳入范围

- 新增 `RelayApp` 常量。
- 扩展 `RelayMode` 常量。
- 扩展 `RelayInfo` 和 `RequestLog` 以记录 `relay_app`。
- 补齐路由注册。
- 新增 wrapper handler，将不同路径映射到正确协议模式。
- 更新 SPA fallback 的 API 路径识别。
- 补充基础测试或手动验收命令。

### 暂不纳入

- 不重构 Forwarder。
- 不实现流式首包预检。
- 不新增熔断器。
- 不实现 Responses native 上游策略。
- 不实现 Claude Desktop Gateway token 认证。
- 不实现富协议模型。
- 不实现完整 Gemini `countTokens` 转换逻辑。

---

## 第一批详细任务

## 1. 常量与类型更新

### 1.1 新增 `RelayApp`

修改文件：

```text
internal/relay/constant/relay_app.go
```

新增文件内容建议：

```go
package constant

// RelayApp 表示当前请求来源客户端/协议命名空间。
type RelayApp string

const (
    RelayAppOpenAI        RelayApp = "openai"
    RelayAppClaude        RelayApp = "claude"
    RelayAppCodex         RelayApp = "codex"
    RelayAppGemini        RelayApp = "gemini"
    RelayAppClaudeDesktop RelayApp = "claude_desktop"
)

func (a RelayApp) String() string {
    return string(a)
}
```

说明：

- `RelayAppOpenAI` 用于保留当前标准 `/v1/chat/completions`、`/v1/completions`、`/v1/embeddings` 等传统 OpenAI 兼容入口。
- `RelayAppCodex` 用于 `/codex/*`、根级 `/responses`、根级 `/chat/completions`、双 `/v1/v1/*` 等 Codex CLI 兼容路径。
- `RelayAppClaude` 用于 `/v1/messages` 和 `/claude/v1/messages`。
- `RelayAppGemini` 用于 `/v1beta/*`、`/gemini/v1beta/*`、`/gemini/v1/*`。
- `RelayAppClaudeDesktop` 第一批只预留，后续第九批实现。

### 1.2 扩展 `RelayMode`

修改文件：

```text
internal/relay/constant/relay_mode.go
```

目标常量：

```go
const (
    RelayModeMessages         RelayMode = "messages"
    RelayModeChatCompletions  RelayMode = "chat_completions"
    RelayModeResponses        RelayMode = "responses"
    RelayModeResponsesCompact RelayMode = "responses_compact"
    RelayModeCompletions      RelayMode = "completions"
    RelayModeEmbeddings       RelayMode = "embeddings"
    RelayModeGeminiNative     RelayMode = "gemini_native"
    RelayModeModels           RelayMode = "models"
    RelayModeCountTokens      RelayMode = "count_tokens"
)
```

第一批实际使用：

- `messages`
- `chat_completions`
- `responses`
- `responses_compact`
- `completions`
- `embeddings`
- `gemini_native`
- `models`

`count_tokens` 可先预留。

---

## 2. RelayInfo 与日志模型更新

### 2.1 扩展 `RelayInfo`

修改文件：

```text
internal/relay/relayinfo/relay_info.go
```

新增字段：

```go
RelayApp constant.RelayApp
OriginalPath string
Endpoint string
Query string
```

第一批要求：

- `RelayApp` 必须写入。
- `OriginalPath` 记录 `c.Request.URL.Path`。
- `Query` 记录 `c.Request.URL.RawQuery`。
- `Endpoint` 第一批可与 `OriginalPath` 相同，后续 Forwarder 重构再标准化。

### 2.2 扩展 `RequestLog`

修改文件：

```text
internal/model/models.go
```

在 `RequestLog` 中新增：

```go
RelayApp string `json:"relay_app" gorm:"size:50;index"`
```

同时修改：

```text
internal/model/db.go
```

确认 `AutoMigrate` 会自动迁移新增字段。

### 2.3 更新日志写入

修改文件：

```text
internal/relay/controller/log.go
```

`RequestLog` 写入时增加：

```go
RelayApp: string(info.RelayApp),
```

控制台日志格式增加：

```text
relay_app=%s
```

`logNoChannel` 也需要接收并写入 `RelayApp`。

---

## 3. 控制器入口更新

### 3.1 修改 `handleRelay` 签名

修改文件：

```text
internal/relay/controller/relay_common.go
```

当前：

```go
func (rc *RelayController) handleRelay(c *gin.Context, mode constant.RelayMode, format constant.RelayFormat)
```

改为：

```go
func (rc *RelayController) handleRelay(c *gin.Context, app constant.RelayApp, mode constant.RelayMode, format constant.RelayFormat)
```

所有 `logNoChannel`、`buildRelayInfo` 调用同步传入 app。

### 3.2 修改 `buildRelayInfo`

当前：

```go
func buildRelayInfo(c *gin.Context, requestID string, startTime time.Time, mode constant.RelayMode, format constant.RelayFormat, meta relayRequestMeta, candidate relayCandidate, isStream bool) *relayinfo.RelayInfo
```

改为：

```go
func buildRelayInfo(c *gin.Context, requestID string, startTime time.Time, app constant.RelayApp, mode constant.RelayMode, format constant.RelayFormat, meta relayRequestMeta, candidate relayCandidate, isStream bool) *relayinfo.RelayInfo
```

写入：

```go
RelayApp: app,
OriginalPath: c.Request.URL.Path,
Endpoint: c.Request.URL.Path,
Query: c.Request.URL.RawQuery,
```

### 3.3 更新现有入口

修改文件：

```text
internal/relay/controller/chat.go
internal/relay/controller/native.go
internal/relay/controller/responses.go
internal/relay/controller/completions.go
internal/relay/controller/embeddings.go
```

目标：

```go
func (rc *RelayController) ChatCompletions(c *gin.Context) {
    rc.handleRelay(c, constant.RelayAppOpenAI, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI)
}

func (rc *RelayController) AnthropicMessages(c *gin.Context) {
    rc.handleRelay(c, constant.RelayAppClaude, constant.RelayModeMessages, constant.RelayFormatAnthropic)
}

func (rc *RelayController) GeminiGenerateContent(c *gin.Context) {
    rc.handleRelay(c, constant.RelayAppGemini, constant.RelayModeGeminiNative, constant.RelayFormatGemini)
}
```

如果当前 Anthropic adaptor 暂时只支持 `RelayModeChatCompletions`，第一批可以采取兼容策略：

- 对外日志记录 `RelayModeMessages`；
- 内部转换仍临时走 `RelayModeChatCompletions`；
- 或者第一批先让 `RelayModeMessages` 在 adaptor 内被当作 ChatCompletions 处理。

推荐第一批采用第二种：在 adaptor 中将 `RelayModeMessages` 视作聊天能力，减少语义偏差。

---

## 4. 新增 wrapper handler

### 4.1 Claude wrapper

修改文件：

```text
internal/relay/controller/native.go
```

新增：

```go
func (rc *RelayController) ClaudeMessages(c *gin.Context) {
    rc.handleRelay(c, constant.RelayAppClaude, constant.RelayModeMessages, constant.RelayFormatAnthropic)
}
```

`AnthropicMessages` 可继续保留，内部同样使用 `RelayAppClaude`。

### 4.2 Codex Chat wrapper

修改文件：

```text
internal/relay/controller/chat.go
```

新增：

```go
func (rc *RelayController) CodexChatCompletions(c *gin.Context) {
    rc.handleRelay(c, constant.RelayAppCodex, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI)
}
```

### 4.3 Codex Responses wrapper

修改文件：

```text
internal/relay/controller/responses.go
```

新增：

```go
func (rc *RelayController) CodexResponses(c *gin.Context) {
    rc.handleResponsesBridgeWithApp(c, constant.RelayAppCodex)
}
```

当前 `Responses` 是：

```go
func (rc *RelayController) Responses(c *gin.Context) {
    rc.handleResponsesBridge(c)
}
```

建议第一批改成：

```go
func (rc *RelayController) Responses(c *gin.Context) {
    rc.handleResponsesBridgeWithApp(c, constant.RelayAppOpenAI)
}
```

并将 `handleResponsesBridge` 改造为：

```go
func (rc *RelayController) handleResponsesBridgeWithApp(c *gin.Context, app constant.RelayApp)
```

内部 `logNoChannel`、`buildRelayInfo` 使用传入 app。

### 4.4 Responses Compact wrapper

新增：

```go
func (rc *RelayController) ResponsesCompact(c *gin.Context) {
    writeRelayError(c, http.StatusNotImplemented, "responses compact is not supported yet", "unsupported_relay_mode", "")
}
```

第一批先返回明确 JSON，而不是 404。

后续第三批再决定是否实现真实 compact 行为。

### 4.5 Gemini Native wrapper

修改文件：

```text
internal/relay/controller/native.go
```

新增：

```go
func (rc *RelayController) GeminiNative(c *gin.Context) {
    rc.handleRelay(c, constant.RelayAppGemini, constant.RelayModeGeminiNative, constant.RelayFormatGemini)
}
```

第一批可以复用当前 `GeminiGenerateContent` 的逻辑，只要路径能进入 handler。

注意：当前 `parseGeminiRequestMeta` 依赖 `c.Param("modelAction")`。新增通配路由参数可能叫 `path`，需要第一批兼容：

```go
modelAction := c.Param("modelAction")
if modelAction == "" {
    modelAction = c.Param("path")
}
```

---

## 5. 路由注册更新

修改文件：

```text
internal/api/router.go
```

### 5.1 OpenAI / Codex models

保留：

```go
v1Group.GET("/models", relayController.GetModels)
v1Group.GET("/models/:model", relayController.GetModel)
```

新增根级：

```go
r.GET("/models", auth, relayController.GetModels)
```

### 5.2 Claude 路由

保留：

```go
POST /v1/messages
```

新增：

```go
r.POST("/claude/v1/messages", auth, relayController.ClaudeMessages)
```

### 5.3 Codex Chat 路由

新增：

```go
r.POST("/chat/completions", auth, relayController.CodexChatCompletions)
r.POST("/v1/v1/chat/completions", auth, relayController.CodexChatCompletions)
r.POST("/codex/v1/chat/completions", auth, relayController.CodexChatCompletions)
```

保留：

```go
POST /v1/chat/completions -> relayController.ChatCompletions
```

也可以选择将 `/v1/chat/completions` 视作 Codex，但为了兼容现有 OpenAI 语义，第一批建议保留 `RelayAppOpenAI`。

### 5.4 Codex Responses 路由

新增：

```go
r.POST("/responses", auth, relayController.CodexResponses)
r.POST("/v1/v1/responses", auth, relayController.CodexResponses)
r.POST("/codex/v1/responses", auth, relayController.CodexResponses)
```

保留：

```go
POST /v1/responses -> relayController.Responses
```

### 5.5 Responses Compact 路由

新增：

```go
r.POST("/responses/compact", auth, relayController.ResponsesCompact)
r.POST("/v1/responses/compact", auth, relayController.ResponsesCompact)
r.POST("/v1/v1/responses/compact", auth, relayController.ResponsesCompact)
```

注意：在 Gin 中，具体路由需要在可能冲突的通配路由之前注册。

### 5.6 Gemini 路由

保留现有：

```go
/v1beta/models
/v1beta/models/*modelPath
/v1beta/models/*modelAction
```

新增：

```go
r.Any("/gemini/v1beta/*path", auth, relayController.GeminiNative)
r.Any("/gemini/v1/*path", auth, relayController.GeminiNative)
```

如果要进一步贴近 CC-Switch，可将 `/v1beta/*path` 也统一到 `GeminiNative`。第一批为降低风险，可先保留现有 `/v1beta` group，同时让新增 `/gemini/*` 走 wrapper。

---

## 6. API fallback 更新

修改文件：

```text
internal/api/router.go
```

更新 `isAPIRoute`：

```go
func isAPIRoute(path string) bool {
    return path == "/api" || strings.HasPrefix(path, "/api/") ||
        path == "/v1" || strings.HasPrefix(path, "/v1/") ||
        path == "/v1beta" || strings.HasPrefix(path, "/v1beta/") ||
        path == "/models" || strings.HasPrefix(path, "/models/") ||
        path == "/chat" || strings.HasPrefix(path, "/chat/") ||
        path == "/responses" || strings.HasPrefix(path, "/responses/") ||
        path == "/codex" || strings.HasPrefix(path, "/codex/") ||
        path == "/claude" || strings.HasPrefix(path, "/claude/") ||
        path == "/gemini" || strings.HasPrefix(path, "/gemini/") ||
        path == "/claude-desktop" || strings.HasPrefix(path, "/claude-desktop/") ||
        path == "/health" || path == "/status"
}
```

---

## 7. Adaptor 兼容 `RelayModeMessages` 与 `RelayModeGeminiNative`

当前 adaptor 多处判断：

```go
if mode != RelayModeChatCompletions { ... }
```

第一批需要避免新 mode 进入后被误判 unsupported。

### 7.1 新增 helper

可在 `internal/relay/constant/relay_mode.go` 增加：

```go
func (m RelayMode) IsChatLike() bool {
    switch m {
    case RelayModeChatCompletions, RelayModeMessages, RelayModeGeminiNative:
        return true
    default:
        return false
    }
}
```

然后将 adaptor 中相关判断改成：

```go
if !mode.IsChatLike() { ... }
```

第一批重点文件：

```text
internal/relay/adaptor/openai/adaptor.go
internal/relay/adaptor/anthropic/adaptor.go
internal/relay/adaptor/gemini/adaptor.go
```

### 7.2 URL path 映射

OpenAI adaptor 的 `modePath` 中：

- `RelayModeMessages` 对 OpenAI 上游应映射 `/chat/completions`。
- `RelayModeGeminiNative` 对 OpenAI 上游第一批仍映射 `/chat/completions`。
- `RelayModeResponsesCompact` 第一批不经过 adaptor。

Anthropic adaptor：

- `RelayModeMessages` 映射 `/messages`。
- `RelayModeChatCompletions` 仍可映射 `/messages`。

Gemini adaptor：

- `RelayModeGeminiNative` 使用模型 path 构建 generateContent / streamGenerateContent。
- 第一批可继续复用现有 `GetRequestURLWithModel`。

---

## 8. 测试与验收

## 8.1 编译测试

```bash
go test ./...
```

如前端类型涉及日志字段，补充：

```bash
npm --prefix web run build
```

## 8.2 路由不返回 404

启动服务后，使用无效 body 或无效 key 时可以返回 401/400，但不能是 Gin 404 或 SPA HTML。

建议手动检查：

```bash
curl -i http://localhost:15722/models \
  -H "Authorization: Bearer $API_KEY"

curl -i -X POST http://localhost:15722/claude/v1/messages \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{}'

curl -i -X POST http://localhost:15722/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{}'

curl -i -X POST http://localhost:15722/v1/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{}'

curl -i -X POST http://localhost:15722/codex/v1/responses \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{}'

curl -i -X POST http://localhost:15722/responses/compact \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{}'

curl -i http://localhost:15722/gemini/v1beta/models \
  -H "Authorization: Bearer $API_KEY"
```

## 8.3 日志字段验收

通过任一 relay 请求后，查询日志：

```bash
curl http://localhost:15722/api/logs \
  -H "Authorization: Bearer $ADMIN_KEY"
```

验收：

- `/claude/v1/messages` 记录 `relay_app=claude`
- `/chat/completions` 记录 `relay_app=codex`
- `/codex/v1/responses` 记录 `relay_app=codex`
- `/v1/chat/completions` 记录 `relay_app=openai`
- `/gemini/v1beta/*` 记录 `relay_app=gemini`

## 8.4 原有接口回归

必须保持可用：

```http
GET  /api/system/health
GET  /v1/models
POST /v1/chat/completions
POST /v1/messages
POST /v1/responses
GET  /v1beta/models
POST /v1beta/models/{model}:generateContent
```

---

## 第一批交付物

完成第一批后，应产生以下变更：

1. 新增：
   - `internal/relay/constant/relay_app.go`
2. 修改：
   - `internal/relay/constant/relay_mode.go`
   - `internal/relay/relayinfo/relay_info.go`
   - `internal/model/models.go`
   - `internal/relay/controller/log.go`
   - `internal/relay/controller/relay_common.go`
   - `internal/relay/controller/chat.go`
   - `internal/relay/controller/responses.go`
   - `internal/relay/controller/responses_bridge.go`
   - `internal/relay/controller/native.go`
   - `internal/relay/adaptor/openai/adaptor.go`
   - `internal/relay/adaptor/anthropic/adaptor.go`
   - `internal/relay/adaptor/gemini/adaptor.go`
   - `internal/api/router.go`
3. 运行：
   - `go test ./...`
4. 更新或确认：
   - README 可后续批次再同步，不阻塞第一批。

---

# 第二批：RequestContext 与 Forwarder 抽象

## 目标

统一当前散落在 `relayJSON`、`relayStream`、`responses_bridge`、`GeminiGenerateContent` 中的逻辑，为后续首包预检、熔断器和统一响应处理打基础。

## 主要任务

- 新增 `RequestContext`。
- 新增 `RelayRequest` / `RelayResponse`。
- 抽出候选渠道解析逻辑。
- 抽出上游请求构建逻辑。
- 抽出错误分类逻辑。
- `relayJSON`、`relayStream` 改为调用统一 Forwarder。
- 保持现有行为不变。

## 验收

- 第一批所有路由仍可用。
- JSON 非流式、SSE 流式、Responses bridge 行为与当前一致。
- 代码中不再重复多份候选渠道循环逻辑。

---

# 第三批：Codex Responses 一等协议支持

## 目标

让 `/responses`、`/v1/responses`、`/codex/v1/responses` 更接近 CC-Switch 的 Codex Responses 行为。

## 主要任务

- 为渠道增加 `supports_responses` / `responses_mode` 配置。
- 支持 `native`、`chat_bridge`、`auto` 三种上游模式。
- 非流式请求如果上游返回 SSE，则聚合成 Responses JSON。
- Responses SSE 输出结构补齐。
- Codex 错误格式规范化。
- 413 错误提示上游网关限制。
- `/responses/compact` 从 501 升级为兼容实现或明确指引。

## 验收

- Codex CLI 对 `/v1/responses` 不再出现 “No Responses API events were parsed”。
- `Accept: text/event-stream` 返回 Responses SSE。
- `stream:false` 返回 JSON，即使上游强制 SSE。

---

# 第四批：Gemini Native 完整路径支持

## 目标

对齐 CC-Switch 的 Gemini 本地路由，支持 Gemini CLI 常用路径。

## 主要任务

- 统一 `ANY /v1beta/*path`、`ANY /gemini/v1beta/*path`、`ANY /gemini/v1/*path`。
- 支持 GET `/models` 和 GET `/models/{model}`。
- 支持 POST `:generateContent`。
- 支持 POST `:streamGenerateContent?alt=sse`。
- 支持 POST `:countTokens`。
- Gemini URL 构建策略重写。
- 支持 `x-goog-api-key` 与 OAuth Bearer。

## 验收

- Gemini CLI 风格请求可通过 `/v1beta` 或 `/gemini/v1beta` 工作。
- `alt=sse` 保留。
- `models/gemini-xxx` 与 `gemini-xxx` 都能识别。

---

# 第五批：流式首包预检与故障转移

## 目标

流式请求在真正开始向客户端输出前，能够验证上游首包是否到达；首包失败时可以切换下一个渠道。

## 主要任务

- 新增 `PrepareStreamResponse`。
- 支持 first byte timeout。
- 支持 idle timeout。
- 首包成功后才写客户端 SSE headers。
- 首包失败时关闭上游 body 并尝试下一个渠道。
- 开始输出后不再切换，只记录流中断。

## 验收

- 第一个渠道返回 200 但无数据时，可以切换第二个渠道。
- 第一个渠道连接失败时，可以切换第二个渠道。
- 客户端已收到数据后，上游中断不会尝试写新的 JSON 错误。

---

# 第六批：app 维度熔断器

## 目标

按 `relay_app:channel_id` 隔离熔断状态，避免一个客户端协议的失败影响另一个协议。

## 主要任务

- 新增 CircuitBreaker。
- 实现 Closed / Open / HalfOpen。
- 支持 failure threshold。
- 支持 success threshold。
- 支持 open timeout。
- 支持 error rate threshold。
- 调度时跳过 Open 状态渠道。
- HalfOpen 只允许有限探测。
- 请求结果同步更新渠道健康状态。

## 验收

- Claude 下某渠道熔断，不影响 Codex 下同渠道尝试。
- 连续失败后进入 Open。
- 超时后进入 HalfOpen。
- 探测成功后恢复 Closed。

---

# 第七批：富协议转换

## 目标

减少 Anthropic / OpenAI / Responses / Gemini 互转时的信息丢失。

## 主要任务

- 新增 RichChatRequest / RichMessage / ContentBlock / ToolCall。
- Anthropic content blocks 完整解析。
- OpenAI tool_calls / tool message 支持。
- Responses input/output item 支持。
- Gemini parts 支持 text / inline_data。
- usage 字段保真。
- Shadow Store / Chat History Store 预研和实现。

## 验收

- Claude Code tool_use 可转换到 OpenAI tool_calls。
- OpenAI tool_calls 可转换到 Anthropic tool_use。
- Gemini 多 part 文本不丢失。
- usage 基础字段转换后可保留。

---

# 第八批：统一响应处理与 UsageLog

## 目标

对齐 CC-Switch 的响应处理流程：解压、SSE 解析、usage 统计、日志和成本计算基础。

## 主要任务

- 新增 response processor。
- 非流式响应统一读取、解压、header 清理。
- 流式响应统一 SSE parser。
- 支持 Claude / OpenAI / Responses / Gemini usage 解析。
- 新增 `usage_logs` 表。
- 记录 first_token_ms。
- 成本计算可先预留字段，后续接模型价格。

## 验收

- 非流式请求记录 input/output/total tokens。
- 流式请求结束后记录 usage。
- gzip 响应可正确处理。
- SSE 多行 data 可正确解析。

---

# 第九批：Claude Desktop Gateway

## 目标

补齐 CC-Switch 的 Claude Desktop 3P Gateway 独立 namespace。

## 主要任务

- 新增 `GET /claude-desktop/v1/models`。
- 新增 `POST /claude-desktop/v1/messages`。
- 新增 gateway token 配置。
- 独立 `RelayAppClaudeDesktop`。
- 日志、熔断器、usage 与 Claude Code 隔离。

## 验收

- 无 token 返回 401。
- token 正确可访问 models/messages。
- 日志记录 `relay_app=claude_desktop`。

---

# 第十批：高级整流与优化

## 目标

逐步补齐 CC-Switch 针对复杂客户端和特殊供应商的稳定性增强。

## 主要任务

- Media fallback。
- Thinking signature strip。
- Thinking budget strip。
- Bedrock prompt cache injection。
- Copilot tool result merge。
- 413 上游限制错误特化。
- 请求日志敏感字段脱敏。

## 验收

- 不支持图片的模型可以自动降级。
- Bedrock thinking signature 不再导致 400。
- 413 错误明确提示上游限制。
- 日志不泄露 API Key。

---

## 总体验收标准

全部批次完成后，APIRelay 应具备以下 CC-Switch 对齐能力：

1. 支持 Claude / Codex / Gemini 三类本地路由入口。
2. 支持 `/claude`、`/codex`、`/gemini` namespace 与常见无前缀兼容路径。
3. 支持 OpenAI / Anthropic / Gemini / Responses 多向转换。
4. 支持 Codex Responses 流式与非流式正确返回。
5. 支持 Gemini Native GET/POST/stream/countTokens 常用路径。
6. 支持流式首包预检与安全故障转移。
7. 支持 app 维度熔断器。
8. 支持 usage 统计和流式 first token 延迟。
9. 复杂协议转换尽量保留 tool/media/thinking/usage 信息。
10. 保持原有 APIRelay 管理 API、渠道管理和前端功能可用。
