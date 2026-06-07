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

- [ ] 启动后端：`./apirelay --config config.yaml.example`
- [ ] 确认服务监听 `http://localhost:8080`
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
   curl -X POST http://localhost:8080/api/channels \
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
   curl http://localhost:8080/api/channels \
     -H "Authorization: Bearer $ADMIN_KEY"
   ```

3. **测试渠道连接**（需要真实 API Key）：
   ```bash
   curl -X POST http://localhost:8080/api/channels/1/test \
     -H "Authorization: Bearer $ADMIN_KEY"
   ```

4. **自动获取模型**：
   ```bash
   curl -X POST http://localhost:8080/api/channels/1/models \
     -H "Authorization: Bearer $ADMIN_KEY"
   ```

5. **批量调整优先级**：
   ```bash
   curl -X PUT http://localhost:8080/api/channels/reorder \
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
   curl http://localhost:8080/v1/models \
     -H "Authorization: Bearer $ADMIN_KEY"
   ```

2. **聊天补全**（需要真实上游 API）：
   ```bash
   curl -X POST http://localhost:8080/v1/chat/completions \
     -H "Authorization: Bearer $ADMIN_KEY" \
     -H "Content-Type: application/json" \
     -d '{
       "model": "gpt-3.5-turbo",
       "messages": [{"role": "user", "content": "Hello"}]
     }'
   ```

3. **检查请求日志**：
   ```bash
   curl http://localhost:8080/api/logs \
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

## 阶段二：核心功能增强（3-5 天）

**目标**：补齐 MVP 缺失的关键能力。

### 2.1 流式响应支持（高优先级）

**背景**：当前 `/v1/chat/completions` 只支持普通 JSON，不支持 `stream: true`。

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
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Count to 5"}],
    "stream": true
  }'
```

应实时输出 `data: {...}` 格式的流。

### 2.2 高级调度策略

**当前**：只支持 `priority` 按优先级选择。

**新增**：

- **weighted**：按权重加权随机
  - 实现公式：`rand.Intn(totalWeight)` 映射到渠道
- **round_robin**：轮询选择
  - 使用 Redis 或内存计数器记录上次选择的渠道索引

**配置**：
```yaml
scheduler:
  strategy: weighted  # priority | weighted | round_robin
```

**验证**：添加多个权重不同的渠道，发送多次请求，观察日志分布是否符合权重比例。

### 2.3 定时健康检查

**实现**：

1. 在 `main.go` 启动时启动 goroutine
2. 每隔 `scheduler.health_check_interval` 秒（默认 60s）
3. 遍历所有启用渠道，调用 `/models` 接口
4. 成功：`health_status = healthy`
5. 失败：累计失败次数，达到 `unhealthy_threshold` 后标记为 `unhealthy`
6. 更新 `last_check` 时间戳

**优化**：不健康的渠道不参与调度，但仍继续检查，恢复后自动重新启用。

### 2.4 协议适配器（Anthropic/Gemini）

**当前**：所有渠道都按 OpenAI 格式转发。

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
- 设置限流、允许模型、IP 白名单
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
- [ ] 渠道选择结果缓存（Redis，1 分钟 TTL）

### 4.2 安全加固

- [ ] 强制修改默认 `admin_key`（启动时检测并警告）
- [ ] API Key 存储使用 bcrypt 哈希
- [ ] 请求日志脱敏（不记录完整 API Key）
- [ ] CORS 配置生产环境白名单
- [ ] 添加请求体大小限制（防止 DoS）

### 4.3 Redis 限流

**实现**：

1. 连接 Redis
2. 使用 `redis.Incr` + `redis.Expire` 实现滑动窗口
3. 中间件检查：
   - 全局限流：`rate_limit.global`
   - 每个 API Key 限流：`rate_limit.per_key`
4. 超限返回 `429 Too Many Requests`

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

### 4.5 Docker 生产镜像优化

- [ ] 多阶段构建：分离前端构建和后端构建
- [ ] 使用 `alpine` 基础镜像减小体积
- [ ] 健康检查：
  ```dockerfile
  HEALTHCHECK --interval=30s --timeout=3s \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/system/health || exit 1
  ```

### 4.6 文档编写

**README.md**：

```markdown
# APIRelay

API 调度中心，支持多渠道统一管理和 OpenAI 兼容转发。

## 快速开始

### Docker 部署（推荐）

1. 克隆仓库：
   ```bash
   git clone https://github.com/yourusername/apirelay.git
   cd apirelay
   ```

2. 修改配置：
   ```bash
   cp config.yaml.example config.yaml
   vim config.yaml  # 修改 admin_key
   ```

3. 启动服务：
   ```bash
   docker-compose up -d
   ```

4. 访问管理后台：
   ```
   http://localhost:8080
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
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer YOUR_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

## 配置说明

见 `config.yaml.example`。

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

### 5.2 PostgreSQL 支持

- 添加 `gorm.io/driver/postgres`
- 配置：
  ```yaml
  database:
    type: postgres
    dsn: "host=localhost user=xxx password=xxx dbname=apirelay"
  ```

### 5.3 用户体系（多租户）

- 添加 `users` 表
- 每个用户独立管理渠道和 API Key
- JWT 登录认证

### 5.4 计费与额度

- 添加 `quotas` 表
- 记录每个 API Key 的 Token 消耗
- 超额自动禁用

### 5.5 渠道预设模板

参考 CCSwitch，预设 50+ 常见渠道模板（OpenAI、Claude、Gemini、DeepSeek、讯飞星火等），一键添加。

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

1. **前端构建产物未集成到后端**：
   - 当前 Dockerfile 和 workflow 未构建前端
   - 需要在构建时将 `web/dist` 复制到后端静态文件目录

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
