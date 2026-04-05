# 数据模型与接口设计

## 1. 设计原则

ForkTool 的关键不是“直接比较源码”，而是先把不同语言提取成统一语义对象，再做规则判定。

所以这一层的设计原则是：

- 统一 IR 必须稳定、可版本化
- 语言适配器只负责“提取”，不负责最终业务决策
- 规则引擎只依赖统一 IR，不依赖具体语言实现
- 报告输出和 CLI 也只依赖统一 IR

## 2. 核心数据模型

## 2.1 WorkspaceConfig

描述一次扫描运行所需的输入。

```yaml
toolVersion: "0.1.0"
localRepo:
  path: "E:/AAA__ZhuoMian/langlang007/sub2api"
  kind: "fork"
officialRepo:
  path: "E:/AAA__ZhuoMian/langlang007/2api官方参考/sub2api"
  kind: "official"
  tag: "v0.1.105"
manifest:
  path: "./examples/sub2api.manifest.example.yaml"
decisionFile:
  path: "./decisions/sub2api.local-decisions.yaml"
output:
  dir: ".forktool/runs/2026-04-05_v0.1.105"
  formats: ["md", "json"]
```

## 2.2 FeatureManifest

功能清单是整个工具最核心的数据源。

推荐结构：

```yaml
version: 1
repoKind: sub2api
features:
  - id: "claude-count-tokens"
    name: "Claude count_tokens 主链"
    riskLevel: "critical"
    owners: ["backend"]
    languages: ["go", "yaml", "markdown"]
    chain:
      routes:
        - pathPattern: "/v1/messages/count_tokens"
      symbols:
        - file: "backend/internal/service/gateway_service.go"
          functions:
            - "ForwardCountTokens"
            - "buildCountTokensRequest"
            - "buildCountTokensRequestAnthropicAPIKeyPassthrough"
      tests:
        - "backend/internal/service/gateway_anthropic_apikey_passthrough_test.go"
    semanticRules:
      - "claude-count-tokens-url"
      - "claude-header-wire-casing"
      - "claude-count-tokens-beta"
    decisions:
      default: "official-required"
```

## 2.3 FeatureChain

运行时统一对象。

```go
type FeatureChain struct {
    ID              string
    Name            string
    RiskLevel       string
    Languages       []string
    LocalNodes      []ChainNode
    OfficialNodes   []ChainNode
    SemanticRules   []string
    DecisionHints   []DecisionHint
}
```

## 2.4 ChainNode

统一表示链路中的任意节点。

```go
type ChainNode struct {
    Kind        NodeKind
    Language    string
    FilePath    string
    SymbolName  string
    Range       SourceRange
    Metadata    map[string]any
    Relations   []NodeRelation
}
```

`NodeKind` 建议取值：

- `route`
- `handler`
- `service`
- `helper`
- `transformer`
- `test`
- `config`
- `migration`
- `page`
- `nav`
- `api`
- `dto`
- `store`
- `doc`

## 2.5 SemanticDiff

描述一条真正需要看的差异。

```go
type SemanticDiff struct {
    FeatureID      string
    Severity       string
    Category       string
    Title          string
    Description    string
    Evidence       []EvidenceRef
    DecisionTag    string
    RecommendedAction string
}
```

`Category` 建议包括：

- `request-url`
- `request-header`
- `request-body`
- `session-routing`
- `metadata-format`
- `response-header`
- `response-shape`
- `error-semantics`
- `observability`
- `config-chain`
- `migration-chain`
- `test-coverage`

## 2.6 DecisionHint

来自本地二开台账或人工决策文件。

```go
type DecisionHint struct {
    FeatureID      string
    Scope          string
    Decision       string
    Reason         string
    Source         string
}
```

## 2.7 AuditReport

最终报告统一结构。

```go
type AuditReport struct {
    RunID             string
    GeneratedAt       time.Time
    LocalRepo         RepoSnapshot
    OfficialRepo      RepoSnapshot
    ManifestVersion   int
    Summary           AuditSummary
    Features          []FeatureReport
}
```

## 3. 语言适配器接口

第一版不建议暴露真正的外部插件系统，先做内部接口。

## 3.1 Adapter 接口

```go
type Adapter interface {
    Name() string
    SupportsLanguage(lang string) bool
    Discover(ctx context.Context, req DiscoverRequest) ([]ChainNode, error)
}
```

### DiscoverRequest

```go
type DiscoverRequest struct {
    RepoRoot      string
    Feature       ManifestFeature
    RepoSide      string // local | official
    FileGlobs     []string
}
```

## 3.2 RuleEngine 接口

```go
type RuleEngine interface {
    Apply(ctx context.Context, feature FeatureChain, decisions []DecisionHint) ([]SemanticDiff, error)
}
```

## 3.3 Reporter 接口

```go
type Reporter interface {
    Format() string // md | json
    Render(report AuditReport) ([]byte, error)
}
```

## 3.4 Integration Provider 接口

用于可选增强能力。

```go
type SymbolProvider interface {
    Enabled() bool
    QueryRelatedSymbols(ctx context.Context, featureID string) ([]ExternalSymbolRef, error)
}
```

适用对象：

- GitNexus
- Repomix
- ripgrep
- git

## 4. CLI 设计

ForkTool 第一版只做 CLI。

## 4.1 命令结构

```text
forktool
  init
  doctor
  baseline
    verify
  scan
    feature
    release
  report
    render
  manifest
    validate
    list
```

## 4.2 关键命令定义

### `forktool init`

作用：

- 初始化 `.forktool/`
- 生成示例配置
- 复制 manifest 模板

示例：

```bash
forktool init --repo-kind sub2api
```

### `forktool baseline verify`

作用：

- 校验官方仓 remote/tag/commit
- 输出当前基线信息

示例：

```bash
forktool baseline verify \
  --official E:/AAA__ZhuoMian/langlang007/2api官方参考/sub2api \
  --tag v0.1.105
```

### `forktool scan feature`

作用：

- 对单个功能链执行审计

示例：

```bash
forktool scan feature \
  --feature openai-responses-compact \
  --local E:/AAA__ZhuoMian/langlang007/sub2api \
  --official E:/AAA__ZhuoMian/langlang007/2api官方参考/sub2api \
  --tag v0.1.105 \
  --manifest ./manifests/sub2api.yaml \
  --format md
```

### `forktool scan release`

作用：

- 对一个官方版本的多条功能链批量审计

示例：

```bash
forktool scan release \
  --local E:/AAA__ZhuoMian/langlang007/sub2api \
  --official E:/AAA__ZhuoMian/langlang007/2api官方参考/sub2api \
  --tag v0.1.105 \
  --critical-only \
  --out .forktool/runs/v0.1.105
```

## 4.3 CLI 输出原则

- 默认终端输出摘要
- 详细内容落盘到 Markdown / JSON
- 风险级别影响 exit code

建议 exit code：

- `0`：无高风险差异
- `1`：存在 `high/critical`
- `2`：输入错误或 manifest 错误
- `3`：官方基线校验失败

## 5. 报告格式

## 5.1 Markdown 报告结构

```md
# ForkTool Audit Report

## Context

## Summary

## Feature: openai-responses-compact

### Finding 1
- Severity:
- Category:
- Decision:
- Evidence:
- Recommendation:
```

## 5.2 JSON 报告结构

建议保留稳定字段，方便后续：

- 机器消费
- CI 集成
- `bd` 草稿生成

最小字段：

```json
{
  "runId": "2026-04-05_v0.1.105",
  "localRepo": {},
  "officialRepo": {},
  "summary": {},
  "features": [
    {
      "id": "openai-responses-compact",
      "status": "drifted",
      "findings": []
    }
  ]
}
```

## 6. Manifest 设计细节

## 6.1 Manifest 顶层字段

- `version`
- `repoKind`
- `defaults`
- `decisionSources`
- `features`

## 6.2 Feature 字段

- `id`
- `name`
- `description`
- `riskLevel`
- `owners`
- `languages`
- `chain`
- `semanticRules`
- `tests`
- `decisions`
- `notes`

## 6.3 semanticRules 字段

不要写自由文本，而是写规则 ID。

例如：

- `claude-metadata-userid-format`
- `claude-session-hash-normalization`
- `claude-count-tokens-beta-suffix`
- `http-header-wire-casing`
- `openai-compact-path-suffix`
- `openai-session-isolation`
- `openai-passthrough-body-normalization`
- `gemini-upstream-model-preserved`
- `gemini-digest-prefix-ua-normalization`

## 7. 决策文件设计

ForkTool 不能只读 manifest，还要读本地决策。

建议单独维护：

```yaml
version: 1
decisions:
  - featureId: "claude-messages-mainchain"
    scope: "error-redaction"
    decision: "keep-local-hook"
    reason: "客户端错误脱敏与中文提示属于本地业务语义"
  - featureId: "openai-responses-http"
    scope: "session-isolation"
    decision: "official-required"
    reason: "协议主链必须与官方一致"
```

## 8. 统一 IR 的边界

统一 IR 只解决“表示问题”，不解决所有分析问题。

### 应该放进 IR 的内容

- 可稳定抽取的结构化语义
- 可用于规则判断的字段
- 可指向源码的位置信息

### 不应放进 IR 的内容

- 过于语言专属的 AST 细枝末节
- 一次运行中临时日志
- 和功能链判断无关的通用源码噪音

## 9. 示例：如何表示 `openai-responses-compact`

```yaml
id: openai-responses-compact
chain:
  routes:
    - pathPattern: "/v1/responses/compact"
  symbols:
    - file: "backend/internal/service/openai_gateway_service.go"
      functions:
        - "buildUpstreamRequest"
        - "buildUpstreamRequestOpenAIPassthrough"
        - "normalizeOpenAIPassthroughOAuthBody"
  tests:
    - "backend/internal/service/openai_gateway_service_test.go"
semanticRules:
  - "openai-compact-path-suffix"
  - "openai-compact-body-normalization"
  - "openai-session-isolation"
```

## 10. 第一版实现约束

- 插件先做内部接口，不做动态加载
- 报告格式先固化，再考虑用户自定义模板
- manifest 先手写，不做可视化编辑器
- 决策文件先 YAML，不引入数据库管理后台
