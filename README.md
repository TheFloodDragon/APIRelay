# APIRelay

一个 **API 聚合中转站**：对外暴露统一的大模型 API，对内将请求转发/聚合到多个上游渠道。
中转站对外协议与上游渠道协议**完全解耦**，二者通过内部规范中枢（IR）互相转换。

> 设计借鉴 [new-api](https://github.com/QuantumNous/new-api) 的 `Adaptor`/`APIType`/`EndpointType` 协议解耦机制，
> 以及 sub2api 的协议互转矩阵与故障转移思路。

## 特性

- **基本模型转发**：兼容 OpenAI `/v1/chat/completions`，支持流式 SSE 与非流式。
- **协议解耦**：对外协议（`EndpointType`）与上游协议（`APIType`）分别实现，通过统一 `Adaptor` 接口 + IR 中枢互转。
- **模型聚合**：基于 `(group, model)` 倒排索引，按 `priority` 分层 + `weight` 加权随机选择渠道；支持模型映射/重定向。
- **故障转移**：多渠道重试 + 失败渠道冷却。
- **令牌鉴权**：sha256 存储、模型白名单、额度与过期校验。
- **完善日志**：zap 结构化运行日志（request_id 贯穿）+ 调用日志落库（上下游协议、模型、token 数、耗时、首字节延迟、状态码、错误）。
- **前后端分离**：REST 管理 API（`/api/*`）。
- **单体部署**：纯 Go SQLite（无需 CGO），可交叉编译为单文件二进制。

## 快速开始

```bash
cp config.example.yaml config.yaml
go run .          # 或 go build -o apirelay . && ./apirelay
```

首次启动会创建管理员（admin/admin123）与一个 root 令牌，并打印在日志中。

### 配置一个渠道

```bash
curl -X POST http://127.0.0.1:3000/api/channels -H 'Content-Type: application/json' -d '{
  "name": "openai-main",
  "type": 1,
  "base_url": "https://api.openai.com",
  "key": "sk-xxx",
  "group": "default",
  "models": "gpt-4o,gpt-4o-mini",
  "weight": 1
}'
```

### 发起请求

```bash
curl -X POST http://127.0.0.1:3000/v1/chat/completions \
  -H "Authorization: Bearer <root-token>" -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}'
```

## 目录结构

| 目录 | 职责 |
|---|---|
| `common/` | 配置、zap 日志、工具 |
| `constant/` | `endpoint_type`（对外协议）/ `api_type`（上游协议）双枚举 |
| `model/` | GORM 模型：Channel / Ability / Token / Log / User |
| `dto/` | `ir.go` 内部规范中枢 + 各协议结构 |
| `relay/adaptor/` | `Adaptor` 接口与各上游适配器 |
| `relay/apicompat/` | 协议 ↔ IR 互转 |
| `relay/relayer.go` | 转发主流程 + 故障转移 |
| `middleware/` | 鉴权 / request_id / recover / CORS |
| `controller/` | 管理后台 REST |
| `router/` | 路由装配 |

## 协议解耦原理

```
对外请求 ──(handler 解析)──► UnifiedRequest(IR) ──Adaptor.ConvertRequest──► 上游协议请求
上游响应/SSE ──Adaptor──► UnifiedResponse/StreamChunk(IR) ──(handler 序列化)──► 对外协议响应
```

新增一种协议 = 1 个入站解析 + 1 个出站序列化，避免 N² 互转。

## 路线图

- [x] 阶段1-2：骨架 + OpenAI 转发 MVP（含 SSE、调用日志、故障转移）
- [ ] 阶段3：Anthropic / OpenAI Responses 适配器 + 跨协议互转
- [ ] 阶段4：模型聚合增强 + 故障转移精细化
- [ ] 阶段5：额度预扣/结算 + 异步用量 + 管理后台鉴权
- [ ] 阶段6：Vue3 前端（embed）

## License

MIT
