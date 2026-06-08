# CC-Switch 本地路由请求处理完整分析

## 目录

1. [架构概览](#架构概览)
2. [本地路由请求处理流程](#本地路由请求处理流程)
3. [支持的 API 协议](#支持的-api-协议)
4. [核心模块详解](#核心模块详解)
5. [请求转发与故障转移](#请求转发与故障转移)
6. [响应处理流程](#响应处理流程)
7. [格式转换机制](#格式转换机制)
8. [熔断器与健康检查](#熔断器与健康检查)
9. [使用量统计](#使用量统计)
10. [完整实现清单](#完整实现清单)

---

## 架构概览

CC-Switch 是一个基于 Tauri 2 + Rust 的本地 HTTP 代理服务器，为 Claude Code、Codex、Gemini CLI 等 AI 编程工具提供统一的 Provider 管理和故障转移能力。

### 核心设计原则

1. **单一数据源（SSOT）**：所有配置存储在 SQLite 数据库（`~/.cc-switch/cc-switch.db`）
2. **双向同步**：切换 Provider 时写入 Live 配置文件，编辑活动 Provider 时回填数据库
3. **原子写入**：使用临时文件 + rename 模式防止配置损坏
4. **并发安全**：Mutex 保护数据库连接，避免竞态条件
5. **分层架构**：清晰的 Commands → Services → DAO → Database 分层

### 技术栈

- **后端框架**：Tauri 2.8 + Rust
- **HTTP 服务器**：Axum + Hyper（手动 HTTP/1.1 accept loop，支持 preserve_header_case）
- **数据库**：SQLite（rusqlite）
- **并发**：Tokio（async runtime）
- **HTTP 客户端**：Reqwest（禁用自动解压以透传 Accept-Encoding）

---

## 本地路由请求处理流程

### 整体流程图

```
客户端请求 → Axum Router → Handler → RequestContext → Forwarder
                                                          ↓
响应 ← Response Processor ← 格式转换（可选） ← Provider Router
```

### 详细处理步骤

#### 第一步：TCP 连接接受（`server.rs`）

```rust
// 使用手动 hyper HTTP/1.1 accept loop
loop {
    tokio::select! {
        result = listener.accept() => {
            let (stream, _remote_addr) = result?;
            
            // Peek 原始 TCP 字节以捕获客户端请求头的原始大小写
            let original_cases = {
                let mut peek_buf = vec![0u8; 8192];
                match stream.peek(&mut peek_buf).await {
                    Ok(n) => OriginalHeaderCases::from_raw_bytes(&peek_buf[..n]),
                    Err(e) => OriginalHeaderCases::default(),
                }
            };
            
            // 启用 preserve_header_case(true) 保留客户端请求头大小写
            hyper::server::conn::http1::Builder::new()
                .preserve_header_case(true)
                .serve_connection(TokioIo::new(stream), service)
                .await
        }
        _ = &mut shutdown_rx => break,
    }
}
```

**关键点**：
- 在 Hyper 解析前 peek TCP 字节流，提取原始 header 大小写
- 通过 `extensions.insert(original_cases)` 传递给下游
- 转发给上游时使用 `hyper-util` 的 `HeaderCaseMap` 恢复原始大小写

#### 第二步：路由匹配（`server.rs::build_router`）

```rust
Router::new()
    // Claude API
    .route("/v1/messages", post(handlers::handle_messages))
    .route("/claude/v1/messages", post(handlers::handle_messages))
    
    // Claude Desktop 3P Gateway（独立 namespace）
    .route("/claude-desktop/v1/models", get(handlers::handle_claude_desktop_models))
    .route("/claude-desktop/v1/messages", post(handlers::handle_claude_desktop_messages))
    
    // Codex API
    .route("/chat/completions", post(handlers::handle_chat_completions))
    .route("/v1/chat/completions", post(handlers::handle_chat_completions))
    .route("/v1/v1/chat/completions", post(handlers::handle_chat_completions)) // 兼容双前缀
    .route("/codex/v1/chat/completions", post(handlers::handle_chat_completions))
    
    // Codex Responses API
    .route("/responses", post(handlers::handle_responses))
    .route("/v1/responses", post(handlers::handle_responses))
    .route("/v1/v1/responses", post(handlers::handle_responses))
    .route("/codex/v1/responses", post(handlers::handle_responses))
    
    // Codex Responses Compact API（远程压缩）
    .route("/responses/compact", post(handlers::handle_responses_compact))
    .route("/v1/responses/compact", post(handlers::handle_responses_compact))
    .route("/v1/v1/responses/compact", post(handlers::handle_responses_compact))
    
    // Gemini API（使用 any() 覆盖所有 HTTP 方法，包括 GET /models）
    .route("/v1beta/*path", any(handlers::handle_gemini))
    .route("/gemini/v1beta/*path", any(handlers::handle_gemini))
    .route("/gemini/v1/*path", any(handlers::handle_gemini))
    
    // 健康检查
    .route("/health", get(handlers::health_check))
    .route("/status", get(handlers::get_status))
    
    .layer(DefaultBodyLimit::max(200 * 1024 * 1024)) // 200MB
    .with_state(state)
```

#### 第三步：Handler 处理（`handlers.rs`）

```rust
pub async fn handle_messages(
    State(state): State<ProxyState>,
    request: axum::extract::Request,
) -> Result<axum::response::Response, ProxyError> {
    // 1. 解析请求
    let (parts, req_body) = request.into_parts();
    let body_bytes = req_body.collect().await?.to_bytes();
    let body: Value = serde_json::from_slice(&body_bytes)?;
    
    // 2. 创建请求上下文
    let mut ctx = RequestContext::new(
        &state, &body, &headers, 
        AppType::Claude, "Claude", "claude"
    ).await?;
    
    // 3. 提取 endpoint
    let endpoint = strip_prefix(&uri.path(), "/claude");
    
    // 4. 判断是否流式
    let is_stream = body.get("stream").and_then(|s| s.as_bool()).unwrap_or(false);
    
    // 5. 创建 Forwarder 并转发
    let forwarder = ctx.create_forwarder(&state);
    let mut result = forwarder.forward_with_retry(
        &AppType::Claude, method, endpoint, body.clone(),
        headers, extensions, ctx.get_providers()
    ).await?;
    
    // 6. 处理响应（可能需要格式转换）
    let adapter = get_adapter(&AppType::Claude);
    if adapter.needs_transform(&ctx.provider) {
        return handle_claude_transform(response, &ctx, &state, ...).await;
    }
    
    // 7. 通用响应处理（透传模式）
    process_response(response, &ctx, &state, &CLAUDE_PARSER_CONFIG, connection_guard).await
}
```

---

## 支持的 API 协议

### 1. Claude API（Anthropic Messages API）

#### 端点

```
POST /v1/messages
POST /claude/v1/messages
```

#### 请求格式

```json
{
  "model": "claude-sonnet-4",
  "messages": [
    {
      "role": "user",
      "content": "Hello"
    }
  ],
  "max_tokens": 1024,
  "stream": false,
  "temperature": 1.0,
  "system": "You are a helpful assistant"
}
```

#### 认证方式

1. **Anthropic 官方**：
   ```
   x-api-key: sk-ant-xxxxx
   anthropic-version: 2023-06-01
   ```

2. **中转服务（bearer_only）**：
   ```
   Authorization: Bearer xxxxx
   ```

3. **GitHub Copilot**：需要格式转换（见下文）

#### 响应格式

**非流式**：
```json
{
  "id": "msg_01...",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hello! How can I help you today?"
    }
  ],
  "model": "claude-sonnet-4",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 10,
    "output_tokens": 25
  }
}
```

**流式**（SSE）：
```
event: message_start
data: {"type":"message_start","message":{"id":"msg_01...","type":"message","role":"assistant"}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":25}}

event: message_stop
data: {"type":"message_stop"}
```

#### 特殊处理

1. **Beta 参数剥离**：查询参数中的 `beta=true` 在转换格式时会被剥离
2. **Thinking 优化**：Bedrock Provider 支持 thinking 块优化和 prompt cache 注入
3. **Media 整流**：不支持图片的模型会自动降级为文本标记

### 2. Codex API（OpenAI 兼容）

#### 支持的端点

```
POST /chat/completions
POST /v1/chat/completions
POST /v1/v1/chat/completions  # 兼容双前缀
POST /codex/v1/chat/completions

POST /responses
POST /v1/responses
POST /v1/v1/responses
POST /codex/v1/responses

POST /responses/compact
POST /v1/responses/compact
POST /v1/v1/responses/compact
POST /codex/v1/responses/compact

GET /models
GET /v1/models
```

#### Chat Completions 请求格式

```json
{
  "model": "gpt-5.4",
  "messages": [
    {
      "role": "user",
      "content": "Hello"
    }
  ],
  "stream": false,
  "temperature": 1.0
}
```

#### Responses API 请求格式

```json
{
  "model": "gpt-5.4",
  "modalities": ["text"],
  "instructions": "You are a helpful assistant",
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": [
        {
          "type": "input_text",
          "text": "Hello"
        }
      ]
    }
  ],
  "stream": false
}
```

#### Chat Completions 响应格式

**非流式**：
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "gpt-5.4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 8,
    "total_tokens": 18
  }
}
```

**流式**（SSE）：
```
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,"model":"gpt-5.4","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,"model":"gpt-5.4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,"model":"gpt-5.4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

#### Responses API 响应格式

**非流式**：
```json
{
  "id": "resp_xxx",
  "object": "realtime.response",
  "status": "completed",
  "model": "gpt-5.4",
  "output": [
    {
      "type": "message",
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "Hello! How can I help you?"
        }
      ]
    }
  ],
  "usage": {
    "input_tokens": 10,
    "output_tokens": 8,
    "total_tokens": 18
  }
}
```

**流式**（SSE）：
```
event: response.output_item.done
data: {"type":"response.output_item.done","item":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello"}]}}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_xxx","status":"completed","model":"gpt-5.4","usage":{"input_tokens":10,"output_tokens":8}}}
```

#### 特殊处理

1. **Chat → Responses 转换**：某些 Provider（如 MiniMax）使用 Chat Completions 上游，但本地代理会转换为 Responses 格式
2. **Tool Call 历史恢复**：`CodexChatHistoryStore` 保存最近的 tool call，用于恢复 `previous_response_id` 引用
3. **错误格式规整**：Chat 错误响应会被转换为 Responses 错误格式（`{"error": {message, type, code, param}}`）
4. **413 特殊处理**：上游 413 Payload Too Large 会被特别标注为上游网关限制，而非本地代理限制
5. **Codex OAuth 强制 SSE**：ChatGPT Plus OAuth 会将 `stream:false` 强制升级为 SSE，本地代理聚合后返回 JSON

### 3. Gemini API

#### 端点

```
GET  /v1beta/models
GET  /v1beta/models/{model}
POST /v1beta/models/{model}:generateContent
POST /v1beta/models/{model}:streamGenerateContent?alt=sse
POST /v1beta/models/{model}:countTokens

# 兼容带前缀的形式
GET  /gemini/v1beta/models
POST /gemini/v1beta/models/{model}:generateContent
POST /gemini/v1/*path
```

#### 请求格式

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {
          "text": "Hello"
        }
      ]
    }
  ],
  "generationConfig": {
    "temperature": 1.0,
    "maxOutputTokens": 1024
  }
}
```

#### 认证方式

1. **API Key**：
   ```
   x-goog-api-key: AIzaSyXXXXX
   或查询参数: ?key=AIzaSyXXXXX
   ```

2. **OAuth**（CLI 模式）：
   ```
   Authorization: Bearer ya29.xxxxx
   ```

#### 响应格式

**非流式**：
```json
{
  "candidates": [
    {
      "content": {
        "role": "model",
        "parts": [
          {
            "text": "Hello! How can I help you?"
          }
        ]
      },
      "finishReason": "STOP"
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 5,
    "candidatesTokenCount": 8,
    "totalTokenCount": 13
  }
}
```

**流式**（SSE，`alt=sse`）：
```
data: {"candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":1,"totalTokenCount":6}}

data: {"candidates":[{"content":{"role":"model","parts":[{"text":"!"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":8,"totalTokenCount":13}}
```

#### 特殊处理

1. **模型名规范化**：接受 `gemini-2.5-pro` 和 `models/gemini-2.5-pro` 两种形式，统一处理
2. **Shadow Store**：`GeminiShadowStore` 保存 `thoughtSignature` 和 tool call 信息，用于转换为 Anthropic 格式时回放
3. **GET 端点支持**：使用 `any()` 路由覆盖所有 HTTP 方法，包括 `/models` 查询端点
4. **URL 构建策略**：
   - Base URL 以 `/v1beta` 结尾：使用 origin（直接拼接）
   - Base URL 是自定义路径：视为 full URL（替换整个路径）

### 4. Claude Desktop 3P Gateway

#### 端点

```
GET  /claude-desktop/v1/models
POST /claude-desktop/v1/messages
```

#### 认证方式

```
Authorization: Bearer {gateway_token}
```

`gateway_token` 存储在数据库中，由 `claude_desktop_config::get_or_create_gateway_token()` 生成。

#### 特性

1. **独立 Provider 命名空间**：使用 `AppType::ClaudeDesktop` 独立管理 Provider
2. **强制认证检查**：`validate_claude_desktop_gateway_auth()` 验证 token
3. **与 Claude Code 隔离**：避免互相干扰配置

---

## 核心模块详解

### 1. Proxy Server（`server.rs`）

#### 核心职责

1. **HTTP 服务器启动**：绑定 `127.0.0.1:15721`（默认端口）
2. **手动 Accept Loop**：使用 Hyper HTTP/1.1 手动循环，启用 `preserve_header_case(true)`
3. **原始 Header 大小写捕获**：Peek TCP 字节流提取客户端请求头的原始大小写
4. **路由注册**：构建 Axum Router，注册所有端点
5. **状态管理**：维护 `ProxyState` 共享状态

#### ProxyState 结构

```rust
pub struct ProxyState {
    pub db: Arc<Database>,
    pub config: Arc<RwLock<ProxyConfig>>,
    pub status: Arc<RwLock<ProxyStatus>>,
    pub start_time: Arc<RwLock<Option<Instant>>>,
    pub current_providers: Arc<RwLock<HashMap<String, (String, String)>>>,
    pub provider_router: Arc<ProviderRouter>,
    pub gemini_shadow: Arc<GeminiShadowStore>,
    pub codex_chat_history: Arc<CodexChatHistoryStore>,
    pub app_handle: Option<tauri::AppHandle>,
    pub failover_manager: Arc<FailoverSwitchManager>,
}
```

#### 启动流程

```rust
pub async fn start(&self) -> Result<ProxyServerInfo, ProxyError> {
    // 1. 检查是否已运行
    if self.shutdown_tx.read().await.is_some() {
        return Err(ProxyError::AlreadyRunning);
    }
    
    // 2. 绑定监听器
    let addr = format!("{}:{}", config.listen_address, config.listen_port);
    let listener = tokio::net::TcpListener::bind(&addr).await?;
    
    // 3. 更新全局代理端口（用于系统代理检测）
    crate::proxy::http_client::set_proxy_port(actual_port);
    
    // 4. 启动 accept loop
    tokio::spawn(async move {
        loop {
            tokio::select! {
                result = listener.accept() => {
                    let (stream, _) = result?;
                    let original_cases = peek_header_cases(&stream).await;
                    
                    tokio::spawn(async move {
                        hyper::server::conn::http1::Builder::new()
                            .preserve_header_case(true)
                            .serve_connection(TokioIo::new(stream), service)
                            .await
                    });
                }
                _ = &mut shutdown_rx => break,
            }
        }
    });
    
    Ok(ProxyServerInfo { address, port, started_at })
}
```

#### 优雅关闭

```rust
pub async fn stop(&self) -> Result<(), ProxyError> {
    // 1. 发送关闭信号
    if let Some(tx) = self.shutdown_tx.write().await.take() {
        let _ = tx.send(());
    }
    
    // 2. 等待服务器任务结束（5 秒超时）
    if let Some(handle) = self.server_handle.write().await.take() {
        tokio::time::timeout(Duration::from_secs(5), handle).await??;
    }
    
    Ok(())
}
```

### 2. Handlers（`handlers.rs`）

#### 核心职责

1. **请求解析**：从 Axum Request 中提取 body、headers、extensions
2. **上下文构建**：创建 `RequestContext`，包含 Provider 信息、会话 ID、超时配置等
3. **转发协调**：调用 `Forwarder` 执行带重试的请求转发
4. **格式转换判断**：根据 Provider 类型决定是否需要格式转换
5. **响应处理**：调用 `process_response` 处理流式/非流式响应

#### RequestContext 结构

```rust
pub struct RequestContext {
    pub provider: Provider,
    pub start_time: Instant,
    pub session_id: String,
    pub session_client_provided: bool,
    pub request_model: String,
    pub app_config: AppProxyConfig,
    pub tag: &'static str,
    pub app_type_str: &'static str,
    pub rectifier_config: RectifierConfig,
    pub optimizer_config: OptimizerConfig,
    pub copilot_optimizer_config: CopilotOptimizerConfig,
}
```

#### Handler 通用模式

```rust
pub async fn handle_xxx(
    State(state): State<ProxyState>,
    request: axum::extract::Request,
) -> Result<axum::response::Response, ProxyError> {
    // 1. 解析请求
    let (parts, req_body) = request.into_parts();
    let body_bytes = req_body.collect().await?.to_bytes();
    let body: Value = serde_json::from_slice(&body_bytes)?;
    
    // 2. 创建上下文
    let mut ctx = RequestContext::new(
        &state, &body, &headers, 
        AppType::Xxx, "Xxx", "xxx"
    ).await?;
    
    // 3. 提取 endpoint 和 stream 标志
    let endpoint = extract_endpoint(&uri);
    let is_stream = body.get("stream").and_then(|s| s.as_bool()).unwrap_or(false);
    
    // 4. 转发请求
    let forwarder = ctx.create_forwarder(&state);
    let mut result = forwarder.forward_with_retry(
        &app_type, method, endpoint, body.clone(),
        headers, extensions, ctx.get_providers()
    ).await.map_err(|mut err| {
        if let Some(provider) = err.provider.take() {
            ctx.provider = provider;
        }
        log_forward_error(&state, &ctx, is_stream, &err.error);
        err.error
    })?;
    
    // 5. 提取响应和连接守卫
    let connection_guard = result.connection_guard.take();
    ctx.provider = result.provider;
    let response = result.response;
    
    // 6. 格式转换判断
    let adapter = get_adapter(&app_type);
    if adapter.needs_transform(&ctx.provider) {
        return handle_transform(response, &ctx, &state, ...).await;
    }
    
    // 7. 通用响应处理
    process_response(response, &ctx, &state, &PARSER_CONFIG, connection_guard).await
}
```

#### 错误处理

```rust
fn log_forward_error(state: &ProxyState, ctx: &RequestContext, is_streaming: bool, error: &ProxyError) {
    let logger = UsageLogger::new(&state.db);
    let status_code = map_proxy_error_to_status(error);
    let error_message = get_error_message(error);
    
    logger.log_error_with_context(
        uuid::Uuid::new_v4().to_string(),
        ctx.provider.id.clone(),
        ctx.app_type_str.to_string(),
        ctx.request_model.clone(),
        status_code,
        error_message,
        ctx.latency_ms(),
        is_streaming,
        Some(ctx.session_id.clone()),
        None,
    )?;
}
```

#### Codex 专属错误格式

```rust
fn build_codex_proxy_error_response(
    ctx: &RequestContext,
    endpoint: &str,
    error: &ProxyError,
) -> Result<Response, ProxyError> {
    let status = map_proxy_error_to_status(error);
    let body = json!({
        "error": {
            "message": format!(
                "CC Switch local proxy failed while handling Codex endpoint {}. Provider: {}; model: {}; cause: {}",
                endpoint, ctx.provider.name, ctx.request_model, get_error_message(error)
            ),
            "type": "proxy_error",
            "code": codex_proxy_error_code(error),
            "param": null,
            "provider": ctx.provider.name,
            "model": ctx.request_model,
            "endpoint": endpoint,
        }
    });
    
    Response::builder()
        .status(status)
        .header("content-type", "application/json")
        .body(Body::from(serde_json::to_vec(&body)?))
        .map_err(|e| ProxyError::Internal(e.to_string()))
}
```

### 3. Request Forwarder（`forwarder.rs`）

#### 核心职责

1. **Provider 选择**：从 `ProviderRouter` 获取可用 Provider 列表
2. **请求整流**：应用 Thinking、Media、Budget 等整流器
3. **请求优化**：Bedrock 优化器、Copilot 优化器
4. **格式转换**：根据 Provider 类型转换请求格式
5. **重试与故障转移**：遍历 Provider 列表，自动重试
6. **响应预检**：非流式响应缓冲首字节，流式响应预读首块
7. **熔断器集成**：请求前获取许可，请求后记录结果

#### 核心方法

```rust
pub async fn forward_with_retry(
    &self,
    app_type: &AppType,
    method: Method,
    endpoint: &str,
    mut body: Value,
    headers: HeaderMap,
    extensions: Extensions,
    providers: Vec<Provider>,
) -> Result<ForwardResult, ForwardError> {
    let mut last_error = None;
    let total_providers = providers.len();
    
    for (attempt, mut provider) in providers.into_iter().enumerate() {
        let attempt_num = attempt + 1;
        
        // 1. 熔断器检查
        let allow_result = self.router.allow_provider_request(&provider.id, app_type.as_str()).await;
        if !allow_result.allowed {
            log::info!("[{}] Provider {} 熔断器 OPEN，跳过", self.tag, provider.name);
            last_error = Some(ProxyError::ProviderUnhealthy(format!("熔断器 OPEN")));
            continue;
        }
        
        // 2. 应用请求整流器
        let mut request_body = body.clone();
        self.apply_rectifiers(&mut request_body, &provider);
        
        // 3. 应用优化器（Bedrock / Copilot）
        self.apply_optimizers(&mut request_body, &provider);
        
        // 4. 格式转换（如果需要）
        let (transformed_endpoint, transformed_body) = self.transform_request(
            app_type, endpoint, &request_body, &provider
        ).await?;
        
        // 5. 构建请求头
        let request_headers = self.build_request_headers(&provider, &headers, &extensions).await?;
        
        // 6. 发送请求
        let response = match self.send_request(
            &provider, method.clone(), &transformed_endpoint, 
            &transformed_body, request_headers
        ).await {
            Ok(resp) => resp,
            Err(e) => {
                self.router.record_result(&provider.id, app_type.as_str(), allow_result.used_half_open_permit, false, Some(e.to_string())).await?;
                
                if self.should_retry(&e, attempt_num, total_providers) {
                    last_error = Some(e);
                    continue;
                }
                
                return Err(ForwardError { provider: Some(provider), error: e });
            }
        };
        
        // 7. 响应预检（确保首字节到达后再标记成功）
        let is_streaming = self.is_streaming_request(endpoint, &request_body, &headers);
        let prepared_response = match self.prepare_success_response_for_failover(response, is_streaming).await {
            Ok(resp) => resp,
            Err(e) => {
                self.router.record_result(&provider.id, app_type.as_str(), allow_result.used_half_open_permit, false, Some(e.to_string())).await?;
                
                if self.should_retry(&e, attempt_num, total_providers) {
                    last_error = Some(e);
                    continue;
                }
                
                return Err(ForwardError { provider: Some(provider), error: e });
            }
        };
        
        // 8. 记录成功
        self.router.record_result(&provider.id, app_type.as_str(), allow_result.used_half_open_permit, true, None).await?;
        
        return Ok(ForwardResult {
            response: prepared_response,
            provider,
            connection_guard: Some(connection_guard),
            claude_api_format: Some(api_format),
        });
    }
    
    // 所有 Provider 都失败
    Err(ForwardError {
        provider: None,
        error: last_error.unwrap_or(ProxyError::NoAvailableProvider),
    })
}
```

#### 请求整流器

```rust
fn apply_rectifiers(&self, body: &mut Value, provider: &Provider) {
    if !self.rectifier_config.enabled {
        return;
    }
    
    // 1. Thinking Signature 整流
    if self.rectifier_config.request_thinking_signature {
        thinking_rectifier::strip_invalid_thinking_signatures(body);
    }
    
    // 2. Thinking Budget 整流
    if self.rectifier_config.request_thinking_budget {
        thinking_budget_rectifier::strip_thinking_budget_params(body);
    }
    
    // 3. Media 降级（不支持图片的模型）
    if self.rectifier_config.request_media_fallback {
        let replaced = self.apply_media_prevention(body, provider);
        if replaced > 0 {
            log::info!("[Rectifier] 预防性替换了 {replaced} 个图片块");
        }
    }
}
```

#### 响应预检

```rust
async fn prepare_success_response_for_failover(
    &self,
    response: ProxyResponse,
    is_streaming: bool,
) -> Result<ProxyResponse, ProxyError> {
    let status = response.status();
    let headers = response.headers().clone();
    
    if !is_streaming {
        // 非流式：缓冲整个响应体
        let timeout = if self.non_streaming_timeout.is_zero() {
            Duration::MAX
        } else {
            self.non_streaming_timeout
        };
        
        let body_bytes = tokio::time::timeout(timeout, response.bytes())
            .await
            .map_err(|_| ProxyError::Timeout("响应体读取超时".to_string()))??;
        
        Ok(ProxyResponse::buffered(status, headers, body_bytes))
    } else {
        // 流式：预读首个数据块
        let timeout = self.streaming_first_byte_timeout;
        let mut stream = response.bytes_stream();
        
        let first = tokio::time::timeout(timeout, stream.next())
            .await
            .map_err(|_| ProxyError::Timeout("流式首包超时".to_string()))?
            .ok_or(ProxyError::ForwardFailed("流式响应在首包前结束".to_string()))??;
        
        // 重新组装流
        let replay = futures::stream::once(async move { Ok(first) }).chain(stream);
        Ok(ProxyResponse::streamed(status, headers, replay))
    }
}
```

### 4. Provider Router（`provider_router.rs`）

#### 核心职责

1. **Provider 选择**：根据故障转移开关决定使用当前 Provider 还是故障转移队列
2. **熔断器管理**：为每个 `app_type:provider_id` 创建独立熔断器
3. **健康状态追踪**：记录请求结果，更新 Provider 健康度
4. **放行许可**：请求前获取熔断器许可（Closed/HalfOpen 状态判断）
5. **中立释放**：整流器场景下释放 HalfOpen 许可但不影响健康统计

#### Provider 选择逻辑

```rust
pub async fn select_providers(&self, app_type: &str) -> Result<Vec<Provider>, AppError> {
    let auto_failover_enabled = self.db.get_proxy_config_for_app(app_type).await?.auto_failover_enabled;
    
    if auto_failover_enabled {
        // 故障转移开启：仅按队列顺序依次尝试（P1 → P2 → ...）
        let all_providers = self.db.get_all_providers(app_type)?;
        let ordered_ids: Vec<String> = self.db.get_failover_queue(app_type)?
            .into_iter()
            .map(|item| item.provider_id)
            .collect();
        
        let mut result = Vec::new();
        let mut circuit_open_count = 0;
        
        for provider_id in ordered_ids {
            let Some(provider) = all_providers.get(&provider_id).cloned() else {
                continue;
            };
            
            let circuit_key = format!("{app_type}:{}", provider.id);
            let breaker = self.get_or_create_circuit_breaker(&circuit_key).await;
            
            if breaker.is_available().await {
                result.push(provider);
            } else {
                circuit_open_count += 1;
            }
        }
        
        if result.is_empty() {
            if circuit_open_count > 0 {
                return Err(AppError::AllProvidersCircuitOpen);
            } else {
                return Err(AppError::NoProvidersConfigured);
            }
        }
        
        Ok(result)
    } else {
        // 故障转移关闭：仅使用当前 Provider，跳过熔断器检查
        let current_id = self.db.get_current_provider(app_type)?.ok_or(AppError::NoProvidersConfigured)?;
        let current = self.db.get_provider_by_id(&current_id, app_type)?.ok_or(AppError::NoProvidersConfigured)?;
        Ok(vec![current])
    }
}
```

#### 熔断器请求前检查

```rust
pub async fn allow_provider_request(&self, provider_id: &str, app_type: &str) -> AllowResult {
    let circuit_key = format!("{app_type}:{provider_id}");
    let breaker = self.get_or_create_circuit_breaker(&circuit_key).await;
    breaker.allow_request().await
}
```

**AllowResult 结构**：
```rust
pub struct AllowResult {
    pub allowed: bool,
    pub used_half_open_permit: bool, // 是否占用了 HalfOpen 探测名额
}
```

#### 请求结果记录

```rust
pub async fn record_result(
    &self,
    provider_id: &str,
    app_type: &str,
    used_half_open_permit: bool,
    success: bool,
    error_msg: Option<String>,
) -> Result<(), AppError> {
    // 1. 获取熔断器配置
    let failure_threshold = self.db.get_proxy_config_for_app(app_type).await?.circuit_failure_threshold;
    
    // 2. 更新熔断器状态
    let circuit_key = format!("{app_type}:{provider_id}");
    let breaker = self.get_or_create_circuit_breaker(&circuit_key).await;
    
    if success {
        breaker.record_success(used_half_open_permit).await;
    } else {
        breaker.record_failure(used_half_open_permit).await;
    }
    
    // 3. 更新数据库健康状态
    self.db.update_provider_health_with_threshold(
        provider_id, app_type, success, error_msg, failure_threshold
    ).await?;
    
    Ok(())
}
```

#### 中立释放（Neutral Release）

```rust
pub async fn release_permit_neutral(
    &self,
    provider_id: &str,
    app_type: &str,
    used_half_open_permit: bool,
) {
    if !used_half_open_permit {
        return;
    }
    
    let circuit_key = format!("{app_type}:{provider_id}");
    let breaker = self.get_or_create_circuit_breaker(&circuit_key).await;
    breaker.release_half_open_permit();
}
```

**使用场景**：整流器拦截请求并返回中立响应（如图片降级），不应计入 Provider 健康度统计，但仍需释放 HalfOpen 探测名额。

#### 熔断器热更新

```rust
pub async fn update_all_configs(&self, config: CircuitBreakerConfig) {
    let breakers = self.circuit_breakers.read().await;
    for breaker in breakers.values() {
        breaker.update_config(config.clone()).await;
    }
}

pub async fn update_app_configs(&self, app_type: &str, config: CircuitBreakerConfig) {
    let prefix = format!("{app_type}:");
    let breakers = self.circuit_breakers.read().await;
    for (key, breaker) in breakers.iter() {
        if key.starts_with(&prefix) {
            breaker.update_config(config.clone()).await;
        }
    }
}
```

---

## 请求转发与故障转移

### 故障转移策略

#### 1. 开关控制

```rust
// 每个应用独立配置
struct AppProxyConfig {
    app_type: String,           // "claude" / "codex" / "gemini"
    enabled: bool,              // 该应用代理总开关
    auto_failover_enabled: bool, // 该应用故障转移开关
    // ...
}
```

#### 2. Provider 选择

**故障转移关闭**：
- 仅使用 `current_provider`
- 跳过熔断器检查
- 单 Provider 失败直接返回错误

**故障转移开启**：
- 按故障转移队列顺序尝试（P1 → P2 → P3 → ...）
- 熔断器 OPEN 的 Provider 跳过
- 全部失败返回 `AllProvidersCircuitOpen` 或 `NoAvailableProvider`

#### 3. 重试判断

```rust
fn categorize_proxy_error(&self, error: &ProxyError) -> ErrorCategory {
    match error {
        // 可重试：网络、超时、上游 5xx/429/401/403/404 等
        ProxyError::Timeout(_) => ErrorCategory::Retryable,
        ProxyError::ForwardFailed(_) => ErrorCategory::Retryable,
        ProxyError::UpstreamError { status, .. } => match status {
            400 | 405 | 406 | 413 | 414 | 415 | 422 | 501 => ErrorCategory::NonRetryable, // 客户端错误
            _ => ErrorCategory::Retryable,
        },
        
        // 不可重试：无可用 Provider、所有 Provider 已熔断
        ProxyError::NoAvailableProvider => ErrorCategory::NonRetryable,
        ProxyError::AllProvidersCircuitOpen => ErrorCategory::NonRetryable,
        
        _ => ErrorCategory::NonRetryable,
    }
}
```

#### 4. 响应预检（Response Preflight）

**非流式响应**：
```rust
// 缓冲整个响应体，确保完整可读后再标记 Provider 成功
let body_bytes = tokio::time::timeout(timeout, response.bytes()).await??;
Ok(ProxyResponse::buffered(status, headers, body_bytes))
```

**流式响应**：
```rust
// 预读首个数据块，确保连接建立且有数据到达
let first = tokio::time::timeout(timeout, stream.next()).await??;
let replay = futures::stream::once(async move { Ok(first) }).chain(stream);
Ok(ProxyResponse::streamed(status, headers, replay))
```

**目的**：防止上游返回 200 OK 响应头后中途断开，导致 Provider 被错误标记为成功。

### 熔断器集成

#### 1. 请求前获取许可

```rust
let allow_result = router.allow_provider_request(&provider.id, app_type).await;

if !allow_result.allowed {
    log::info!("Provider {} 熔断器 OPEN，跳过", provider.name);
    continue; // 尝试下一个 Provider
}
```

#### 2. 请求后记录结果

```rust
// 成功
router.record_result(
    &provider.id, app_type, 
    allow_result.used_half_open_permit, 
    true, None
).await?;

// 失败
router.record_result(
    &provider.id, app_type, 
    allow_result.used_half_open_permit, 
    false, Some(error.to_string())
).await?;
```

#### 3. 中立释放（Neutral Release）

```rust
// 整流器拦截，不计入健康统计，但释放 HalfOpen 名额
router.release_permit_neutral(
    &provider.id, app_type, 
    allow_result.used_half_open_permit
).await;
```

### 日志与监控

#### 单 Provider 失败

```rust
log::warn!(
    "[{}] Provider {} 请求失败: {}",
    log_code::SINGLE_PROVIDER_FAILED,
    provider.name,
    summarize_error(&error)
);
```

#### 多 Provider 重试

```rust
log::info!(
    "[{}] Provider {} 失败，继续尝试下一个 ({}/{}): {}",
    log_code::PROVIDER_FAILED_RETRY,
    provider.name, attempt, total_providers,
    summarize_error(&error)
);
```

#### 全部失败

```rust
log::error!(
    "[{}] 已尝试 {}/{} 个 Provider，均失败。最后错误: {}",
    log_code::ALL_PROVIDERS_FAILED,
    total_providers, total_providers,
    summarize_error(&last_error)
);
```

### 连接守卫（Connection Guard）

```rust
pub struct ActiveConnectionGuard {
    status: Arc<RwLock<ProxyStatus>>,
    released: AtomicBool,
}

impl ActiveConnectionGuard {
    pub fn new(status: Arc<RwLock<ProxyStatus>>) -> Self {
        status.write().await.active_connections += 1;
        Self { status, released: AtomicBool::new(false) }
    }
}

impl Drop for ActiveConnectionGuard {
    fn drop(&mut self) {
        if !self.released.swap(true, Ordering::SeqCst) {
            self.status.write().await.active_connections -= 1;
        }
    }
}
```

**作用**：自动追踪活跃连接数，响应完全处理完毕后自动释放。

---

## 响应处理流程

### 响应类型检测

```rust
pub fn is_sse_response(response: &ProxyResponse) -> bool {
    response.headers()
        .get("content-type")
        .and_then(|v| v.to_str().ok())
        .map(|ct| ct.contains("text/event-stream"))
        .unwrap_or(false)
}
```

### 流式响应处理

#### 1. 创建使用量收集器

```rust
let usage_collector = if usage_logging_enabled(state) {
    Some(SseUsageCollector::new(
        start_time,
        Some(usage_event_filter), // 过滤非 usage 相关的事件
        move |events, first_token_ms| {
            // 解析 usage 并异步记录
            let usage = TokenUsage::from_stream_events(&events);
            tokio::spawn(async move {
                log_usage(state, provider_id, app_type, model, usage, latency_ms, first_token_ms, true, status_code, session_id).await;
            });
        }
    ))
} else {
    None
};
```

#### 2. 创建带超时的透传流

```rust
pub fn create_logged_passthrough_stream(
    upstream: impl Stream<Item = Result<Bytes, std::io::Error>> + Send + Unpin + 'static,
    tag: &str,
    usage_collector: Option<SseUsageCollector>,
    timeout_config: StreamingTimeoutConfig,
    connection_guard: Option<ActiveConnectionGuard>,
) -> impl Stream<Item = Result<Bytes, std::io::Error>> + Send + Unpin {
    Box::pin(async_stream::stream! {
        let mut upstream = Box::pin(upstream);
        let mut last_activity = Instant::now();
        let mut total_bytes = 0;
        
        loop {
            // 静默超时检查
            let idle_timeout = if timeout_config.streaming_idle_timeout > 0 {
                Duration::from_secs(timeout_config.streaming_idle_timeout)
            } else {
                Duration::MAX
            };
            
            let chunk = tokio::time::timeout(idle_timeout, upstream.next()).await;
            
            match chunk {
                Ok(Some(Ok(bytes))) => {
                    last_activity = Instant::now();
                    total_bytes += bytes.len();
                    
                    // 使用量收集
                    if let Some(ref collector) = usage_collector {
                        collector.process_chunk(&bytes).await;
                    }
                    
                    yield Ok(bytes);
                }
                Ok(Some(Err(e))) => {
                    log::warn!("[{tag}] 流式响应读取错误: {e}");
                    yield Err(e);
                    break;
                }
                Ok(None) => {
                    // 流正常结束
                    if let Some(collector) = usage_collector {
                        collector.finish().await;
                    }
                    log::debug!("[{tag}] 流式响应完成: {total_bytes} bytes");
                    break;
                }
                Err(_) => {
                    // 静默超时
                    log::error!("[{tag}] 流式响应静默超时: {}s 无数据", idle_timeout.as_secs());
                    yield Err(std::io::Error::other("stream idle timeout"));
                    break;
                }
            }
        }
        
        drop(connection_guard); // 确保在流结束后释放
    })
}
```

#### 3. SSE 使用量收集

```rust
impl SseUsageCollector {
    pub async fn process_chunk(&self, chunk: &Bytes) {
        if self.inner.finished.load(Ordering::Relaxed) {
            return;
        }
        
        // 记录首个事件时间（用于计算 TTFT）
        if !self.inner.first_event_set.load(Ordering::Relaxed) {
            *self.inner.first_event_time.lock().await = Some(Instant::now());
            self.inner.first_event_set.store(true, Ordering::Relaxed);
        }
        
        let text = String::from_utf8_lossy(chunk);
        let mut buffer = text.to_string();
        
        while let Some(block) = take_sse_block(&mut buffer) {
            let mut data_lines = Vec::new();
            
            for line in block.lines() {
                if let Some(data) = strip_sse_field(line, "data") {
                    data_lines.push(data);
                }
            }
            
            if data_lines.is_empty() {
                continue;
            }
            
            let data_str = data_lines.join("\n");
            if data_str.trim() == "[DONE]" {
                continue;
            }
            
            if let Ok(event) = serde_json::from_str::<Value>(&data_str) {
                // 使用过滤器判断是否需要收集
                if let Some(ref filter) = self.inner.should_collect {
                    if !(filter)(&event) {
                        continue;
                    }
                }
                
                self.inner.events.lock().await.push(event);
            }
        }
    }
    
    pub async fn finish(self) {
        if self.inner.finished.swap(true, Ordering::SeqCst) {
            return;
        }
        
        let events = self.inner.events.lock().await.clone();
        let first_token_ms = self.inner.first_event_time.lock().await
            .map(|t| t.duration_since(self.inner.start_time).as_millis() as u64);
        
        (self.inner.on_complete)(events, first_token_ms);
    }
}
```

### 非流式响应处理

#### 1. 读取并解压响应体

```rust
pub async fn read_decoded_body(
    response: ProxyResponse,
    tag: &str,
    body_timeout: Duration,
) -> Result<(HeaderMap, StatusCode, Bytes), ProxyError> {
    let mut headers = response.headers().clone();
    let status = response.status();
    
    // 带超时的读取
    let raw_bytes = if body_timeout.is_zero() {
        response.bytes().await?
    } else {
        tokio::time::timeout(body_timeout, response.bytes())
            .await
            .map_err(|_| ProxyError::Timeout("响应体读取超时".to_string()))??
    };
    
    // 解压（如果需要）
    let body_bytes = if let Some(encoding) = get_content_encoding(&headers) {
        let decompressed = decompress_body(&encoding, &raw_bytes)?;
        strip_entity_headers_for_rebuilt_body(&mut headers);
        Bytes::from(decompressed)
    } else {
        raw_bytes
    };
    
    Ok((headers, status, body_bytes))
}
```

#### 2. 解析使用量

```rust
if usage_logging_enabled(state) {
    if let Ok(json_value) = serde_json::from_slice::<Value>(&body_bytes) {
        if let Some(usage) = (parser_config.response_parser)(&json_value) {
            let model = usage.model.as_ref()
                .or_else(|| json_value.get("model").and_then(|m| m.as_str()))
                .unwrap_or(&ctx.request_model);
            
            spawn_log_usage(state, ctx, usage, model, &ctx.request_model, status.as_u16(), false);
        }
    }
}
```

#### 3. 构建响应

```rust
let mut builder = Response::builder().status(status);

// 复制响应头（剥离 hop-by-hop 头）
strip_hop_by_hop_response_headers(&mut response_headers);
for (key, value) in response_headers.iter() {
    builder = builder.header(key, value);
}

let body = Body::from(body_bytes);
builder.body(body).map_err(|e| ProxyError::Internal(e.to_string()))
```

### 响应解压

```rust
fn decompress_body(content_encoding: &str, body: &[u8]) -> Result<Vec<u8>, std::io::Error> {
    match content_encoding {
        "gzip" | "x-gzip" => {
            let mut decoder = flate2::read::GzDecoder::new(body);
            let mut decompressed = Vec::new();
            decoder.read_to_end(&mut decompressed)?;
            Ok(decompressed)
        }
        "deflate" => {
            let mut decoder = flate2::read::DeflateDecoder::new(body);
            let mut decompressed = Vec::new();
            decoder.read_to_end(&mut decompressed)?;
            Ok(decompressed)
        }
        "br" => {
            let mut decompressed = Vec::new();
            brotli::BrotliDecompress(&mut std::io::Cursor::new(body), &mut decompressed)?;
            Ok(decompressed)
        }
        _ => Ok(body.to_vec()),
    }
}
```

---

## 格式转换机制

### 转换场景

#### 1. Claude → OpenAI Chat Completions

**使用场景**：GitHub Copilot、OpenRouter（旧版）

**转换方向**：
- 请求：Anthropic Messages → OpenAI Chat Completions
- 响应：OpenAI Chat Completions → Anthropic Messages

**请求转换**：
```rust
pub fn anthropic_to_openai(anthropic_request: Value) -> Result<Value, ProxyError> {
    let messages = anthropic_request["messages"].as_array()
        .ok_or(ProxyError::TransformError("missing messages".to_string()))?;
    
    let mut openai_messages = Vec::new();
    
    // 转换 system 为 system message
    if let Some(system) = anthropic_request.get("system").and_then(|s| s.as_str()) {
        openai_messages.push(json!({
            "role": "system",
            "content": system
        }));
    }
    
    // 转换 messages
    for msg in messages {
        let role = msg["role"].as_str().ok_or(ProxyError::TransformError("missing role".to_string()))?;
        let content = &msg["content"];
        
        if let Some(text) = content.as_str() {
            // 简单文本
            openai_messages.push(json!({
                "role": role,
                "content": text
            }));
        } else if let Some(blocks) = content.as_array() {
            // 多模态内容块
            let mut openai_content = Vec::new();
            for block in blocks {
                match block["type"].as_str() {
                    Some("text") => {
                        openai_content.push(json!({
                            "type": "text",
                            "text": block["text"]
                        }));
                    }
                    Some("image") => {
                        openai_content.push(json!({
                            "type": "image_url",
                            "image_url": {
                                "url": format!("data:{};base64,{}", 
                                    block["source"]["media_type"].as_str().unwrap_or("image/png"),
                                    block["source"]["data"].as_str().unwrap_or("")
                                )
                            }
                        }));
                    }
                    Some("tool_use") => {
                        // 转换为 tool_calls
                        openai_messages.push(json!({
                            "role": "assistant",
                            "tool_calls": [{
                                "id": block["id"],
                                "type": "function",
                                "function": {
                                    "name": block["name"],
                                    "arguments": serde_json::to_string(&block["input"]).unwrap_or_default()
                                }
                            }]
                        }));
                    }
                    Some("tool_result") => {
                        // 转换为 tool message
                        openai_messages.push(json!({
                            "role": "tool",
                            "tool_call_id": block["tool_use_id"],
                            "content": block["content"]
                        }));
                    }
                    _ => {}
                }
            }
            
            if !openai_content.is_empty() {
                openai_messages.push(json!({
                    "role": role,
                    "content": openai_content
                }));
            }
        }
    }
    
    Ok(json!({
        "model": anthropic_request["model"],
        "messages": openai_messages,
        "max_tokens": anthropic_request.get("max_tokens").or(Some(&json!(4096))),
        "temperature": anthropic_request.get("temperature"),
        "stream": anthropic_request.get("stream"),
    }))
}
```

**响应转换（流式）**：
```rust
pub fn create_anthropic_sse_stream_from_chat(
    upstream: impl Stream<Item = Result<Bytes, std::io::Error>> + Send + Unpin + 'static,
) -> impl Stream<Item = Result<Bytes, std::io::Error>> + Send + Unpin {
    Box::pin(async_stream::stream! {
        let mut buffer = String::new();
        let mut message_id = format!("msg_{}", uuid::Uuid::new_v4());
        let mut role = "assistant";
        let mut content_index = 0;
        
        // 发送 message_start
        yield Ok(Bytes::from(format!(
            "event: message_start\ndata: {{\"type\":\"message_start\",\"message\":{{\"id\":\"{}\",\"type\":\"message\",\"role\":\"{}\"}}}}\n\n",
            message_id, role
        )));
        
        // 发送 content_block_start
        yield Ok(Bytes::from(format!(
            "event: content_block_start\ndata: {{\"type\":\"content_block_start\",\"index\":{},\"content_block\":{{\"type\":\"text\",\"text\":\"\"}}}}\n\n",
            content_index
        )));
        
        let mut upstream = Box::pin(upstream);
        while let Some(chunk) = upstream.next().await {
            let chunk = match chunk {
                Ok(c) => c,
                Err(e) => {
                    yield Err(e);
                    break;
                }
            };
            
            buffer.push_str(&String::from_utf8_lossy(&chunk));
            
            while let Some(block) = take_sse_block(&mut buffer) {
                for line in block.lines() {
                    if let Some(data) = strip_sse_field(line, "data") {
                        if data.trim() == "[DONE]" {
                            continue;
                        }
                        
                        if let Ok(event) = serde_json::from_str::<Value>(data) {
                            if let Some(delta) = event["choices"][0]["delta"]["content"].as_str() {
                                // 转换为 content_block_delta
                                yield Ok(Bytes::from(format!(
                                    "event: content_block_delta\ndata: {{\"type\":\"content_block_delta\",\"index\":{},\"delta\":{{\"type\":\"text_delta\",\"text\":{}}}}}\n\n",
                                    content_index, serde_json::to_string(delta).unwrap()
                                )));
                            }
                            
                            if event["choices"][0]["finish_reason"].is_string() {
                                // 转换为 message_delta
                                yield Ok(Bytes::from(format!(
                                    "event: content_block_stop\ndata: {{\"type\":\"content_block_stop\",\"index\":{}}}\n\n",
                                    content_index
                                )));
                                
                                yield Ok(Bytes::from(format!(
                                    "event: message_delta\ndata: {{\"type\":\"message_delta\",\"delta\":{{\"stop_reason\":\"end_turn\"}}}}\n\n"
                                )));
                            }
                        }
                    }
                }
            }
        }
        
        // 发送 message_stop
        yield Ok(Bytes::from("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"));
    })
}
```

#### 2. Claude → OpenAI Responses API

**使用场景**：Codex OAuth（ChatGPT Plus）

**特殊处理**：Codex OAuth 会将 `stream:false` 强制升级为 SSE，本地代理需要聚合后返回 JSON

**响应聚合**：
```rust
fn responses_sse_to_response_value(body: &str) -> Result<Value, ProxyError> {
    let mut buffer = body.to_string();
    let mut completed_response: Option<Value> = None;
    let mut output_items = Vec::new();
    
    while let Some(block) = take_sse_block(&mut buffer) {
        let mut event_name = "";
        let mut data_lines: Vec<&str> = Vec::new();
        
        for line in block.lines() {
            if let Some(evt) = strip_sse_field(line, "event") {
                event_name = evt.trim();
            } else if let Some(d) = strip_sse_field(line, "data") {
                data_lines.push(d);
            }
        }
        
        if data_lines.is_empty() {
            continue;
        }
        
        let data_str = data_lines.join("\n");
        let data: Value = serde_json::from_str(&data_str)?;
        
        match event_name {
            "response.output_item.done" => {
                if let Some(item) = data.get("item") {
                    output_items.push(item.clone());
                }
            }
            "response.completed" => {
                completed_response = Some(data.get("response").cloned().unwrap_or(data));
            }
            "response.failed" => {
                let message = data.pointer("/response/error/message")
                    .and_then(|v| v.as_str())
                    .unwrap_or("response.failed event received");
                return Err(ProxyError::TransformError(message.to_string()));
            }
            _ => {}
        }
    }
    
    let mut response = completed_response.ok_or_else(|| {
        ProxyError::TransformError("No response.completed event in upstream SSE".to_string())
    })?;
    
    if !output_items.is_empty() {
        response.as_object_mut().unwrap().insert("output".to_string(), Value::Array(output_items));
    }
    
    Ok(response)
}
```

#### 3. Claude → Gemini Native

**使用场景**：使用 Gemini 作为 Claude 的上游

**端点转换**：
```rust
// /v1/messages?beta=true → /v1beta/models/gemini-2.5-pro:generateContent
// /v1/messages?beta=true (stream) → /v1beta/models/gemini-2.5-pro:streamGenerateContent?alt=sse

let model = body["model"].as_str().unwrap_or("gemini-2.5-pro");
let is_stream = body.get("stream").and_then(|v| v.as_bool()).unwrap_or(false);

let target_path = if is_stream {
    format!("/v1beta/models/{}:streamGenerateContent", model)
} else {
    format!("/v1beta/models/{}:generateContent", model)
};

let query = if is_stream { Some("alt=sse") } else { None };
```

**Shadow Store**：保存 `thoughtSignature` 和 tool call 信息，用于回放

```rust
pub struct GeminiShadowStore {
    shadows: RwLock<HashMap<String, GeminiShadow>>,
}

pub struct GeminiShadow {
    pub thought_signatures: Vec<String>,
    pub tool_calls: Vec<ToolCall>,
}
```

#### 4. Codex Chat → Responses

**使用场景**：某些 Provider（如 MiniMax）仅支持 Chat Completions，但 Codex CLI 使用 Responses API

**转换方向**：
- 请求：Responses → Chat Completions
- 响应：Chat Completions → Responses

**Tool Call 历史恢复**：
```rust
pub struct CodexChatHistoryStore {
    history: RwLock<HashMap<String, ChatHistoryEntry>>,
}

pub struct ChatHistoryEntry {
    pub response_id: String,
    pub tool_calls: Vec<ToolCall>,
}
```

**用途**：Responses API 的 `previous_response_id` 字段引用上一轮的 tool call，转换为 Chat 时需要恢复这些信息。

---

## 熔断器与健康检查

### 熔断器状态机

```
Closed (正常) → Open (熔断) → HalfOpen (探测) → Closed (恢复)
     ↓               ↓                ↓
  连续失败达阈值   超时后进入探测    探测成功达阈值
```

#### 状态定义

```rust
pub enum CircuitState {
    Closed,   // 正常状态，所有请求通过
    Open,     // 熔断状态，拒绝所有请求
    HalfOpen, // 探测状态，限流放行探测请求
}
```

### 熔断器配置

```rust
pub struct CircuitBreakerConfig {
    pub failure_threshold: u32,      // 触发熔断的连续失败次数（默认 5）
    pub success_threshold: u32,      // 恢复正常的连续成功次数（默认 2）
    pub timeout_seconds: u64,        // Open → HalfOpen 的超时时间（默认 30 秒）
    pub error_rate_threshold: f64,   // 错误率阈值（0.0-1.0，默认 0.5）
    pub min_requests: u32,           // 计算错误率的最小请求数（默认 10）
}
```

### 熔断器实现

```rust
pub struct CircuitBreaker {
    state: RwLock<CircuitState>,
    config: RwLock<CircuitBreakerConfig>,
    consecutive_failures: AtomicU32,
    consecutive_successes: AtomicU32,
    total_requests: AtomicU32,
    failed_requests: AtomicU32,
    last_failure_time: RwLock<Option<Instant>>,
    half_open_permits: AtomicU32, // HalfOpen 状态下的探测名额
}

impl CircuitBreaker {
    pub async fn allow_request(&self) -> AllowResult {
        let mut state = self.state.write().await;
        
        match *state {
            CircuitState::Closed => {
                // 正常状态，直接放行
                AllowResult { allowed: true, used_half_open_permit: false }
            }
            CircuitState::Open => {
                // 检查是否超时
                let config = self.config.read().await;
                let timeout = Duration::from_secs(config.timeout_seconds);
                
                if let Some(last_failure) = *self.last_failure_time.read().await {
                    if last_failure.elapsed() >= timeout {
                        // 超时，切换到 HalfOpen
                        *state = CircuitState::HalfOpen;
                        self.half_open_permits.store(1, Ordering::SeqCst);
                        return AllowResult { allowed: true, used_half_open_permit: true };
                    }
                }
                
                AllowResult { allowed: false, used_half_open_permit: false }
            }
            CircuitState::HalfOpen => {
                // 限流探测
                let permits = self.half_open_permits.load(Ordering::SeqCst);
                if permits > 0 {
                    self.half_open_permits.fetch_sub(1, Ordering::SeqCst);
                    AllowResult { allowed: true, used_half_open_permit: true }
                } else {
                    AllowResult { allowed: false, used_half_open_permit: false }
                }
            }
        }
    }
    
    pub async fn record_success(&self, used_half_open_permit: bool) {
        self.consecutive_failures.store(0, Ordering::SeqCst);
        let successes = self.consecutive_successes.fetch_add(1, Ordering::SeqCst) + 1;
        self.total_requests.fetch_add(1, Ordering::SeqCst);
        
        let mut state = self.state.write().await;
        let config = self.config.read().await;
        
        if *state == CircuitState::HalfOpen {
            if successes >= config.success_threshold {
                // 恢复正常
                *state = CircuitState::Closed;
                self.consecutive_successes.store(0, Ordering::SeqCst);
                log::info!("[CircuitBreaker] 熔断器恢复: 连续成功 {}", successes);
            } else if used_half_open_permit {
                // 释放探测名额，允许下一次探测
                self.half_open_permits.store(1, Ordering::SeqCst);
            }
        }
    }
    
    pub async fn record_failure(&self, used_half_open_permit: bool) {
        self.consecutive_successes.store(0, Ordering::SeqCst);
        let failures = self.consecutive_failures.fetch_add(1, Ordering::SeqCst) + 1;
        self.total_requests.fetch_add(1, Ordering::SeqCst);
        self.failed_requests.fetch_add(1, Ordering::SeqCst);
        
        *self.last_failure_time.write().await = Some(Instant::now());
        
        let mut state = self.state.write().await;
        let config = self.config.read().await;
        
        match *state {
            CircuitState::Closed => {
                // 检查是否达到熔断阈值
                if failures >= config.failure_threshold {
                    *state = CircuitState::Open;
                    log::warn!("[CircuitBreaker] 熔断器触发: 连续失败 {}", failures);
                }
                
                // 检查错误率
                let total = self.total_requests.load(Ordering::SeqCst);
                if total >= config.min_requests {
                    let failed = self.failed_requests.load(Ordering::SeqCst);
                    let error_rate = failed as f64 / total as f64;
                    
                    if error_rate >= config.error_rate_threshold {
                        *state = CircuitState::Open;
                        log::warn!("[CircuitBreaker] 熔断器触发: 错误率 {:.2}%", error_rate * 100.0);
                    }
                }
            }
            CircuitState::HalfOpen => {
                // 探测失败，重新打开熔断器
                *state = CircuitState::Open;
                log::warn!("[CircuitBreaker] 探测失败，重新熔断");
                
                if used_half_open_permit {
                    self.half_open_permits.store(0, Ordering::SeqCst);
                }
            }
            _ => {}
        }
    }
    
    pub fn release_half_open_permit(&self) {
        // 中立释放：仅释放名额，不影响健康统计
        self.half_open_permits.fetch_add(1, Ordering::SeqCst);
    }
}
```

### 健康状态数据库

```rust
pub struct ProviderHealth {
    pub provider_id: String,
    pub app_type: String,
    pub is_healthy: bool,
    pub consecutive_failures: u32,
    pub last_success_at: Option<String>,
    pub last_failure_at: Option<String>,
    pub last_error: Option<String>,
    pub updated_at: String,
}
```

**更新逻辑**：

```rust
pub async fn update_provider_health_with_threshold(
    &self,
    provider_id: &str,
    app_type: &str,
    success: bool,
    error_msg: Option<String>,
    failure_threshold: u32,
) -> Result<(), AppError> {
    let now = chrono::Utc::now().to_rfc3339();
    
    let mut health = self.get_provider_health(provider_id, app_type)
        .unwrap_or_else(|| ProviderHealth {
            provider_id: provider_id.to_string(),
            app_type: app_type.to_string(),
            is_healthy: true,
            consecutive_failures: 0,
            last_success_at: None,
            last_failure_at: None,
            last_error: None,
            updated_at: now.clone(),
        });
    
    if success {
        health.consecutive_failures = 0;
        health.is_healthy = true;
        health.last_success_at = Some(now.clone());
    } else {
        health.consecutive_failures += 1;
        health.last_failure_at = Some(now.clone());
        health.last_error = error_msg;
        
        if health.consecutive_failures >= failure_threshold {
            health.is_healthy = false;
        }
    }
    
    health.updated_at = now;
    self.save_provider_health(&health)?;
    
    Ok(())
}
```

### 手动恢复

```rust
// UI 触发手动恢复
pub async fn reset_circuit_breaker(&self, provider_id: &str, app_type: &str) {
    let circuit_key = format!("{app_type}:{provider_id}");
    let breakers = self.circuit_breakers.read().await;
    
    if let Some(breaker) = breakers.get(&circuit_key) {
        breaker.reset().await;
        log::info!("[CircuitBreaker] 手动重置熔断器: {}", circuit_key);
    }
}

impl CircuitBreaker {
    pub async fn reset(&self) {
        *self.state.write().await = CircuitState::Closed;
        self.consecutive_failures.store(0, Ordering::SeqCst);
        self.consecutive_successes.store(0, Ordering::SeqCst);
        self.total_requests.store(0, Ordering::SeqCst);
        self.failed_requests.store(0, Ordering::SeqCst);
        *self.last_failure_time.write().await = None;
        self.half_open_permits.store(0, Ordering::SeqCst);
    }
}
```

---

## 使用量统计

### 数据模型

```rust
pub struct TokenUsage {
    pub input_tokens: u64,
    pub output_tokens: u64,
    pub total_tokens: u64,
    pub cache_creation_input_tokens: Option<u64>,
    pub cache_read_input_tokens: Option<u64>,
    pub model: Option<String>,
    pub request_id: Option<String>,
}
```

### 解析器配置

```rust
pub struct UsageParserConfig {
    pub app_type_str: &'static str,
    pub response_parser: fn(&Value) -> Option<TokenUsage>,
    pub stream_event_parser: fn(&Value) -> Option<TokenUsage>,
}

// Claude 解析器
pub const CLAUDE_PARSER_CONFIG: UsageParserConfig = UsageParserConfig {
    app_type_str: "claude",
    response_parser: parse_claude_response,
    stream_event_parser: parse_claude_stream_event,
};

fn parse_claude_response(response: &Value) -> Option<TokenUsage> {
    let usage = response.get("usage")?;
    Some(TokenUsage {
        input_tokens: usage["input_tokens"].as_u64().unwrap_or(0),
        output_tokens: usage["output_tokens"].as_u64().unwrap_or(0),
        total_tokens: usage["input_tokens"].as_u64().unwrap_or(0) + usage["output_tokens"].as_u64().unwrap_or(0),
        cache_creation_input_tokens: usage["cache_creation_input_tokens"].as_u64(),
        cache_read_input_tokens: usage["cache_read_input_tokens"].as_u64(),
        model: response["model"].as_str().map(|s| s.to_string()),
        request_id: response["id"].as_str().map(|s| s.to_string()),
    })
}

// Codex 解析器
pub const CODEX_PARSER_CONFIG: UsageParserConfig = UsageParserConfig {
    app_type_str: "codex",
    response_parser: parse_codex_response,
    stream_event_parser: parse_codex_stream_event,
};

fn parse_codex_response(response: &Value) -> Option<TokenUsage> {
    let usage = response.get("usage")?;
    Some(TokenUsage {
        input_tokens: usage["prompt_tokens"].as_u64().or_else(|| usage["input_tokens"].as_u64()).unwrap_or(0),
        output_tokens: usage["completion_tokens"].as_u64().or_else(|| usage["output_tokens"].as_u64()).unwrap_or(0),
        total_tokens: usage["total_tokens"].as_u64().unwrap_or(0),
        cache_creation_input_tokens: None,
        cache_read_input_tokens: None,
        model: response["model"].as_str().map(|s| s.to_string()),
        request_id: response["id"].as_str().map(|s| s.to_string()),
    })
}

// Gemini 解析器
pub const GEMINI_PARSER_CONFIG: UsageParserConfig = UsageParserConfig {
    app_type_str: "gemini",
    response_parser: parse_gemini_response,
    stream_event_parser: parse_gemini_stream_event,
};

fn parse_gemini_response(response: &Value) -> Option<TokenUsage> {
    let usage = response.get("usageMetadata")?;
    Some(TokenUsage {
        input_tokens: usage["promptTokenCount"].as_u64().unwrap_or(0),
        output_tokens: usage["candidatesTokenCount"].as_u64().unwrap_or(0),
        total_tokens: usage["totalTokenCount"].as_u64().unwrap_or(0),
        cache_creation_input_tokens: None,
        cache_read_input_tokens: None,
        model: None,
        request_id: None,
    })
}
```

### 流式事件过滤器

```rust
pub type StreamUsageEventFilter = fn(&Value) -> bool;

// Claude 过滤器：仅收集 message_delta 和 message_stop 事件
pub fn claude_stream_usage_event_filter(event: &Value) -> bool {
    matches!(
        event.get("type").and_then(|t| t.as_str()),
        Some("message_delta") | Some("message_stop")
    )
}

// Codex 过滤器：仅收集最后一个 chunk（有 finish_reason）
pub fn codex_stream_usage_event_filter(event: &Value) -> bool {
    event["choices"][0]["finish_reason"].is_string()
        || event.get("usage").is_some()
}

// Gemini 过滤器：仅收集最后一个事件（有 usageMetadata）
pub fn gemini_stream_usage_event_filter(event: &Value) -> bool {
    event.get("usageMetadata").is_some()
}
```

### 日志记录

```rust
pub async fn log_usage(
    state: &ProxyState,
    provider_id: &str,
    app_type: &str,
    model: &str,
    request_model: &str,
    usage: TokenUsage,
    latency_ms: u64,
    first_token_ms: Option<u64>,
    is_streaming: bool,
    status_code: u16,
    session_id: Option<String>,
) {
    let logger = UsageLogger::new(&state.db);
    
    // 获取定价配置
    let (multiplier, pricing_model_source) = logger.resolve_pricing_config(provider_id, app_type).await;
    
    // 决定定价模型
    let pricing_model = if pricing_model_source == PRICING_SOURCE_REQUEST {
        request_model
    } else {
        model
    };
    
    let request_id = usage.dedup_request_id();
    
    logger.log_with_calculation(
        request_id,
        provider_id.to_string(),
        app_type.to_string(),
        model.to_string(),
        request_model.to_string(),
        pricing_model.to_string(),
        usage,
        multiplier,
        latency_ms,
        first_token_ms,
        status_code,
        session_id,
        None,
        is_streaming,
    )?;
}
```

### 成本计算

```rust
pub struct UsageLogger {
    db: Arc<Database>,
}

impl UsageLogger {
    pub fn calculate_cost(
        &self,
        usage: &TokenUsage,
        pricing_model: &str,
        multiplier: f64,
    ) -> (f64, f64, f64) {
        // 从数据库获取定价信息
        let pricing = match self.db.get_model_pricing(pricing_model) {
            Ok(Some(p)) => p,
            _ => return (0.0, 0.0, 0.0),
        };
        
        // 计算输入成本（包括 cache）
        let mut input_cost = (usage.input_tokens as f64) * pricing.input_price_per_million / 1_000_000.0;
        
        if let Some(cache_creation) = usage.cache_creation_input_tokens {
            input_cost += (cache_creation as f64) * pricing.cache_creation_price_per_million.unwrap_or(pricing.input_price_per_million) / 1_000_000.0;
        }
        
        if let Some(cache_read) = usage.cache_read_input_tokens {
            input_cost += (cache_read as f64) * pricing.cache_read_price_per_million.unwrap_or(pricing.input_price_per_million * 0.1) / 1_000_000.0;
        }
        
        // 计算输出成本
        let output_cost = (usage.output_tokens as f64) * pricing.output_price_per_million / 1_000_000.0;
        
        // 应用倍率
        let total_cost = (input_cost + output_cost) * multiplier;
        
        (input_cost * multiplier, output_cost * multiplier, total_cost)
    }
    
    pub async fn resolve_pricing_config(&self, provider_id: &str, app_type: &str) -> (f64, String) {
        // 从数据库获取 Provider 定价配置
        match self.db.get_provider_pricing_config(provider_id, app_type) {
            Ok(Some(config)) => (config.multiplier, config.pricing_model_source),
            _ => (1.0, "response".to_string()),
        }
    }
}
```

### 数据库表结构

```sql
CREATE TABLE usage_logs (
    id TEXT PRIMARY KEY,
    provider_id TEXT NOT NULL,
    app_type TEXT NOT NULL,
    model TEXT NOT NULL,
    request_model TEXT NOT NULL,
    pricing_model TEXT NOT NULL,
    input_tokens INTEGER NOT NULL,
    output_tokens INTEGER NOT NULL,
    total_tokens INTEGER NOT NULL,
    cache_creation_input_tokens INTEGER,
    cache_read_input_tokens INTEGER,
    input_cost REAL NOT NULL,
    output_cost REAL NOT NULL,
    total_cost REAL NOT NULL,
    latency_ms INTEGER NOT NULL,
    first_token_ms INTEGER,
    status_code INTEGER NOT NULL,
    session_id TEXT,
    is_streaming BOOLEAN NOT NULL,
    created_at TEXT NOT NULL,
    INDEX idx_provider_id (provider_id),
    INDEX idx_app_type (app_type),
    INDEX idx_created_at (created_at),
    INDEX idx_session_id (session_id)
);
```

### UI 统计查询

```rust
// 按时间范围统计
pub fn get_usage_summary(&self, start: &str, end: &str) -> Result<UsageSummary, AppError> {
    let conn = self.db.conn.lock()?;
    
    let (total_requests, total_cost, total_tokens) = conn.query_row(
        "SELECT COUNT(*), SUM(total_cost), SUM(total_tokens) FROM usage_logs WHERE created_at >= ? AND created_at < ?",
        params![start, end],
        |row| Ok((row.get(0)?, row.get(1)?, row.get(2)?))
    )?;
    
    Ok(UsageSummary { total_requests, total_cost, total_tokens })
}

// 按 Provider 统计
pub fn get_usage_by_provider(&self, start: &str, end: &str) -> Result<Vec<ProviderUsage>, AppError> {
    let conn = self.db.conn.lock()?;
    
    let mut stmt = conn.prepare(
        "SELECT provider_id, COUNT(*), SUM(total_cost), SUM(total_tokens) 
         FROM usage_logs 
         WHERE created_at >= ? AND created_at < ? 
         GROUP BY provider_id"
    )?;
    
    let rows = stmt.query_map(params![start, end], |row| {
        Ok(ProviderUsage {
            provider_id: row.get(0)?,
            requests: row.get(1)?,
            cost: row.get(2)?,
            tokens: row.get(3)?,
        })
    })?;
    
    rows.collect()
}
```

---

## 完整实现清单

### 核心模块

#### 1. 代理服务器（`src-tauri/src/proxy/`）

| 文件 | 职责 | 关键功能 |
|------|------|----------|
| `mod.rs` | 模块导出 | 公开 API 和类型定义 |
| `server.rs` | HTTP 服务器 | Axum Router、手动 accept loop、header 大小写保留 |
| `handlers.rs` | 请求处理器 | 路由 handler、上下文构建、格式转换判断 |
| `forwarder.rs` | 请求转发器 | 重试逻辑、整流器、优化器、响应预检 |
| `provider_router.rs` | Provider 路由 | Provider 选择、熔断器管理、健康追踪 |
| `types.rs` | 类型定义 | ProxyConfig、ProxyStatus、AppProxyConfig 等 |
| `error.rs` | 错误类型 | ProxyError 定义 |

#### 2. HTTP 客户端（`src-tauri/src/proxy/`）

| 文件 | 职责 | 关键功能 |
|------|------|----------|
| `hyper_client.rs` | Hyper 客户端 | 原始 header 大小写恢复、连接池 |
| `http_client.rs` | Reqwest 包装 | 禁用自动解压、系统代理检测 |

#### 3. Provider 适配器（`src-tauri/src/proxy/providers/`）

| 文件 | 职责 | 关键功能 |
|------|------|----------|
| `mod.rs` | 适配器导出 | ProviderType、get_adapter() |
| `adapter.rs` | 适配器 trait | ProviderAdapter trait 定义 |
| `auth.rs` | 认证策略 | AuthStrategy、AuthInfo |
| `claude.rs` | Claude 适配器 | API 格式检测、请求规范化 |
| `codex.rs` | Codex 适配器 | 端点判断、模型映射 |
| `gemini.rs` | Gemini 适配器 | OAuth 检测、URL 构建 |

#### 4. 格式转换（`src-tauri/src/proxy/providers/`）

| 文件 | 职责 | 关键功能 |
|------|------|----------|
| `transform.rs` | OpenAI ↔ Anthropic | Chat Completions 格式转换 |
| `transform_responses.rs` | Responses ↔ Anthropic | Responses API 格式转换 |
| `transform_gemini.rs` | Gemini ↔ Anthropic | Gemini 格式转换 + Shadow |
| `transform_codex_chat.rs` | Chat ↔ Responses | Codex 专用转换 + 历史恢复 |

#### 5. 流式处理（`src-tauri/src/proxy/providers/`）

| 文件 | 职责 | 关键功能 |
|------|------|----------|
| `streaming.rs` | 流式转换基础 | SSE 解析、事件提取 |
| `streaming_codex_chat.rs` | Chat SSE 转换 | Chat Completions → Responses SSE |
| `streaming_gemini.rs` | Gemini SSE 转换 | Gemini SSE → Anthropic SSE |
| `streaming_responses.rs` | Responses SSE 转换 | Responses SSE → Anthropic SSE |

#### 6. 响应处理（`src-tauri/src/proxy/`）

| 文件 | 职责 | 关键功能 |
|------|------|----------|
| `response_processor.rs` | 响应处理器 | 流式/非流式判断、解压、使用量解析 |
| `response_handler.rs` | 响应 handler | StreamHandler、NonStreamHandler |
| `sse.rs` | SSE 解析 | take_sse_block、strip_sse_field |

#### 7. 熔断器（`src-tauri/src/proxy/`）

| 文件 | 职责 | 关键功能 |
|------|------|----------|
| `circuit_breaker.rs` | 熔断器实现 | 状态机、放行许可、HalfOpen 探测 |
| `health.rs` | 健康检查 | 健康度追踪、数据库更新 |

#### 8. 整流器与优化器（`src-tauri/src/proxy/`）

| 文件 | 职责 | 关键功能 |
|------|------|----------|
| `thinking_rectifier.rs` | Thinking 签名整流 | 剥离无效 thinking 签名 |
| `thinking_budget_rectifier.rs` | Thinking Budget 整流 | 剥离 budget 参数 |
| `media_sanitizer.rs` | Media 整流 | 图片降级为文本标记 |
| `thinking_optimizer.rs` | Thinking 优化器 | Bedrock thinking 优化 |
| `cache_injector.rs` | Cache 注入器 | Bedrock prompt cache 注入 |
| `copilot_optimizer.rs` | Copilot 优化器 | x-initiator 分类、tool 合并 |

#### 9. 使用量统计（`src-tauri/src/proxy/usage/`）

| 文件 | 职责 | 关键功能 |
|------|------|----------|
| `parser.rs` | 使用量解析 | TokenUsage、解析器配置 |
| `logger.rs` | 使用量日志 | 数据库记录、成本计算 |

#### 10. 辅助模块（`src-tauri/src/proxy/`）

| 文件 | 职责 | 关键功能 |
|------|------|----------|
| `session.rs` | 会话管理 | 会话 ID 提取、ClientFormat 检测 |
| `failover_switch.rs` | 故障转移开关 | 开关管理、热切换 |
| `switch_lock.rs` | 切换锁 | 防止并发切换 |
| `model_mapper.rs` | 模型映射 | 模型名称映射 |
| `error_mapper.rs` | 错误映射 | 错误状态码映射 |
| `log_codes.rs` | 日志代码 | 结构化日志代码 |
| `body_filter.rs` | Body 过滤 | 敏感字段过滤 |
| `json_canonical.rs` | JSON 规范化 | 键排序、哈希计算 |
| `gemini_url.rs` | Gemini URL | URL 构建策略 |

### 数据库表

#### 1. providers

```sql
CREATE TABLE providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    app_type TEXT NOT NULL,
    settings_config TEXT NOT NULL,
    sort_index INTEGER,
    in_failover_queue BOOLEAN DEFAULT 0,
    created_at TEXT NOT NULL
);
```

#### 2. proxy_config

```sql
CREATE TABLE proxy_config (
    app_type TEXT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    auto_failover_enabled BOOLEAN NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    streaming_first_byte_timeout INTEGER NOT NULL DEFAULT 60,
    streaming_idle_timeout INTEGER NOT NULL DEFAULT 120,
    non_streaming_timeout INTEGER NOT NULL DEFAULT 600,
    circuit_failure_threshold INTEGER NOT NULL DEFAULT 5,
    circuit_success_threshold INTEGER NOT NULL DEFAULT 2,
    circuit_timeout_seconds INTEGER NOT NULL DEFAULT 30,
    circuit_error_rate_threshold REAL NOT NULL DEFAULT 0.5,
    circuit_min_requests INTEGER NOT NULL DEFAULT 10
);
```

#### 3. failover_queue

```sql
CREATE TABLE failover_queue (
    app_type TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    sort_index INTEGER NOT NULL,
    PRIMARY KEY (app_type, provider_id)
);
```

#### 4. provider_health

```sql
CREATE TABLE provider_health (
    provider_id TEXT NOT NULL,
    app_type TEXT NOT NULL,
    is_healthy BOOLEAN NOT NULL DEFAULT 1,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_success_at TEXT,
    last_failure_at TEXT,
    last_error TEXT,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (provider_id, app_type)
);
```

#### 5. usage_logs

```sql
CREATE TABLE usage_logs (
    id TEXT PRIMARY KEY,
    provider_id TEXT NOT NULL,
    app_type TEXT NOT NULL,
    model TEXT NOT NULL,
    request_model TEXT NOT NULL,
    pricing_model TEXT NOT NULL,
    input_tokens INTEGER NOT NULL,
    output_tokens INTEGER NOT NULL,
    total_tokens INTEGER NOT NULL,
    cache_creation_input_tokens INTEGER,
    cache_read_input_tokens INTEGER,
    input_cost REAL NOT NULL,
    output_cost REAL NOT NULL,
    total_cost REAL NOT NULL,
    latency_ms INTEGER NOT NULL,
    first_token_ms INTEGER,
    status_code INTEGER NOT NULL,
    session_id TEXT,
    is_streaming BOOLEAN NOT NULL,
    created_at TEXT NOT NULL
);
```

### 关键配置

#### 1. 默认端口

```rust
pub const DEFAULT_PROXY_PORT: u16 = 15721;
```

#### 2. 超时配置

```rust
// 流式首字超时：等待首个数据块的最大时间
pub const DEFAULT_STREAMING_FIRST_BYTE_TIMEOUT: u64 = 60; // 秒

// 流式静默超时：两个数据块之间的最大间隔
pub const DEFAULT_STREAMING_IDLE_TIMEOUT: u64 = 120; // 秒

// 非流式总超时：非流式请求的总超时时间
pub const DEFAULT_NON_STREAMING_TIMEOUT: u64 = 600; // 秒（10 分钟）
```

#### 3. 熔断器配置

```rust
pub const DEFAULT_CIRCUIT_FAILURE_THRESHOLD: u32 = 5;      // 连续失败 5 次触发熔断
pub const DEFAULT_CIRCUIT_SUCCESS_THRESHOLD: u32 = 2;      // 连续成功 2 次恢复
pub const DEFAULT_CIRCUIT_TIMEOUT_SECONDS: u64 = 30;       // 熔断 30 秒后进入探测
pub const DEFAULT_CIRCUIT_ERROR_RATE_THRESHOLD: f64 = 0.5; // 错误率 50% 触发熔断
pub const DEFAULT_CIRCUIT_MIN_REQUESTS: u32 = 10;          // 至少 10 个请求才计算错误率
```

#### 4. 请求体限制

```rust
pub const DEFAULT_BODY_LIMIT: usize = 200 * 1024 * 1024; // 200MB
```

### 特殊处理场景

#### 1. Header 大小写保留

- **场景**：Native Claude 客户端发送 `X-Api-Key`（非标准大小写）
- **实现**：Peek TCP 字节流 → `OriginalHeaderCases` → 转发时恢复
- **作用**：避免中转服务因 header 大小写不匹配拒绝请求

#### 2. Codex OAuth SSE 聚合

- **场景**：ChatGPT Plus OAuth 将 `stream:false` 强制升级为 SSE
- **实现**：`responses_sse_to_response_value()` 聚合 SSE 事件为 JSON
- **作用**：让非流式客户端收到正确的 JSON 响应

#### 3. 413 错误特殊处理

- **场景**：上游网关 413 Payload Too Large（nginx `client_max_body_size`）
- **实现**：错误消息明确指向上游限制，提供 `/compact` 等操作指引
- **作用**：避免用户误认为是本地代理的限制

#### 4. Media 降级

- **场景**：不支持图片的模型（如 DeepSeek V4 Pro）
- **实现**：预防性 + 反应式双路径，替换图片为 `[Unsupported Image]` 标记
- **作用**：让对话不中断，同时保留上下文

#### 5. Thinking 整流

- **场景**：Bedrock 不支持 thinking 块的 `signature` 字段
- **实现**：`thinking_rectifier::strip_invalid_thinking_signatures()`
- **作用**：避免 400 Bad Request

#### 6. Copilot 优化

- **场景**：GitHub Copilot 代理消耗量异常（Issue #1813）
- **实现**：
  - x-initiator 请求分类（区分测试连接）
  - Tool result 消息合并（减少冗余）
  - Tool 使用条件注入（仅 code_execution 场景启用）
- **作用**：大幅降低 token 消耗

### API 兼容性矩阵

| 客户端 | 上游 Provider | 格式转换 | 特殊处理 |
|--------|--------------|----------|----------|
| Claude Code | Anthropic 官方 | 无 | Header 大小写保留 |
| Claude Code | OpenRouter | Anthropic → Chat | 已支持 Claude 接口，默认透传 |
| Claude Code | GitHub Copilot | Anthropic → Chat | 动态 endpoint、模型解析 |
| Claude Code | Gemini | Anthropic → Gemini | Shadow Store、URL 构建策略 |
| Codex CLI | OpenAI 官方 | 无 | 透传 |
| Codex CLI | MiniMax | Responses → Chat → Responses | Tool Call 历史恢复 |
| Codex CLI | ChatGPT Plus OAuth | Responses → Responses | SSE 聚合（`stream:false`） |
| Gemini CLI | Google 官方 | 无 | OAuth Token、GET 端点支持 |
| Claude Desktop | 任意 Provider | 根据 Provider | 独立 Gateway Token 认证 |

### 测试覆盖

#### 单元测试（`src-tauri/src/proxy/*.rs`）

- 熔断器状态机
- 格式转换器（Anthropic ↔ OpenAI ↔ Gemini ↔ Responses）
- SSE 解析器
- 整流器逻辑
- Provider 选择逻辑
- 响应预检逻辑

#### 集成测试（`tests/`）

- 端到端请求流程
- 故障转移场景
- 熔断器触发与恢复
- 流式响应处理
- 使用量统计

---

## 总结

CC-Switch 的本地路由实现是一个完整的企业级 HTTP 代理服务器，具备以下核心能力：

1. **多协议支持**：Claude、Codex、Gemini 三大主流 API 协议
2. **智能格式转换**：Anthropic ↔ OpenAI ↔ Gemini 多向转换
3. **故障转移**：自动重试 + 熔断器 + 健康检查
4. **请求整流**：Thinking、Media、Budget 等多种整流器
5. **性能优化**：Bedrock 优化、Copilot 优化
6. **使用量统计**：流式/非流式统一追踪 + 成本计算
7. **Header 保真**：原始大小写保留，兼容非标准客户端
8. **响应预检**：防止上游中途断开导致的误判

所有处理流程都经过精心设计，确保在各种边缘场景下都能正确工作，同时保持代码的可维护性和可扩展性。
