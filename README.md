# ForkTool

ForkTool 是一套面向“官方开源仓库同步到本地二开仓库”场景的功能链路级同步审计器设计文档包。

它的目标不是替代 `git diff`，也不是做一个泛化的任意仓库对比器，而是解决下面这类真实问题：

- 同一功能分散在路由、handler、service、helper、测试、配置、迁移多个文件里，单文件 diff 不能说明“这条链路是否真的同步到位”。
- 本地二开仓库允许保留部分业务 hook，但核心协议链路又必须与官方一致，单纯的文本比较无法自动区分“应该保留”和“必须回官方”的差异。
- 前后端、配置、迁移跨语言协作时，文件还在不代表功能还活着；真正关键的是页面入口、接口 DTO、请求头、请求体、字段语义、观测字段、测试覆盖是否成链闭环。

当前规划优先覆盖 Sub2API 这类技术栈：

- 后端：Go
- 前端：Vue 3 + TypeScript
- 数据与迁移：SQL
- 配置与文档：YAML / JSON / Markdown

## 文档导航

- `docs/01-product-background.md`
  说明为什么要做 ForkTool、当前同步痛点、目标用户、成功标准。
- `docs/02-system-architecture.md`
  说明系统架构、模块边界、运行流程、技术选型、仓库结构。
- `docs/03-feature-design.md`
  说明核心功能、差异规则、多语言适配策略、链路级语义对比方式。
- `docs/04-data-model-and-interfaces.md`
  说明 manifest、统一 IR、插件接口、CLI、报告格式、集成边界。
- `docs/05-delivery-plan.md`
  说明从规划到实现的分阶段路线、里程碑、开发顺序、测试策略。
- `docs/06-backlog-and-acceptance.md`
  说明可直接落地的任务拆解、优先级、验收标准、风险与开放问题。
- `examples/sub2api.manifest.example.yaml`
  提供面向当前仓库的示例功能 manifest。
- `examples/report.example.md`
  提供预期输出的 Markdown 报告样例。

## 一句话定位

ForkTool = `manifest 驱动的功能链路抽取 + 语义规则对比 + 同步决策输出`

它要回答的不是：

- “这两个文件有哪些行不一样？”

而是：

- “Claude `count_tokens` 这条链路从路由到上游 header 透传到底哪里还没和官方对齐？”
- “OpenAI `/responses/compact` 路径本地是否已经同步了官方 compact 语义？”
- “Gemini native / compat 两条链路里，哪些差异是允许的本地 hook，哪些是必须官方化的协议偏离？”

## 推荐阅读顺序

1. 先读 `docs/01-product-background.md`
2. 再读 `docs/02-system-architecture.md`
3. 然后读 `docs/03-feature-design.md`
4. 开始准备实现前，重点读 `docs/04-data-model-and-interfaces.md`
5. 开工排期和 issue 拆分时，读 `docs/05-delivery-plan.md` 与 `docs/06-backlog-and-acceptance.md`

## 设计原则

- 以当前仓库场景优先，不预支“全语言万能平台”的复杂度。
- 先做 CLI 与 Markdown/JSON 报告，不先做 GUI。
- 先解决“找出哪里没同步对”，再考虑自动合并。
- 核心判定必须尽量确定性，不把 AI 作为唯一判定器。
- 支持多语言，但采用“语言无关核心 + 语言适配器”架构。
- 对官方同步最关键的链路，必须能落到“功能级别”的风险与决策，而不是文件级别。

## 当前建议的首批纳管功能

- `claude-messages-mainchain`
- `claude-count-tokens`
- `openai-responses-http`
- `openai-responses-compact`
- `openai-responses-ws`
- `gemini-native-v1beta`
- `gemini-messages-compat`

这些功能已经足以覆盖当前仓库最容易出同步事故的主链。
