# ForkTool Audit Report

## Context

- Local Repo: `E:/AAA__ZhuoMian/langlang007/sub2api`
- Official Repo: `E:/AAA__ZhuoMian/langlang007/2api官方参考/sub2api`
- Official Tag: `v0.1.105`
- Official Commit: `9398ea7af575a59065d4dd967f8277d235726563`
- Manifest: `manifests/sub2api.yaml`

## Summary

- Features Scanned: `6`
- Critical Findings: `3`
- High Findings: `4`
- Medium Findings: `2`
- Recommended Action: `Claude / OpenAI 主链整链路官方化，Gemini 修补 session 与 observability 偏差`

## Feature: claude-count-tokens

### Finding 1

- Severity: `critical`
- Category: `request-url`
- Decision: `official-required`
- Title: `count_tokens 上游 URL 丢失 beta=true`
- Evidence:
  - official: `backend/internal/service/gateway_service.go`
  - local: `backend/internal/service/gateway_service.go`
- Recommendation:
  - 回收为官方 `?beta=true` 组装逻辑

### Finding 2

- Severity: `high`
- Category: `request-header`
- Decision: `official-required`
- Title: `count_tokens 透传头未保持官方 wire casing/raw write`
- Recommendation:
  - 回收为 `resolveWireCasing + addHeaderRaw`

## Feature: openai-responses-compact

### Finding 1

- Severity: `critical`
- Category: `request-url`
- Decision: `official-required`
- Title: `compact 请求未透传 /compact suffix`
- Recommendation:
  - 回收为官方 compact suffix 逻辑

### Finding 2

- Severity: `critical`
- Category: `request-body`
- Decision: `official-required`
- Title: `compact body normalize 与官方不一致`
- Recommendation:
  - compact 模式删除 `store/stream`，仅保留允许字段

### Finding 3

- Severity: `high`
- Category: `test-coverage`
- Decision: `test-missing`
- Title: `官方 compact/session isolation 测试缺失`
- Recommendation:
  - 引入 `openai_gateway_service_session_isolation_test.go`

## Feature: gemini-native-v1beta

### Finding 1

- Severity: `high`
- Category: `session-routing`
- Decision: `official-required`
- Title: `GenerateGeminiPrefixHash 未使用官方 UA normalize`
- Recommendation:
  - 回收为 `NormalizeSessionUserAgent` 后再生成 digest

## Feature: gemini-messages-compat

### Finding 1

- Severity: `high`
- Category: `observability`
- Decision: `official-required`
- Title: `ForwardResult.UpstreamModel 被本地删除`
- Recommendation:
  - 恢复官方 `mappedModel -> UpstreamModel` 观测链

## Action Plan

1. 先整链路回收 Claude `metadata/session/count_tokens`。
2. 再整链路回收 OpenAI compact + session isolation。
3. 最后补 Gemini `UpstreamModel` 与 prefix hash。
4. 同步补齐官方测试缺口。
