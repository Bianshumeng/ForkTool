# 功能设计

## 1. 功能总览

ForkTool 第一版围绕一个核心概念展开：

- `Feature Chain`，即“同一个功能从入口到行为落点的全链路定义”

所有功能都围绕这个对象工作。

## 2. 核心能力列表

### 2.1 功能 Manifest 管理

作用：

- 把“要比较什么”从代码里剥离出来
- 按功能维护，而不是按文件维护

Manifest 至少包含：

- 功能 ID
- 功能名称
- 风险等级
- 适用语言
- 链路节点定义
- 必须检查的语义规则
- 允许保留的本地 hook
- 必跑测试

这让 ForkTool 可以明确知道：

- `claude-count-tokens`
- `openai-responses-compact`
- `gemini-native-v1beta`

这些不是一堆文件，而是可审计的功能对象。

### 2.2 双仓基线校验

作用：

- 确保比较对象真的是“本地仓 vs 官方指定版本”
- 避免拿错官方仓、错 tag、错 remote

最低要求：

- 校验官方仓路径
- 校验 remote URL
- 校验 target tag / commit
- 输出一次扫描上下文快照

### 2.3 链路节点发现

对每个功能，工具需要自动或半自动发现节点。

节点类型包括：

- 路由
- Handler / Controller
- Service / Domain logic
- Helper / Transformer / Compat
- Config key
- Migration
- 前端页面
- 前端路由
- 导航入口
- API / DTO / 类型
- 测试
- 文档/决策引用

### 2.4 语义级差异对比

这是整个工具最值钱的地方。

它不只比较文本，还比较这些语义：

#### 请求层

- URL / path suffix
- method
- query 参数
- header 白名单
- header casing
- 特殊 header 的注入 / 覆盖 / 删除
- request body 字段
- compact / stream / store 变体

#### 会话与路由层

- session hash 生成
- sticky session 绑定
- conversation/session id 隔离
- prompt cache key 参与方式
- metadata.user_id 格式

#### 响应层

- response header 过滤
- SSE / JSON 流式形态
- 错误响应结构
- passthrough 行为

#### 观测层

- `requested_model`
- `upstream_model`
- `endpoint`
- `request_type`
- `upstream_url`
- `service_tier`

#### 配置层

- config struct 是否存在
- setting key 是否存在
- handler DTO 是否透出
- frontend 表单是否消费
- service 是否真正使用

#### 测试层

- 官方测试是否存在
- 本地测试是否存在
- 本地是否缺少关键保护测试

### 2.5 决策标签输出

每一条差异都不只是 `diff`，而要带决策标签。

首批标签建议：

- `official-required`
  表示该差异属于必须官方化的链路偏离
- `keep-local-hook`
  表示该差异命中本地保留语义
- `manual-merge`
  表示必须人工判断和整合
- `test-missing`
  表示官方已有关键测试，本地缺失
- `config-chain-check`
  表示配置生效链需要额外核验
- `observability-drift`
  表示观测字段或埋点语义已漂移

### 2.6 报告生成

输出至少包含两种格式：

- Markdown
- JSON

Markdown 给人看，JSON 给其他工具消费。

报告最小结构：

- 扫描对象
- 官方基线
- 功能摘要
- 风险汇总
- 按功能列出差异
- 每条差异的证据、标签、建议动作

### 2.7 任务系统集成

第一版不强绑具体任务系统，但要预留导出：

- `bd` issue note
- issue 草稿
- PR review comment 草稿

## 3. 多语言支持策略

## 3.1 不是“全语言支持”，而是“当前技术栈优先”

第一版支持范围：

- Go
- TypeScript
- Vue SFC
- SQL
- YAML
- JSON
- Markdown

不做：

- Python / Rust / Java 的一开始就上

## 3.2 语言适配策略

### Go

提取内容：

- `gin` 路由注册
- handler 方法
- service 方法
- helper / compat / transformer
- 单测 / 集成测试
- config struct / setting service 调用

### TS / Vue

提取内容：

- Vue 路由
- 页面组件
- layout / nav 入口
- API 调用
- DTO / 类型定义
- store / composable
- i18n key 引用

### SQL

提取内容：

- migration 文件
- 受影响表 / 列 / 索引
- 与 feature manifest 的 config / field 绑定关系

### YAML / JSON / env

提取内容：

- 配置键
- 默认值
- feature flag
- response header 过滤规则等结构化配置

### Markdown

提取内容：

- 本地二开台账
- 同步决策矩阵
- 保护区说明

## 4. 首批重点功能设计

## 4.1 `claude-messages-mainchain`

关注点：

- `/v1/messages` 路由
- `GatewayHandler.Messages`
- `GatewayService.Forward`
- `GenerateSessionHash`
- `buildOAuthMetadataUserID`
- `responseheaders`
- `gateway_request`
- 相关测试

重点规则：

- `metadata.user_id` 格式必须与官方一致
- session hash 组成必须与官方一致
- allowed header / wire casing 必须与官方一致

## 4.2 `claude-count-tokens`

关注点：

- `count_tokens` URL
- `beta=true` 语义
- header passthrough
- request body model mapping
- 404 fallback 语义

重点规则：

- 路径是否包含 `?beta=true`
- passthrough header 是否按官方 casing/raw 写入
- 官方测试是否存在、本地是否同步

## 4.3 `openai-responses-http`

关注点：

- `buildUpstreamRequest`
- `buildUpstreamRequestOpenAIPassthrough`
- OAuth / API Key 分支
- passthrough header
- `store / stream` 正规化
- `originator / accept / OpenAI-Beta`

重点规则：

- session / conversation 头是否隔离
- compact 与非 compact 是否分流
- request suffix 是否保留

## 4.4 `openai-responses-compact`

关注点：

- path suffix `/compact`
- compact body 归一化
- compact session id
- compact 专项测试

重点规则：

- 是否会删除 `store` / `stream`
- 是否只保留允许字段
- 是否把 `/responses/compact` 正确映射到上游

## 4.5 `openai-responses-ws`

关注点：

- WS ingress
- reconnect / fallback
- previous_response_id
- turn metadata replay
- store=false 约束

重点规则：

- HTTP / WS 模式切换规则
- reconnect 时 previous_response_id 处理
- upstream metadata replay 行为

## 4.6 `gemini-native-v1beta`

关注点：

- handler 入口
- account failover
- digest/session
- native response header filter
- native upstream error passthrough

重点规则：

- digest prefix 是否和官方一致
- failover 后是否保留正确观测字段
- response header filter 是否按官方走

## 4.7 `gemini-messages-compat`

关注点：

- Gemini -> Claude compat
- `ForwardResult.UpstreamModel`
- `thoughtSignature`
- usage 提取
- native/compat 错误写回

重点规则：

- `UpstreamModel` 是否丢失
- compat 结果体中的 usage 字段是否与官方一致
- 相关测试是否同步

## 5. 风险模型

建议把每条差异按下面四级输出：

- `critical`
  核心协议语义错误，影响生产行为
- `high`
  主要链路偏离官方，影响同步可信度
- `medium`
  测试、观测、配置链不完整
- `low`
  注释、日志、轻微结构差异

### 差异升级规则

- 命中请求头 / 请求体 / session / metadata / compact / upstream_model -> 至少 `high`
- 命中路由、handler 入口、failover 关键分支 -> 至少 `high`
- 仅命中日志、注释、变量名 -> `low`

## 6. 输出建议动作

每条差异必须给建议动作，不要只报告问题。

动作类型建议：

- `replace-with-official`
- `keep-local-hook`
- `merge-official-into-local-hook`
- `add-missing-test`
- `verify-config-chain`
- `verify-observability-chain`
- `defer-with-tech-debt`

## 7. 第一版不做的高级功能

这些功能可以进入后续版本，但不要抢第一版资源：

- 自动 patch 生成
- 交互式 TUI
- 可视化功能链图谱
- AI 自动生成 merge patch
- 跨版本自动 release note 聚合
