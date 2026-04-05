# 实施计划与开发路线

## 1. 总体策略

ForkTool 的开发必须遵循“小步快跑，但每一步都能交付”的节奏。

这不是一个适合一次性做大的项目。正确路径是：

1. 先有能跑的 CLI 骨架
2. 再有 manifest
3. 再有 Go 主链扫描
4. 再有规则引擎
5. 再补前端与配置链
6. 最后再考虑高级集成

## 2. 推荐阶段划分

## 阶段 0：项目初始化

目标：

- 建立 ForkTool 仓库
- 跑通 CLI 骨架
- 建立目录结构、配置加载、日志、错误码、基础测试

交付物：

- 可运行的 `forktool version`
- 可运行的 `forktool init`
- 项目目录与基础 CI

建议时长：

- 1 到 2 天

## 阶段 1：Manifest 与基线校验

目标：

- 能加载 manifest
- 能校验官方仓身份与 tag
- 能生成一次 run context

交付物：

- `forktool manifest validate`
- `forktool baseline verify`
- `.forktool/runs/<run-id>/context.json`

建议时长：

- 2 到 3 天

## 阶段 2：Go 链路抽取 MVP

目标：

- 先只支持 Go
- 能从当前仓库中抽出：
  - 路由
  - handler
  - service
  - helper
  - tests

交付物：

- Go Adapter 初版
- 功能链节点抽取结果
- 单功能扫描命令可运行

建议时长：

- 4 到 6 天

这是第一版最关键的阶段。

## 阶段 3：规则引擎 MVP

目标：

- 对首批关键规则做确定性判断

首批规则建议：

- `claude-metadata-userid-format`
- `claude-session-hash-normalization`
- `claude-count-tokens-beta-suffix`
- `http-header-wire-casing`
- `openai-compact-path-suffix`
- `openai-session-isolation`
- `openai-passthrough-body-normalization`
- `gemini-upstream-model-preserved`
- `gemini-digest-prefix-ua-normalization`
- `test-file-presence`

交付物：

- `forktool scan feature <id>`
- Markdown / JSON 报告

建议时长：

- 5 到 7 天

## 阶段 4：TS/Vue + Config + SQL 扩展

目标：

- 把前端功能层、配置链、迁移链纳入

交付物：

- TS/Vue Adapter
- SQL Adapter
- Config Adapter
- 页面功能 parity 规则
- config chain 检查规则

建议时长：

- 1 到 2 周

## 阶段 5：批量 release 扫描与集成

目标：

- 支持一轮版本同步的批量审计
- 支持接入 `bd` / issue 草稿
- 支持可选使用 GitNexus / Repomix 增强证据

交付物：

- `forktool scan release`
- `forktool report render`
- optional integrations

建议时长：

- 1 周

## 3. 推荐开发顺序

工程实现建议严格按下面顺序，不要乱序：

1. CLI 骨架
2. config / workspace
3. manifest
4. baseline verify
5. Go Adapter
6. 统一 IR
7. 报告输出
8. 规则引擎
9. TS/Vue Adapter
10. Config / SQL / Markdown Adapter
11. 批量扫描
12. 外部集成

原因：

- 如果先做规则，没有 IR 承载，会不断返工
- 如果先做前端适配器，没有 Go 主链验证，很难确认工具路线是否正确
- 如果先做外部集成，会把核心设计拖偏

## 4. 仓库初始化建议

### 4.1 Go module

建议：

- Go 1.25+
- 单仓单 module

### 4.2 测试框架

建议：

- `testing`
- `testify`

### 4.3 CLI 框架

建议：

- `cobra`
- `viper`

### 4.4 解析依赖

建议：

- tree-sitter Go bindings
- 必要时少量正则与结构化解析

## 5. 开发中的工程守门

### 5.1 先做示例，再做泛化

例如：

- 先把 `openai-responses-compact` 扫对
- 再抽象成通用 `request-path-suffix` 规则

不要反过来。

### 5.2 先做确定性规则，再做模糊匹配

第一版优先规则：

- 是否有 `?beta=true`
- 是否调用 `NormalizeSessionUserAgent`
- 是否使用 `FormatMetadataUserID`
- 是否存在 `isolateOpenAISessionID`
- 是否存在官方测试文件

这些都能精确判断。

### 5.3 每个阶段都要有可演示成果

不要做 2 周“内部重构”没有产出。

每阶段至少能回答：

- 现在能扫哪些功能
- 能看见哪些差异
- 报告长什么样

## 6. 测试策略

## 6.1 单元测试

覆盖：

- manifest 解析
- path / route 识别
- rule matcher
- report renderer
- CLI 参数校验

## 6.2 夹具测试

建议建立 `testdata/`：

- `local/`
- `official/`
- `expected-report/`

对关键功能放最小仓库夹具，保证：

- 输入固定
- 输出可 snapshot

## 6.3 端到端测试

至少覆盖：

- `forktool baseline verify`
- `forktool scan feature claude-count-tokens`
- `forktool scan feature openai-responses-compact`
- `forktool scan feature gemini-native-v1beta`

## 6.4 回归测试

每增加一个规则，必须新增：

- 命中样例
- 不命中样例
- 报告 snapshot

## 7. 开发团队角色建议

如果有多人开发，建议按下面拆：

- 工程师 A：CLI / workspace / manifest / report
- 工程师 B：Go Adapter / 规则引擎 MVP
- 工程师 C：TS/Vue / Config / SQL 适配器
- reviewer / owner：规则边界、manifest 审核、验收

如果只有一人：

- 仍按模块顺序推进，不要同时铺三条线

## 8. 风险与应对

### 风险 1：功能定义过度泛化

表现：

- 工具试图自动理解所有功能
- manifest 形同虚设

应对：

- 先强依赖 manifest
- 先只纳管关键功能

### 风险 2：规则做得太弱，只是在包一层 diff

表现：

- 报告只是“文件不同”
- 不能指出协议差异

应对：

- 每条关键链必须有专属 semantic rule

### 风险 3：规则做得太重，第一版迟迟不可用

表现：

- 试图一次做完所有规则
- 没有 MVP

应对：

- 先做 10 条最关键规则

### 风险 4：前端支持拖慢第一版

应对：

- 第一版先以后端 Go 主链为核心
- 前端 parity 在第二阶段补入

## 9. 里程碑建议

### M0

- CLI + workspace + manifest 骨架完成

### M1

- Go 主链扫描可用
- 能扫 3 条关键功能链

### M2

- 报告可稳定输出
- 10 条关键规则可运行

### M3

- TS/Vue / Config / SQL 进入
- 页面 parity 和配置链开始可用

### M4

- release 批量扫描可用
- 可输出到任务系统草稿

## 10. 交付节奏建议

推荐按周交付：

- 第 1 周：M0 + M1
- 第 2 周：M2
- 第 3 周：M3
- 第 4 周：M4 与收尾

如果资源紧：

- 至少先交付 M2

因为只要 M2 在，工具就已经能直接为当前同步工作省时间。
