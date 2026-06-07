# APIRelay 聚合中转站功能设计文档

## 概述

参考 cc-switch 的本地路由功能，APIRelay 实现了完整的聚合中转站能力，支持多协议适配、模型路由和智能调度。

## 核心功能

### 1. 模型路由（Model Routing）

#### 1.1 模型别名（Alias）

**功能**：为模型设置别名，简化模型名称或统一命名规范。

**示例**：
```json
{
  "alias": "gpt-4-turbo",
  "real_model": "gpt-4-turbo-2024-04-09"
}
```

**API**：
- 设置别名：`POST /api/routes/aliases`
- 删除别名：`DELETE /api/routes/aliases/:alias`

**使用场景**：
- 简化长模型名：`claude-3-opus-20240229` → `claude-opus`
- 版本统一：所有 `gpt-4` 请求统一到最新版本

#### 1.2 模型重定向（Redirect）

**功能**：将请求的模型自动重定向到另一个模型。

**示例**：
```json
{
  "source_model": "gpt-4",
  "target_model": "claude-3-opus-20240229"
}
```

**API**：
- 设置重定向：`POST /api/routes/redirects`
- 删除重定向：`DELETE /api/routes/redirects/:source`

**使用场景**：
- 成本优化：将昂贵的 GPT-4 请求转到更便宜的 Claude
- 供应商切换：临时切换到备用供应商
- A/B 测试：对比不同模型的效果

**防护机制**：
- 自动检测循环重定向（A→B→A）
- 最大重定向深度限制

#### 1.3 模型组（Group）

**功能**：将多个模型组合成一个虚拟模型，实现负载均衡。

**示例**：
```json
{
  "group_name": "fast-model",
  "models": ["gpt-3.5-turbo", "claude-3-haiku", "gemini-pro"]
}
```

**API**：
- 设置模型组：`POST /api/routes/groups`
- 删除模型组：`DELETE /api/routes/groups/:group`

**使用场景**：
- 负载均衡：在多个快速模型间分散请求
- 高可用性：某个模型失败时自动切换到其他模型
- 成本优化：混合使用不同价格的模型

**调度策略**：
- 继承全局调度策略（priority/weighted/round_robin）
- 支持渠道级别的优先级和权重

### 2. 协议适配器（Protocol Adapter）

#### 2.1 设计架构

```
客户端请求（OpenAI 格式）
    ↓
模型路由（别名/重定向/组）
    ↓
选择渠道（调度器）
    ↓
协议适配器
    ↓ 请求转换
上游 API（OpenAI/Anthropic/Gemini）
    ↓ 响应转换
协议适配器
    ↓
统一 OpenAI 格式返回
```

#### 2.2 支持的协议

**OpenAI Compatible（直通）**：
- OpenAI
- DeepSeek
- 其他 OpenAI 兼容接口

**Anthropic Claude**：
- 请求转换：
  - 提取 `system` 消息到独立字段
  - 添加必需的 `max_tokens` 参数
  - 转换 `stop` 参数为 `stop_sequences`
- 响应转换：
  - `content` 数组提取为纯文本
  - `stop_reason` 映射为 OpenAI 格式
  - Token 使用量字段转换

**Google Gemini**：
- 请求转换：
  - `assistant` 角色转为 `model`
  - `system` 消息合并到首条 `user` 消息
  - 参数名称转换（`topP` → `top_p`）
  - 设置安全设置为最宽松
- 响应转换：
  - 提取 `parts` 中的文本
  - `finishReason` 映射为 OpenAI 格式

#### 2.3 流式响应支持

**Anthropic SSE 转换**：
- 解析事件类型：`message_start`、`content_block_delta`、`message_delta`
- 提取增量文本
- 转换为 OpenAI 流式格式

**Gemini JSON Lines 转换**：
- 按行解析 JSON 对象
- 提取 `candidates` 中的文本
- 包装为 SSE 格式

### 3. 智能调度

#### 3.1 路由解析流程

```
用户请求：model="gpt-4"
    ↓
1. 检查别名：gpt-4 → gpt-4-turbo（如果设置）
    ↓
2. 检查重定向：gpt-4-turbo → claude-3-opus（如果设置）
    ↓
3. 检查模型组：claude-3-opus → [opus-ch1, opus-ch2]（如果设置）
    ↓
4. 查找支持的渠道
    ↓
5. 应用调度策略（priority/weighted/round_robin）
    ↓
6. 选择最终渠道并发送请求
```

#### 3.2 失败重试

- 按优先级或调度策略依次尝试所有可用渠道
- 记录每次尝试的日志和错误信息
- 流式请求：一旦开始传输数据则不再重试

### 4. 配置管理

#### 4.1 持久化存储

路由配置存储在 `system_config` 表：

```sql
CREATE TABLE system_config (
  key VARCHAR(100) PRIMARY KEY,
  value TEXT,
  updated_at DATETIME
);

-- 存储示例
INSERT INTO system_config VALUES
  ('model_routes', '{"aliases":{...},"redirects":{...},"groups":{...}}', NOW());
```

#### 4.2 运行时更新

- 配置修改即时生效，无需重启服务
- 支持手动重载：`POST /api/routes/reload`
- 线程安全：使用读写锁保护路由表

## 使用示例

### 示例 1：多供应商高可用

**场景**：使用 OpenAI 格式调用，自动在多个供应商间切换。

**配置**：
```bash
# 1. 添加多个渠道
POST /api/channels
{
  "name": "OpenAI GPT-4",
  "type": "openai",
  "api_key": "sk-xxx",
  "base_url": "https://api.openai.com/v1",
  "models": ["gpt-4"],
  "priority": 10
}

POST /api/channels
{
  "name": "Claude Opus",
  "type": "anthropic",
  "api_key": "sk-ant-xxx",
  "base_url": "https://api.anthropic.com/v1",
  "models": ["claude-3-opus-20240229"],
  "priority": 8
}

# 2. 设置模型组
POST /api/routes/groups
{
  "group_name": "best-model",
  "models": ["gpt-4", "claude-3-opus-20240229"]
}
```

**调用**：
```bash
curl -X POST http://localhost:15722/v1/chat/completions \
  -H "Authorization: Bearer YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "best-model",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

**效果**：
- 优先使用 OpenAI GPT-4（优先级 10）
- GPT-4 失败时自动切换到 Claude Opus（优先级 8）
- 响应格式始终为 OpenAI 标准格式

### 示例 2：成本优化

**场景**：将部分 GPT-4 请求重定向到更便宜的模型。

**配置**：
```bash
# 设置重定向
POST /api/routes/redirects
{
  "source_model": "gpt-4",
  "target_model": "claude-3-sonnet-20240229"
}
```

**效果**：
- 所有请求 `gpt-4` 的调用自动转到 Claude Sonnet
- 节省约 60% 成本
- 对客户端完全透明

### 示例 3：简化模型名

**场景**：统一使用简短的模型名称。

**配置**：
```bash
POST /api/routes/aliases
{"alias": "gpt4", "real_model": "gpt-4-turbo-2024-04-09"}

POST /api/routes/aliases
{"alias": "claude", "real_model": "claude-3-opus-20240229"}

POST /api/routes/aliases
{"alias": "gemini", "real_model": "gemini-pro"}
```

**调用**：
```bash
# 使用简短名称
curl ... -d '{"model": "gpt4", ...}'
curl ... -d '{"model": "claude", ...}'
curl ... -d '{"model": "gemini", ...}'
```

## API 接口汇总

### 路由配置

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/routes` | 获取所有路由配置 |
| POST | `/api/routes/aliases` | 设置模型别名 |
| DELETE | `/api/routes/aliases/:alias` | 删除模型别名 |
| POST | `/api/routes/redirects` | 设置模型重定向 |
| DELETE | `/api/routes/redirects/:source` | 删除模型重定向 |
| POST | `/api/routes/groups` | 设置模型组 |
| DELETE | `/api/routes/groups/:group` | 删除模型组 |
| POST | `/api/routes/reload` | 重新加载路由配置 |

### 请求示例

**获取所有路由**：
```bash
GET /api/routes
{
  "aliases": {
    "gpt4": "gpt-4-turbo-2024-04-09",
    "claude": "claude-3-opus-20240229"
  },
  "redirects": {
    "gpt-4": "claude-3-sonnet-20240229"
  },
  "groups": {
    "fast-model": ["gpt-3.5-turbo", "claude-3-haiku"]
  }
}
```

## 技术优势

### 1. 统一接口

- 客户端只需支持 OpenAI 格式
- 自动适配不同供应商的协议
- 降低集成成本

### 2. 高可用性

- 自动失败重试
- 多渠道负载均衡
- 健康检查自动屏蔽不可用渠道

### 3. 灵活配置

- 运行时动态修改路由规则
- 支持复杂的映射关系
- 配置持久化存储

### 4. 性能优化

- 内存缓存路由表
- 读写锁保证并发安全
- 最小化数据库访问

## 未来扩展

1. **条件路由**：根据请求内容、用户、时间等条件选择不同路由
2. **流量分配**：支持 A/B 测试的流量百分比分配
3. **熔断降级**：渠道异常时自动降级到备用方案
4. **成本追踪**：统计不同路由的 Token 消耗和成本
5. **智能路由**：基于历史数据自动选择最优渠道

## 总结

APIRelay 的聚合中转站功能提供了完整的多供应商管理能力：

- ✅ 模型别名、重定向、分组
- ✅ 多协议自动适配（OpenAI/Anthropic/Gemini）
- ✅ 流式响应支持
- ✅ 智能调度和失败重试
- ✅ 运行时配置更新

这使得 APIRelay 可以作为统一的 AI API 网关，简化多模型、多供应商的管理和调用。
