# 任务拆解与验收标准

## 1. Epic 列表

### EPIC-01 初始化 ForkTool CLI

目标：

- 建立可运行仓库
- 建立 CLI 骨架、配置、日志、错误码

输出：

- `forktool version`
- `forktool init`

### EPIC-02 实现 manifest 与 baseline 管理

目标：

- 能定义功能链
- 能校验官方仓身份

输出：

- manifest parser
- baseline verify
- run context

### EPIC-03 实现 Go 链路提取

目标：

- 能从 Go 仓库中提取链路节点

输出：

- route / handler / service / helper / tests

### EPIC-04 实现规则引擎 MVP

目标：

- 能对关键语义差异做判断

### EPIC-05 实现前端 / 配置 / SQL 扩展

目标：

- 支持 Vue/TS
- 支持 config chain
- 支持 migration chain

### EPIC-06 实现批量扫描与集成

目标：

- 支持 release 扫描
- 输出 Markdown / JSON / task draft

## 2. 第一批任务拆解

## 2.1 CLI 与基础设施

### TASK-001 建立 Go module 与仓库结构

验收：

- 仓库目录结构与设计文档一致
- `go test ./...` 可运行

### TASK-002 实现 CLI 主命令与子命令骨架

验收：

- `forktool init`
- `forktool doctor`
- `forktool scan`

### TASK-003 实现配置加载与 workspace 初始化

验收：

- 能生成 `.forktool/config.yaml`
- 能创建 runs/cache 目录

## 2.2 Manifest 与 baseline

### TASK-010 实现 manifest schema 与校验

验收：

- 能解析示例 manifest
- 错误字段能给出清晰提示

### TASK-011 实现 baseline verify

验收：

- 能校验官方仓路径、remote、tag、commit
- 失败时 exit code 非 0

### TASK-012 实现 run context 输出

验收：

- 每次运行都写出 `context.json`

## 2.3 Go Adapter

### TASK-020 解析 gin 路由注册

验收：

- 能从 `routes/*.go` 找出 manifest 指定路径

### TASK-021 解析 handler / service / helper 符号节点

验收：

- 能输出带文件位置的统一 IR

### TASK-022 解析测试文件引用

验收：

- 能识别 manifest 对应测试是否存在

## 2.4 规则引擎 MVP

### TASK-030 实现 `claude-metadata-userid-format`

验收：

- 能识别是否使用 `FormatMetadataUserID`
- 能识别手写 legacy 变体

### TASK-031 实现 `claude-session-hash-normalization`

验收：

- 能识别是否调用 `NormalizeSessionUserAgent`

### TASK-032 实现 `claude-count-tokens-beta-suffix`

验收：

- 能识别 URL 是否保留 `?beta=true`

### TASK-033 实现 `http-header-wire-casing`

验收：

- 能识别 `resolveWireCasing + addHeaderRaw`
- 能识别直接 `Header.Add` 的偏离

### TASK-034 实现 `openai-compact-path-suffix`

验收：

- 能识别 `/responses/compact` suffix 是否落到上游 URL

### TASK-035 实现 `openai-session-isolation`

验收：

- 能识别是否使用 `isolateOpenAISessionID`
- 能识别官方 session isolation 测试缺失

### TASK-036 实现 `openai-passthrough-body-normalization`

验收：

- 能区分 compact / non-compact 的 body normalize 行为

### TASK-037 实现 `gemini-upstream-model-preserved`

验收：

- 能识别 `ForwardResult.UpstreamModel` 是否被丢失

### TASK-038 实现 `gemini-digest-prefix-ua-normalization`

验收：

- 能识别 Gemini digest 是否使用 UA normalization

## 2.5 报告层

### TASK-040 Markdown 报告渲染

验收：

- 报告可读，带功能汇总与 finding 列表

### TASK-041 JSON 报告渲染

验收：

- 结构稳定，可供其他工具消费

### TASK-042 风险汇总与 exit code

验收：

- `high/critical` 会影响退出码

## 2.6 扩展适配器

### TASK-050 TS/Vue 路由与页面抽取

验收：

- 能识别页面、路由、导航入口三类节点

### TASK-051 Config chain 抽取

验收：

- 能识别 config key / setting handler / frontend form 的存在性

### TASK-052 SQL migration 抽取

验收：

- 能识别 feature 命中的迁移脚本

## 3. 第一批功能验收集

ForkTool 第一版至少要能稳定扫描下面 5 条：

- `claude-messages-mainchain`
- `claude-count-tokens`
- `openai-responses-http`
- `openai-responses-compact`
- `gemini-native-v1beta`

每条功能至少要满足：

- 能找到本地链路节点
- 能找到官方链路节点
- 能输出至少 1 条真实 finding
- 能给出建议动作

## 4. 文档与实现同步要求

ForkTool 开发过程中，必须同步维护：

- manifest 示例
- 报告样例
- 已支持规则清单
- 已支持语言适配器清单

否则工程师只看代码，很快又会回到“靠猜工具到底能干什么”。

## 5. 验收标准总表

### MVP 验收

- CLI 可运行
- baseline verify 可用
- manifest validate 可用
- Go 主链扫描可用
- 10 条关键规则可用
- Markdown / JSON 报告可用

### V1 验收

- 增加 Vue/TS / SQL / config 支持
- release 批量扫描可用
- 与现有同步流程可对接

## 6. 风险清单

### 风险 A：manifest 维护成本过高

缓解：

- 先只维护高风险功能
- 提供示例和模板

### 风险 B：规则误报太多

缓解：

- 每条规则必须先在真实样例上打磨
- 先上线高确定性规则

### 风险 C：树解析器兼容性问题

缓解：

- 允许“AST + 正则 + 手工 hint”的混合模式
- 先做最小稳定支持

## 7. 开放问题

这些问题不阻塞启动，但应尽早定：

- 是否把 GitNexus 作为可选增强依赖内置支持
- manifest 是放仓内还是单独仓维护
- 未来是否要支持自动生成 `bd` issue
- V2 是否要加入 patch 建议
