# Fusion Delivery Tracker

This is the working progress board for the Fusion feature. Update this file after every implementation stage so the next worker can see what is finished, what is blocked, and what must be validated next.

## Current Status

| Item | State |
|---|---|
| Product/design scope | Ready |
| Implementation plan | Ready |
| Business code changes | Stage 14 agent upstream streaming resilience complete |
| Frontend changes | Stage 9 template/key test UI complete |
| Security validation | Stage 7 focused tests passed; Codex Security diff scan complete with 0 unresolved findings |
| Current next action | User live retest with Codex-style client, then tune Fusion timeout/model order if the upstream model itself remains slow |

Current source documents:

- `docs/fusion/fusion-design.md`
- `docs/fusion/fusion-implementation-plan.md`
- `docs/fusion/fusion-progress.md`
- `docs/fusion/fusion-release-handoff.md`

## First-Version Scope

Fusion v1 builds a saved-config-only multi-model aggregation flow:

- `POST /v1/chat/completions` and `POST /v1/responses` as primary endpoints when `model` is a `fusion:xxx` alias, with `POST /fusion` and `POST /v1/fusion/chat/completions` kept as chat compatibility aliases.
- User-owned encrypted upstream keys selected through administrator-managed upstream templates.
- User-owned Fusion configs selected by `model`, such as `fusion:research`.
- Parallel candidate calls followed by a Judge synthesis call.
- Admin-controlled platform service billing through `fusion_setting.billing_expr`.
- Fusion disabled by default through `fusion_setting.enabled=false`.
- Key test/probe flow that can suggest per-key upstream JSON config without saving it automatically.
- Agent tools passthrough for Codex/Claude Code style clients using first-success candidate tool-call selection.
- Agent/tool candidate and Judge calls are consumed through internal OpenAI-compatible SSE when the request is stream/tool-like, while normal non-tool requests remain non-streaming upstream calls.
- No direct free proxy using user-supplied API keys or base URLs.

Fusion v1 intentionally does not include:

- True incremental streaming aggregation. `stream=true` is accepted only as a final-result SSE compatibility layer with heartbeat keepalive while aggregation runs.
- Realtime, image, audio, video, or task endpoints.
- Direct single-call bring-your-own-key proxy.
- Legacy `functions` / `function_call`.
- `best_of` or `vote` strategy execution.
- Admin `Channel` fallback when user keys fail.
- Billed live config test execution.

## Locked MVP Decisions

These decisions are binding for the first implementation pass:

1. Only `strategy=synthesize` is executable. Save validation rejects other strategy values.
2. `stream=true` is accepted only as a final-result SSE compatibility layer with heartbeat keepalive while aggregation runs; `tools` / `tool_choice` are passed through to candidates; `n>1`, legacy `functions`, and legacy `function_call` are rejected.
3. Saved key direct relay is rejected. A saved key can only run through an enabled Fusion config after platform pre-consume succeeds.
4. User request JSON cannot include raw `api_key`, `base_url`, `key_id`, candidate key IDs, or Judge key ID.
5. `CRYPTO_SECRET` must be explicitly configured. `SessionSecret` fallback is not accepted for Fusion secret storage.
6. `base_url` normalization uses this rule:
   - `https://host` becomes `https://host/v1/chat/completions`.
   - `https://host/v1` becomes `https://host/v1/chat/completions`.
   - `https://host/openai/v1` becomes `https://host/openai/v1/chat/completions`.
7. Failed candidate service fee is controlled only by `fusion_setting.charge_failed_candidates`, `fusion_setting.failed_candidate_quota`, and `fusion_setting.billing_expr`.

## Stage Board

| Stage | Name | State | Exit Criteria |
|---|---|---|---|
| 0 | Planning baseline | Complete | Docs exist, no stale open decisions, diff check passes |
| 1 | Secret, settings, and base URL guard | Complete | Explicit crypto-secret guard, Fusion settings, SSRF validation tests pass |
| 2 | Data models and migrations | Complete | Fusion key/config models migrate on SQLite and ownership tests pass |
| 3 | Management API | Complete | `/api/fusion` key/config CRUD passes controller tests; config test remains fail-closed |
| 4 | Fusion engine and billing | Complete | Parallel candidate, Judge synthesis, Fusion expression billing tests pass |
| 5 | Relay endpoint | Complete | `/v1/fusion/chat/completions` enforces auth, billing, no direct-key bypass, and returns OpenAI-compatible output |
| 6 | Frontend user and admin UI | Complete | User key/config pages and admin settings work in the active frontend theme |
| 7 | Security and compatibility validation | Complete | Existing relay unchanged, Fusion abuse/security checks pass |
| 8 | Release handoff | Complete | Docs updated with final status, validation evidence, and deployment notes |
| 9 | Upstream templates and standard `/v1` Fusion entry | Complete | `/v1/chat/completions` Fusion alias routing, compatibility routes, DB-backed templates, per-key JSON config, and key probe UI are implemented |
| 10 | Stream and Responses compatibility | Complete | `/v1/chat/completions stream=true` and `/v1/responses` with `model=fusion:xxx` are routed through billed Fusion execution |
| 11 | Agent tools compatibility | Complete | Modern `tools`/`tool_choice` pass through to candidates; first successful candidate `tool_calls` are returned without Judge |
| 12 | Stream heartbeat keepalive | Complete | Stream clients receive early SSE headers and heartbeat comments while candidate/Judge aggregation is still running |
| 13 | Agent tool observation loop | Complete | Responses `function_call` and `function_call_output` history is preserved when Fusion calls candidates |
| 14 | Agent upstream streaming resilience | Complete | Tool/stream candidate and Judge calls use internal SSE parsing with first-response/idle timeout semantics |

## Stage 0: Planning Baseline

State: Complete

Files:

- `docs/fusion/README.md`
- `docs/fusion/fusion-design.md`
- `docs/fusion/fusion-implementation-plan.md`
- `docs/fusion/fusion-progress.md`

Completion criteria:

- Design covers product, billing, security, database, frontend, and compatibility boundaries.
- Implementation plan contains task-by-task file ownership and validation commands.
- Progress tracker records current state and next action.
- No business code is changed in this stage.

Validation:

```powershell
git diff --check -- docs\fusion
rg -n -e ("TO" + "DO") -e ("TB" + "D") -e ("Open " + "Decisions") -e ("token" + "_multiplier") -e ("Fusion" + "FixedRequestQuota") -e ("Fusion" + "CandidateTokenMultiplier") -e ("Fusion" + "JudgeTokenMultiplier") docs\fusion
```

Expected result:

```text
git diff --check exits 0
rg exits 1 with no matches
```

Validation result recorded on 2026-06-21:

```text
git diff --check -- docs\fusion: pass
stale keyword scan: pass
commit: 07eb5173 docs: add fusion implementation baseline
```

## Stage 1: Secret, Settings, And Base URL Guard

State: Complete

Primary files:

- `common/init.go`
- `common/secret.go`
- `common/secret_test.go`
- `common/fusion_base_url.go`
- `common/fusion_base_url_test.go`
- `main.go`
- `setting/fusion_setting/fusion_setting.go`

Completion criteria:

- `HasPersistentCryptoSecret()` returns true only when `CRYPTO_SECRET` is explicitly configured.
- Fusion key storage fails closed when `CryptoSecret` came from `SessionSecret`.
- `fusion_setting` is registered through `config.GlobalConfig.Register("fusion_setting", &FusionSetting{})`.
- `setting/fusion_setting` is imported at startup so the registration runs in production.
- Default `fusion_setting.enabled=false`.
- Fusion billing expression validates before save.
- `ValidateFusionBaseURL` requires HTTPS, rejects userinfo, blocks private/internal targets by default, validates redirect targets, and prevents DNS rebinding.
- `common` does not import `setting/fusion_setting`; callers pass a policy struct into `common`.

Validation:

```powershell
go test ./common -run "TestEncryptSecret|TestFingerprintAndMaskSecret|TestValidateFusionBaseURL" -count=1
go test ./setting/... -run Fusion -count=1
go test . -run '^$' -count=1
```

Validation result recorded on 2026-06-21:

```text
go test ./common -run "TestEncryptSecret|TestFingerprintAndMaskSecret|TestValidateFusionBaseURL" -count=1: pass
go test ./setting/... -run Fusion -count=1: pass
go test . -run '^$' -count=1: pass
git diff --check -- main.go common setting docs\fusion: pass
```

## Stage 2: Data Models And Migrations

State: Complete

Primary files:

- `model/fusion_api_key.go`
- `model/fusion_config.go`
- `model/fusion_test.go`
- `model/main.go`

Completion criteria:

- `fusion_api_keys` stores encrypted user-owned upstream credentials, masked hints, fingerprints, normalized base URLs, and soft deletes.
- `fusion_configs` stores saved aliases such as `fusion:research`, candidate key IDs, model overrides, Judge key, strategy, timeout, and caps.
- JSON-like lists/maps are stored as `TEXT` and use `common.Marshal` / `common.Unmarshal`.
- SQLite, MySQL, and PostgreSQL compatibility is preserved.
- User 1 cannot load or reference user 2's Fusion keys/configs.
- Candidate and Judge model choices respect each saved key's model allowlist.

Validation:

```powershell
go test ./model -run Fusion -count=1
```

Validation result recorded on 2026-06-21:

```text
go test ./model -run Fusion -count=1: pass
```

## Stage 3: Management API

State: Complete

Primary files:

- `dto/fusion.go`
- `controller/fusion.go`
- `router/fusion-router.go`
- `router/api-router.go`

Completion criteria:

- `/api/fusion/keys` supports list/create/update/delete under `middleware.UserAuth()`.
- `/api/fusion/configs` supports list/create/update/delete under `middleware.UserAuth()`.
- API responses never return plaintext keys, ciphertext, or complete fingerprints.
- Key/config test endpoints are mounted under `middleware.UserAuth()` but remain non-user-facing fail-closed stubs in v1.
- Key test does not accept arbitrary request body passthrough.
- Live config testing is excluded from v1; if reintroduced later, it must use the same billing and execution path as the Fusion relay.

Implemented behavior:

- `dto/fusion.go` defines separate request/response DTOs so encrypted key material and full fingerprints are never serialized.
- `controller/fusion.go` validates user ownership, max key/config counts, provider type, key status, model allowlists, model alias format, base URL policy, duplicate key fingerprints, duplicate config aliases, and key-in-use deletion.
- `router/fusion-router.go` registers `/api/fusion/keys` and `/api/fusion/configs` under `middleware.UserAuth()`.
- Key create and key update with a new plaintext key require explicit persistent `CRYPTO_SECRET`.
- Key/config test endpoints require `fusion_setting.enabled=true` and persistent `CRYPTO_SECRET`, then return a clear not-implemented error without decrypting keys or calling upstream. They are not exposed in the v1 frontend.

Validation:

```powershell
go test ./controller ./router ./model ./common -run Fusion -count=1
```

Validation result recorded on 2026-06-21:

```text
go test ./controller ./router ./model ./common -run Fusion -count=1: pass
```

## Stage 4: Fusion Engine And Billing

State: Complete

Primary files:

- `service/fusion.go`
- `service/fusion_billing.go`
- `service/fusion_test.go`

Completion criteria:

- Candidate calls run in bounded parallelism with per-candidate timeout.
- Candidate outputs are treated as untrusted text in the Judge prompt.
- Candidate output size and Judge input token caps are enforced.
- `strategy=synthesize` runs Judge; other strategies are rejected in v1.
- Missing upstream usage falls back to conservative local estimates.
- `RunFusionBillingExpr` exposes Fusion variables and returns new-api quota units directly.
- Existing tiered billing provider-price conversion is not used directly.
- Invalid Fusion expressions fail closed.
- Failed-candidate variables are zeroed unless failed-candidate charging is enabled.

Validation:

```powershell
go test ./service -run Fusion -count=1
```

Implemented behavior:

- `service/fusion.go` runs saved-config-only Fusion execution with bounded parallel candidate calls and a Judge synthesis call.
- Candidate and Judge calls use user-owned encrypted keys, re-check saved `base_url` against current Fusion SSRF policy, and do not use admin `Channel` distribution.
- Redirect following is disabled for Fusion upstream calls; 3xx responses are treated as upstream failures.
- `stream=true` is normalized to non-streaming upstream Fusion execution; modern tools/tool_choice pass through to candidate calls; legacy functions/function_call and `n>1` are still rejected in v1.
- Missing upstream usage falls back to conservative local token estimates.
- Failed candidates keep estimated prompt tokens once an upstream request was attempted, so the failed-candidate billing policy can charge them later if enabled.
- Candidate output is truncated before Judge prompt construction; Judge input is trimmed against the configured token cap.
- `service/fusion_billing.go` evaluates Fusion-specific expressions where the expression result is already new-api quota units, then applies minimum quota and group ratio.
- Fusion billing expressions expose only Fusion variables (`cp`, `cc`, `jp`, `jc`, aliases, `failed`, `failed_prompt`, `failed_quota`, `success`, `total`, `min_quota`) and reject tiered provider-price variables such as `p` and `c`.

Validation result recorded on 2026-06-21:

```text
go test ./service -run Fusion -count=1: pass
go test ./controller ./router ./service ./model ./common -run Fusion -count=1: pass
git diff --check -- service docs\fusion: pass
```

## Stage 5: Relay Endpoint

State: Complete

Primary files:

- `controller/fusion.go`
- `router/relay-router.go`
- `router/fusion_relay_test.go`

Completion criteria:

- `POST /v1/fusion/chat/completions` is mounted without `middleware.Distribute()`.
- Route uses token auth, performance check, route tag, and model request rate limit middleware.
- `fusion_setting.enabled=false` blocks before config lookup, key decryption, billing, and upstream I/O.
- Raw JSON is scanned before DTO parsing so direct-key fields cannot be ignored.
- Token model limits are manually enforced for the Fusion alias.
- `original_model` / `ContextKeyOriginalModel` is set to the requested Fusion alias.
- Platform quota is pre-consumed before upstream calls and settled/refunded after completion.
- Consume logs use `channel_id=0` and `other.fusion=true`.
- Existing `/v1/chat/completions` continues through the normal relay path.

Validation:

```powershell
go test ./controller ./router ./service ./model ./common -run Fusion -count=1
```

Implemented behavior:

- `/v1/fusion/chat/completions` is mounted under `TokenAuth`, `SystemPerformanceCheck`, `RouteTag("fusion")`, and `ModelRequestRateLimit`, without `middleware.Distribute()`.
- Disabled Fusion and missing persistent `CRYPTO_SECRET` fail before raw body parsing, config lookup, key decryption, billing, or upstream calls.
- Raw request JSON rejects direct upstream credential/routing fields before parsing into `dto.GeneralOpenAIRequest`.
- V1 accepts `stream=true` as a final-result SSE compatibility layer, passes modern `tools` / `tool_choice` to candidate calls, and rejects unsupported chat fields: `n>1`, legacy `functions`, and legacy `function_call`.
- Token model limits are manually enforced against the Fusion alias because the route bypasses normal channel distribution.
- The requested Fusion alias is set as `original_model` / `ContextKeyOriginalModel` before relay info and billing setup.
- Platform service quota is pre-consumed before candidate or Judge calls, then settled or refunded through the existing `BillingSession` path.
- Successful responses are OpenAI-compatible chat completions using the Fusion alias as `model`.
- Consume logs use `channel_id=0`, `other.fusion=true`, candidate/Judge metadata, service quota fields, and no candidate text or stored API key material.
- Existing `/v1/chat/completions` remains on the normal relay route and channel distribution path.

Validation result recorded on 2026-06-21:

```text
go test ./controller ./router ./service ./model ./common -run Fusion -count=1: pass
git diff --check -- controller router docs\fusion: pass
go test ./controller ./router ./service ./model ./common -count=1: fails in existing non-Fusion service test TestObserveChannelAffinityUsageCacheByRelayFormat_MixedMode, expected 2 actual 3
go test ./service -run TestObserveChannelAffinityUsageCacheByRelayFormat_MixedMode -count=1: pass
```

## Stage 6: Frontend User And Admin UI

State: Complete

Primary files:

- `web/default/src/features/fusion/*`
- `web/default/src/routes/_authenticated/fusion/index.tsx`
- `web/default/src/hooks/use-sidebar-data.ts`
- `web/default/src/hooks/use-sidebar-config.ts`
- `web/default/src/features/system-settings/models/fusion-settings-card.tsx`
- `web/default/src/features/system-settings/models/index.tsx`
- `web/default/src/features/system-settings/models/section-registry.tsx`
- `web/default/src/features/system-settings/types.ts`
- `web/default/src/features/models/components/drawers/model-mutate-drawer.tsx`
- `web/default/src/features/auth/types.ts`
- `web/default/src/i18n/locales/*.json`

Completion criteria:

- User can create, list, edit, disable, and delete Fusion upstream keys.
- User can create, list, edit, disable, and delete Fusion configs.
- UI never reveals plaintext keys after save.
- Admin can configure Fusion enable switch, billing expression, minimum quota, failed-candidate policy, caps, domain policy, and port policy.
- UI shows a clear warning when `CRYPTO_SECRET` is not explicitly configured.
- Active frontend theme is handled. If classic is active and no classic page exists, Fusion navigation is hidden there.
- All visible text uses i18n.

Validation from `web/default`:

```powershell
bun run typecheck
bun run lint
```

Implementation notes:

- Added the default frontend `/fusion` route with tabs for upstream keys and Fusion configs.
- Added user CRUD UI for Fusion upstream keys and configs. Saved key plaintext is never shown after create/update.
- Removed dashboard test controls from v1. Operators can validate the formal Fusion relay path directly after development.
- Added sidebar navigation for the default frontend only; no classic frontend navigation was added.
- Added admin Fusion settings under Models & Routing, including enablement, service billing expression, failure-charging policy, execution caps, and base URL domain/port policy.
- Added `/api/status` frontend-safe flags for `fusion_enabled` and `fusion_crypto_secret_configured`.
- Added backend option validation so invalid Fusion billing expressions, domain lists, or port lists do not replace the last valid value.
- Added Fusion strings to all default frontend locale files. `bun run i18n:sync` reports `missingCount=0` and `untranslatedCount=0` for en, zh, fr, ja, ru, and vi.

Validation results:

```powershell
gofmt -w controller/misc.go controller/option.go: pass
bun run typecheck: pass
bun run build: pass
bun x oxlint -c .oxlintrc.json src/features/fusion src/routes/_authenticated/fusion src/features/system-settings/models/fusion-settings-card.tsx src/features/system-settings/models/index.tsx src/features/system-settings/models/section-registry.tsx src/features/system-settings/types.ts src/features/models/components/drawers/model-mutate-drawer.tsx src/hooks/use-sidebar-config.ts src/hooks/use-sidebar-data.ts src/features/auth/types.ts: pass with existing import(no-cycle) warning in section-registry.tsx
go test ./controller ./setting/... -run Fusion -count=1: pass
go test ./controller ./router ./service ./model ./common -run Fusion -count=1: pass
bun run lint: fails on existing non-Fusion lint debt, including forgot-password-form.tsx prefer-optional-catch-binding and custom-oauth preset-selector no-useless-spread
```

## Stage 7: Security And Compatibility Validation

State: Complete

Primary files:

- `common/fusion_base_url.go`
- `service/fusion.go`
- `service/fusion_test.go`
- `controller/fusion_test.go`
- `router/fusion_relay_test.go`

Completion criteria:

- Existing relay smoke test still works through normal channel distribution.
- Fusion disabled state blocks before key decrypt and upstream calls.
- Insufficient quota blocks before upstream calls.
- Raw direct-key fields are rejected before DTO parsing.
- Key/config test endpoints cannot be used as free execution bypasses.
- Token model limits are enforced.
- Invalid billing expression disables execution instead of free usage.
- SSRF tests cover private IP, loopback, userinfo, redirect-to-private, disallowed port, and DNS rebinding.
- Security scan or manual threat review covers secret leakage, SSRF, ownership, billing bypass, disabled bypass, direct-key bypass, and existing relay regression.

Implemented behavior:

- Revalidated saved Fusion `base_url` before decrypting the stored upstream API key.
- Wrapped Fusion upstream HTTP clients with a protected transport that disables proxies, disables redirects, validates host and port at connect time, resolves DNS inside `DialContext`, rejects every private/internal resolved IP by default, and dials the validated IP directly.
- Kept fallback validation for non-`*http.Transport` clients.
- Redacted known upstream API keys from sanitized upstream error text.
- Added regression tests for model allowlists, missing `CRYPTO_SECRET` before lookup, nested direct credential rejection, invalid billing expressions before upstream I/O, protected connect-time DNS/IP validation, upstream API-key redaction, and normal `/v1/chat/completions` route registration.
- Key/config test endpoints remain fail-closed with `501 not implemented` and are not exposed in the v1 frontend; they cannot be used as a free execution bypass.
- No live real-provider smoke test was run. Existing relay compatibility was covered by route/middleware source inspection and the new route registration regression test, not by a configured production channel request.

Validation:

```powershell
go test ./controller ./router ./service ./model ./common -run Fusion -count=1
go test ./common ./model ./service ./controller ./router -count=1
go test ./... -count=1
```

If `go test ./...` has unrelated existing failures, record exact package names and error messages in this file.

Validation result recorded on 2026-06-21:

```text
go test ./controller ./router ./service ./model ./common -run Fusion -count=1: pass
Codex Security diff scan 30155b28-6914-4da2-953e-1f77415d4035 over 4d425e6b..28ebc96f: complete, findingCount=0
Security report: C:\Users\imyyy\AppData\Local\Temp\codex-security-scans-DMfXa9\new-api\28ebc96f9a8ce81dd20dea76ae4f06d1af618327_20260621T104938Z_xe3y5oqs\report.md
go test ./common ./model ./service ./controller ./router -count=1: fails in existing non-Fusion package github.com/QuantumNous/new-api/service
  TestObserveChannelAffinityUsageCacheByRelayFormat_MixedMode: expected int(2), actual int64(3)
  TestObserveChannelAffinityUsageCacheByRelayFormat_UnsupportedModeKeepsEmpty: expected int(1), actual int64(4)
go test ./service -run '^TestObserveChannelAffinityUsageCacheByRelayFormat_MixedMode$' -count=1: pass
go test ./service -run '^TestObserveChannelAffinityUsageCacheByRelayFormat_UnsupportedModeKeepsEmpty$' -count=1: pass
go test ./... -count=1: same existing non-Fusion package failure in github.com/QuantumNous/new-api/service; other listed packages passed
```

## Stage 8: Release Handoff

State: Complete

Primary files:

- `docs/fusion/README.md`
- `docs/fusion/fusion-design.md`
- `docs/fusion/fusion-implementation-plan.md`
- `docs/fusion/fusion-progress.md`
- `docs/fusion/fusion-release-handoff.md`

Completion criteria:

- This tracker is updated with final stage states and validation evidence.
- `docs/fusion/fusion-design.md` matches implemented behavior.
- `docs/fusion/fusion-implementation-plan.md` checkboxes reflect completed work.
- Deployment notes mention `CRYPTO_SECRET`, `fusion_setting.enabled`, billing expression defaults, and private base URL risk.
- Git status separates Fusion changes from unrelated local files.

Implemented behavior:

- Added `docs/fusion/fusion-release-handoff.md` with release status, required settings, billing notes, base URL risk, release gaps, validation evidence, rollout checklist, and rollback steps.
- Updated the Fusion docs index to include the release handoff.
- Reconciled implementation status with current behavior: `/v1/fusion/chat/completions` is implemented and billed; key/config live test execution is excluded from v1.
- Recorded that broad backend test failures are existing non-Fusion `service/channel_affinity_usage_cache_test.go` state-isolation issues.

Validation:

```powershell
git status --short
git diff --check -- docs\fusion
```

Validation result recorded on 2026-06-21:

```text
git diff --check -- docs\fusion: pass
git status --short: shows only Fusion docs staged/modified plus pre-existing unrelated local files outside Stage 8 scope
```

## Stage 9: Upstream Templates And Standard `/v1` Fusion Entry

State: Complete

Primary files:

- `model/fusion_upstream_template.go`
- `model/fusion_api_key.go`
- `service/fusion.go`
- `controller/fusion.go`
- `router/relay-router.go`
- `web/default/src/features/fusion/*`
- `web/default/src/features/system-settings/models/fusion-settings-card.tsx`

Completion criteria:

- `POST /v1/chat/completions` routes `model=fusion:xxx` into Fusion while ordinary models still use the normal relay distributor.
- `POST /v1/responses` routes `model=fusion:xxx` into Fusion while ordinary Responses models still use the normal relay distributor.
- `POST /fusion` and `POST /v1/fusion/chat/completions` remain mounted as compatibility aliases.
- `fusion_upstream_templates` stores administrator-managed protocol templates as cross-database GORM models with TEXT JSON fields.
- Fusion keys store `template_id` and per-key `upstream_config` JSON.
- The Fusion engine merges template defaults and per-key config before building upstream requests.
- User key create/edit UI lets users choose an upstream protocol template and edit extra request config JSON.
- Key test/probe can return `detected_config` and fill it into the UI without saving automatically.
- Admin Fusion settings expose upstream template management.
- Ordinary `/v1/chat/completions` models remain unchanged.

Validation:

```powershell
go test ./model ./service ./controller ./router -run Fusion -count=1
cd web/default
bun run typecheck
bun run build
```

## Stage 10: Stream And Responses Compatibility

State: Complete

Primary files:

- `constant/context_key.go`
- `middleware/distributor.go`
- `controller/relay.go`
- `controller/fusion.go`
- `controller/fusion_test.go`
- `service/fusion.go`
- `service/fusion_test.go`
- `docs/fusion/*`

Completion criteria:

- `POST /v1/chat/completions` with `model=fusion:xxx` and `stream=true` no longer fails only because of streaming.
- Fusion still executes candidate and Judge upstream calls internally as non-streaming requests.
- Chat stream clients receive an SSE-compatible final `chat.completion.chunk` followed by `[DONE]`.
- `POST /v1/responses` with `model=fusion:xxx` routes into billed Fusion execution while ordinary Responses requests continue through normal relay distribution.
- Responses requests convert text `input`, string `instructions`, sampling fields, `max_output_tokens`, and `stream` into the internal Fusion chat request.
- Responses stream clients receive final-result `response.output_text.delta` and `response.completed` SSE events.

Validation result recorded on 2026-06-22:

```text
go test ./controller ./middleware ./router -run "Fusion|Distribute" -count=1: pass
go test ./model ./service ./controller ./router ./middleware -run "Fusion|Token|Distribute" -count=1: pass
```

## Stage 11: Agent Tools Compatibility

State: Complete

Primary files:

- `controller/fusion.go`
- `controller/fusion_test.go`
- `service/fusion.go`
- `service/fusion_test.go`
- `docs/fusion/*`

Completion criteria:

- Chat and Responses Fusion requests accept modern `tools` and `tool_choice`.
- Candidate upstream requests receive `tools` and `tool_choice`.
- If a successful candidate returns `tool_calls`, Fusion returns that tool call to the client and does not call Judge.
- Tool-call selection is deterministic `first_success` by configured candidate order.
- Chat stream returns a final tool_call SSE chunk plus `[DONE]`.
- Responses stream returns `response.output_item.done` for `function_call` plus `response.completed`.
- Legacy `functions` and `function_call` remain rejected.

Validation result recorded on 2026-06-22:

```text
go test ./service ./controller -run "Fusion.*Tool|Fusion|Responses" -count=1: pass
go test ./model ./service ./controller ./router ./middleware -run "Fusion|Token|Distribute" -count=1: pass
```

## Stage 12: Stream Heartbeat Keepalive

State: Complete

Primary files:

- `controller/fusion.go`
- `controller/fusion_test.go`
- `docs/fusion/*`

Problem fixed:

- Before this stage, Fusion `stream=true` compatibility only wrote SSE headers and the final chunk after `service.RunFusionEngine` completed.
- Slow candidate and Judge calls could leave Codex/Claude Code style clients with no response bytes for many seconds, causing reconnect loops or idle timeout behavior even though the request eventually succeeded.

Completion criteria:

- Stream requests still run Fusion internally as non-streaming candidate/Judge aggregation.
- Pre-flight validation, token/group/model-limit checks, config lookup, and platform pre-consume still fail with normal JSON errors before SSE headers are committed.
- After pre-consume succeeds, Chat and Responses stream handlers open SSE immediately and send `: PING` comments while aggregation runs.
- Only the request goroutine writes SSE data; the Fusion execution goroutine uses a copied Gin context and returns results through a channel.
- Chat stream still ends with final `chat.completion.chunk` plus `[DONE]`.
- Responses stream still ends with final Responses SSE events.
- Consume logs now record the external `stream=true` flag instead of the internal non-streaming upstream execution flag.

Validation result recorded on 2026-06-22:

```text
go test ./controller -run "Fusion.*Stream|Fusion.*Responses|Fusion" -count=1: pass
```

## Stage 13: Agent Tool Observation Loop

State: Complete

Primary files:

- `controller/fusion.go`
- `controller/fusion_test.go`
- `docs/fusion/*`

Completion criteria:

- `/v1/responses` Fusion input conversion preserves previous `function_call` items as internal chat assistant `tool_calls`.
- `/v1/responses` Fusion input conversion preserves `function_call_output` items as internal chat `role=tool` messages with the matching `call_id`.
- Candidate upstream requests receive the tool observation history before deciding whether to call another tool or produce final text.
- Existing `first_success` tool-call selection and Judge synthesis behavior remain unchanged.

Validation result recorded on 2026-06-22:

```text
go test ./controller -run TestResponsesFusionPreservesFunctionCallOutputForCandidates -count=1: pass
go test ./service ./controller -run "Fusion.*Tool|Fusion|Responses" -count=1: pass
go test ./model ./service ./controller ./router ./middleware -run "Fusion|Token|Distribute" -count=1: pass
```

## Stage 14: Agent Upstream Streaming Resilience

State: Complete

Primary files:

- `service/fusion.go`
- `service/fusion_test.go`
- `docs/fusion/*`

Problem fixed:

- Codex/Claude Code style clients can send long-running agent turns to Fusion.
- Before this stage, Fusion forced candidate upstream calls to `stream=false` and waited for one complete JSON body. Judge calls also stayed non-streaming. Slow reasoning/tool models could return HTTP 200 and then stay silent until Fusion's timeout expired, producing `fusion minimum successes not met` or Judge timeout errors.

Completion criteria:

- If a Fusion request is stream/tool-like, candidate and Judge upstream calls are sent with `stream=true`.
- Fusion consumes OpenAI-compatible upstream SSE internally and reconstructs text, tool calls, finish reason, and usage.
- In the internal SSE path, `timeout_ms` is treated as first-response / idle timeout. An upstream stream can exceed that wall-clock duration if it keeps sending events before the idle timer expires.
- Streaming tool-call deltas are assembled into final `tool_calls` before the existing `first_success` tool-call selection.
- Ordinary non-tool, non-stream Fusion requests continue to use non-streaming upstream calls.

Validation result recorded on 2026-06-22:

```text
go test ./service -run "FusionEngineAllowsActiveStreamBeyondConfiguredIdleTimeout|FusionEngineStreamsAgentCandidateToAvoidBodyTimeout|FusionEngineStreamsJudgeForExternalStreamRequest|FusionEngineParsesStreamingToolCallCandidate" -count=1: pass
go test ./service ./controller -run "Fusion.*Tool|Fusion|Responses|FusionEngineStreams|FusionEngineParses" -count=1: pass
go test ./model ./service ./controller ./router ./middleware -run "Fusion|Token|Distribute" -count=1: pass
```

## Update Rules

When working on Fusion:

1. Update only the active stage state before starting work.
2. After each stage, add exact validation commands and results under that stage.
3. If implementation differs from design, update `fusion-design.md` and this tracker in the same stage.
4. Do not mark a stage complete without passing its focused validation or recording the exact blocking failure.
5. Do not advance to frontend work until backend relay and billing behavior are validated.

Allowed state values:

- Not started
- In progress
- Blocked
- Complete
- Deferred

## Final Implementation Handoff Definition Of Done

Fusion implementation handoff is done when all of these are true:

- A normal user can manage upstream keys and Fusion configs without seeing stored plaintext secrets.
- A normal API token can call `/v1/chat/completions` or `/v1/responses` with `model=fusion:xxx`, `/fusion`, or `/v1/fusion/chat/completions` only when Fusion is globally enabled and the token may access the Fusion model alias.
- Platform service quota is charged before any upstream call and settled after completion.
- User-owned upstream keys are never usable as a direct free proxy.
- Failed candidate charging follows admin policy.
- SSRF protections prevent user-configured base URLs from reaching private/internal targets by default.
- Existing new-api relay behavior is unchanged.
- Backend tests, frontend checks for the active frontend, and security validation have been run and recorded.

Release gaps that remain explicit:

- Config live test execution is not part of v1; the backend config test stub still fails closed with `501`.
- Key test/probe execution is implemented as a configuration helper and does not enter Fusion billing.
- No live real-provider smoke test has been run for `/v1/chat/completions` or `/v1/responses` with `model=fusion:xxx`.
- Broad backend test sweeps still expose existing non-Fusion channel affinity usage cache test isolation failures.
- Full frontend lint still exposes existing non-Fusion lint debt outside the Fusion change set.
