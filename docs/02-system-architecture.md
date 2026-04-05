# 系统架构设计

## 1. 设计总览

ForkTool 采用“语言无关核心 + 语言适配器 + 规则引擎 + 报告输出”的分层结构。

目标不是构建一个大型 IDE，而是提供一个面向同步场景的 CLI 审计器。

```text
                +---------------------------+
                |          CLI / API        |
                | init / baseline / scan    |
                +-------------+-------------+
                              |
                +-------------v-------------+
                |      Application Layer    |
                | workspace / manifest /    |
                | run orchestration         |
                +-------------+-------------+
                              |
      +-----------------------+------------------------+
      |                                                |
+-----v------+                                 +-------v-------+
| Discovery  |                                 | Rule Engine   |
| local/off. |                                 | semantic diff |
| extraction |                                 | decision tags |
+-----+------+                                 +-------+-------+
      |                                                |
      +-----------------------+------------------------+
                              |
                +-------------v-------------+
                |     Unified Chain IR      |
                | routes/symbols/tests/     |
                | config/migrations/docs    |
                +-------------+-------------+
                              |
      +-----------------------+------------------------+
      |                                                |
+-----v------+   +---------------+   +----------------v-----+
| Go Adapter  |   | TS/Vue Adapter|   | SQL/Config Adapter  |
+------------+   +---------------+   +----------------------+
                              |
                +-------------v-------------+
                | Report / Export Layer     |
                | markdown / json / bd /    |
                | exit code / cache         |
                +---------------------------+
```

## 2. 模块划分

### 2.1 CLI 层

职责：

- 解析命令行参数
- 加载 workspace 配置
- 选择运行模式
- 控制输出格式与 exit code

推荐技术：

- Go
- `cobra` 作为命令树框架
- `pflag` / `viper` 处理参数与配置

选择 Go 的原因：

- 当前仓库后端主力语言是 Go，团队接手成本低
- 单二进制发布方便
- 文件扫描、并发和本地 CLI 集成足够稳定
- 后续对 Go 语义提取更容易打深

### 2.2 Application 层

职责：

- 管理一次 scan 的上下文
- 协调双仓路径、manifest、规则包、决策文件
- 调度适配器提取链路
- 调用规则引擎产出差异与结论

这一层不做语言细节，只做流程编排。

### 2.3 Workspace / Baseline 管理器

职责：

- 校验本地仓与官方仓身份
- 记录官方 tag / commit
- 管理本地缓存目录
- 输出可复现的扫描上下文

建议目录：

```text
.forktool/
  cache/
  runs/
  baselines/
  config.yaml
```

MVP 不强依赖数据库。第一版只需要：

- run 级 JSON 快照
- 文件 hash 缓存
- 报告落盘

如后续需要大规模增量扫描，再评估 SQLite。

### 2.4 Manifest 管理器

职责：

- 加载功能清单
- 校验 manifest 语法与引用路径
- 解析 feature -> chain definition -> node patterns
- 将 manifest 与本地台账合并成一次扫描配置

manifest 是整个系统的核心，不应把功能定义硬编码在程序里。

### 2.5 语言适配器

职责：

- 从源码中提取统一 IR 所需的节点
- 不直接做最终 diff，只负责“把源码翻译成统一结构”

首批适配器：

- Go Adapter
- TS/Vue Adapter
- SQL Adapter
- Config Adapter（YAML / JSON / env）
- Doc Adapter（Markdown 台账、决策矩阵）

### 2.6 统一 IR 层

ForkTool 不直接比较 AST，而是先把多语言内容提取成统一语义对象。

统一 IR 至少包含：

- `FeatureChain`
- `RouteNode`
- `CodeSymbol`
- `RequestRule`
- `ResponseRule`
- `HeaderRule`
- `BodyFieldRule`
- `ObservabilityRule`
- `ConfigRef`
- `MigrationRef`
- `TestRef`
- `DecisionRef`

这样做的好处是：

- 语言差异被适配器吸收
- 规则引擎只面对统一对象
- 报告层不需要理解每种 AST

### 2.7 规则引擎

职责：

- 对统一 IR 做功能级比较
- 输出结构化差异
- 结合本地决策文件给出标签与建议

规则引擎不只做“same / diff”，而要做：

- 缺失
- 语义漂移
- 测试缺口
- 配置链缺口
- 观测字段缺口
- 本地保护区命中

### 2.8 报告与导出层

职责：

- 生成 Markdown 报告
- 生成 JSON 报告
- 输出 exit code
- 可选生成 `bd` 草稿或 issue 模板

## 3. 技术选型

## 3.1 核心实现语言

建议：

- 核心 CLI 与编排：Go

原因：

- 当前仓库后端主战场是 Go
- 单二进制交付简单
- 和现有同步脚本、Git CLI、Windows/PowerShell 场景兼容
- 容易与 `gopls`、`go list`、测试命令等集成

## 3.2 多语言解析方案

建议：

- 统一采用 tree-sitter 做源码级抽取
- Go 用 `tree-sitter-go`
- TS 用 `tree-sitter-typescript`
- Vue SFC 用 `tree-sitter-vue`
- SQL 用 `tree-sitter-sql`
- YAML / JSON 用对应 parser 或轻量结构解析

说明：

- 不建议第一版直接依赖语言原生编译器 API 作为唯一方案
- tree-sitter 更适合做统一抽取层
- 对特别关键的 Go 信息，可结合 `go list` / `gopls` 补强

## 3.3 外部工具集成

ForkTool 不应该把现有工具推倒重来，而应做集成层。

### 建议集成但不强依赖的能力

- `git diff --no-index`
  用于保留原始文本 diff 证据
- `rg`
  用于快速候选定位
- `GitNexus`
  用于调用链、影响面、执行流程补强
- `Repomix`
  用于打包指定文件组做长文上下文对照

### 原则

- ForkTool 的主判定不能依赖 MCP 是否在线
- 即使没有 GitNexus / Repomix，也应能完成基础扫描
- 如果这些工具存在，则应作为“增强证据源”

## 4. 仓库结构建议

建议 ForkTool 仓库第一版采用下面结构：

```text
forktool/
  cmd/forktool/
    main.go
  internal/
    app/
    baseline/
    manifest/
    discovery/
    ir/
    rules/
    report/
    cache/
    integrations/
      gitnexus/
      repomix/
      git/
      ripgrep/
    adapters/
      gox/
      tsvue/
      sqlx/
      configx/
      markdownx/
  pkg/
    model/
    cliui/
  examples/
  docs/
```

说明：

- `internal/app` 放 scan orchestration
- `internal/adapters/*` 放语言适配器
- `internal/rules` 放差异判定规则
- `internal/integrations` 放对外工具桥接
- `pkg/model` 可放跨层公共结构

## 5. 运行流程

一次标准扫描的推荐流程如下：

```text
1. 读取本地仓路径 / 官方仓路径 / manifest / 本地决策文件
2. 校验官方基线（tag / commit / remote 身份）
3. 选择目标功能或 release 范围
4. 语言适配器并行抽取本地与官方的链路节点
5. 组装统一 IR
6. 规则引擎执行语义 diff
7. 结合本地台账与决策矩阵打标签
8. 生成 Markdown/JSON 报告
9. 可选输出 bd / PR review 草稿
```

## 6. 为什么不是“全自动合并器”

第一版明确不做自动 merge，原因很简单：

- 当前最贵的问题不是 patch 应用速度，而是“判断失误”
- 自动 merge 之前，先得保证差异分类可信
- 一旦分类错误，自动合并会把错误批量扩散

所以架构优先级必须是：

1. 正确抽取链路
2. 正确分类差异
3. 正确输出决策建议
4. 最后才考虑自动修复

## 7. 和当前同步流程的关系

ForkTool 不是要替换现有同步 skill，而是要成为其中的“审计核心”。

推荐关系：

- baseline 脚本继续负责官方仓对齐
- ForkTool 负责功能链路扫描与差异决策
- 现有同步流程负责最终合并、验证、关单

也就是说：

- baseline = 真相源准备
- ForkTool = 功能链差异判断
- sync workflow = 实施与收口

## 8. 架构边界

### ForkTool 负责

- 功能定义
- 多语言链路抽取
- 语义差异判断
- 风险分类
- 报告输出

### ForkTool 不负责

- 自动修复业务代码
- 替工程师做最终技术裁决
- 承担全部代码搜索任务
- 替代任务系统

## 9. 第一版落地策略

第一版要坚持“小而硬”的策略：

- 先覆盖当前仓库最危险的关键链
- 先把 Markdown/JSON 报告做扎实
- 先把 Go + Vue/TS + SQL + Config 打通
- 先和现有同步流程兼容

不要一开始就做：

- 通用 GUI
- 远程 SaaS
- 任意语言插件市场
- 自动代码改写引擎
