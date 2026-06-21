# Fusion Delivery Tracker

This is the working progress board for the Fusion feature. Update this file after every implementation stage so the next worker can see what is finished, what is blocked, and what must be validated next.

## Current Status

| Item | State |
|---|---|
| Product/design scope | Ready |
| Implementation plan | Ready |
| Business code changes | Not started |
| Frontend changes | Not started |
| Security validation | Not started |
| Current next action | Commit the planning docs, then start Stage 1 |

Current source documents:

- `docs/fusion/fusion-design.md`
- `docs/fusion/fusion-implementation-plan.md`
- `docs/fusion/fusion-progress.md`

## First-Version Scope

Fusion v1 builds a saved-config-only multi-model aggregation flow:

- `POST /v1/fusion/chat/completions`
- User-owned encrypted OpenAI-compatible upstream keys.
- User-owned Fusion configs selected by `model`, such as `fusion:research`.
- Parallel candidate calls followed by a Judge synthesis call.
- Admin-controlled platform service billing through `fusion_setting.billing_expr`.
- Fusion disabled by default through `fusion_setting.enabled=false`.
- No direct free proxy using user-supplied API keys or base URLs.

Fusion v1 intentionally does not include:

- Streaming aggregation.
- Responses API.
- Realtime, image, audio, video, or task endpoints.
- Direct single-call bring-your-own-key proxy.
- Tool/function calling passthrough.
- `best_of` or `vote` strategy execution.
- Admin `Channel` fallback when user keys fail.

## Locked MVP Decisions

These decisions are binding for the first implementation pass:

1. Only `strategy=synthesize` is executable. Save validation rejects other strategy values.
2. `stream=true`, `n>1`, `tools`, `tool_choice`, `functions`, and `function_call` are rejected.
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
| 0 | Planning baseline | Ready to commit | Docs exist, no stale open decisions, diff check passes |
| 1 | Secret, settings, and base URL guard | Not started | Explicit crypto-secret guard, Fusion settings, SSRF validation tests pass |
| 2 | Data models and migrations | Not started | Fusion key/config models migrate on SQLite and ownership tests pass |
| 3 | Management API | Not started | `/api/fusion` key/config CRUD and billed test endpoints pass controller tests |
| 4 | Fusion engine and billing | Not started | Parallel candidate, Judge synthesis, Fusion expression billing tests pass |
| 5 | Relay endpoint | Not started | `/v1/fusion/chat/completions` enforces auth, billing, no direct-key bypass, and returns OpenAI-compatible output |
| 6 | Frontend user and admin UI | Not started | User key/config pages and admin settings work in the active frontend theme |
| 7 | Security and compatibility validation | Not started | Existing relay unchanged, Fusion abuse/security checks pass |
| 8 | Release handoff | Not started | Docs updated with final status, validation evidence, and deployment notes |

## Stage 0: Planning Baseline

State: Ready to commit

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

## Stage 1: Secret, Settings, And Base URL Guard

State: Not started

Primary files:

- `common/init.go`
- `common/secret.go`
- `common/secret_test.go`
- `common/fusion_base_url.go`
- `common/fusion_base_url_test.go`
- `setting/fusion_setting/fusion_setting.go`

Completion criteria:

- `HasPersistentCryptoSecret()` returns true only when `CRYPTO_SECRET` is explicitly configured.
- Fusion key storage fails closed when `CryptoSecret` came from `SessionSecret`.
- `fusion_setting` is registered through `config.GlobalConfig.Register("fusion_setting", &FusionSetting{})`.
- Default `fusion_setting.enabled=false`.
- Fusion billing expression validates before save.
- `ValidateFusionBaseURL` requires HTTPS, rejects userinfo, blocks private/internal targets by default, validates redirect targets, and prevents DNS rebinding.
- `common` does not import `setting/fusion_setting`; callers pass a policy struct into `common`.

Validation:

```powershell
go test ./common -run "TestEncryptSecret|TestFingerprintAndMaskSecret|TestValidateFusionBaseURL" -count=1
go test ./setting/... -run Fusion -count=1
```

## Stage 2: Data Models And Migrations

State: Not started

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

## Stage 3: Management API

State: Not started

Primary files:

- `dto/fusion.go`
- `controller/fusion.go`
- `router/fusion-router.go`
- `router/api-router.go`

Completion criteria:

- `/api/fusion/keys` supports list/create/update/delete under `middleware.UserAuth()`.
- `/api/fusion/configs` supports list/create/update/delete under `middleware.UserAuth()`.
- API responses never return plaintext keys, ciphertext, or complete fingerprints.
- Key/config test endpoints require Fusion enabled, persistent `CRYPTO_SECRET`, ownership checks, rate limits, pre-consume billing, and sanitized errors.
- Key test does not accept arbitrary request body passthrough.
- Config test runs through the same billing and execution path as the Fusion relay.

Validation:

```powershell
go test ./controller ./router ./model ./common -run Fusion -count=1
```

## Stage 4: Fusion Engine And Billing

State: Not started

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

## Stage 5: Relay Endpoint

State: Not started

Primary files:

- `controller/fusion.go`
- `router/relay-router.go`

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

## Stage 6: Frontend User And Admin UI

State: Not started

Primary files:

- `web/default/src/features/fusion/*`
- `web/default/src/routes/_authenticated/fusion/index.tsx`
- Active navigation/sidebar files discovered during implementation
- Active admin settings files discovered during implementation
- `web/default/src/i18n/locales/*.json`

Completion criteria:

- User can create, list, edit, disable, delete, and test Fusion upstream keys.
- User can create, list, edit, disable, delete, and test Fusion configs.
- UI never reveals plaintext keys after save.
- Admin can configure Fusion enable switch, billing expression, minimum quota, key-test quota, failed-candidate policy, caps, domain policy, and port policy.
- UI shows a clear warning when `CRYPTO_SECRET` is not explicitly configured.
- Active frontend theme is handled. If classic is active and no classic page exists, Fusion navigation is hidden there.
- All visible text uses i18n.

Validation from `web/default`:

```powershell
bun run typecheck
bun run lint
```

## Stage 7: Security And Compatibility Validation

State: Not started

Primary files:

- Files changed in stages 1 through 6.

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

Validation:

```powershell
go test ./common ./model ./service ./controller ./router -count=1
go test ./... -count=1
```

If `go test ./...` has unrelated existing failures, record exact package names and error messages in this file.

## Stage 8: Release Handoff

State: Not started

Completion criteria:

- This tracker is updated with final stage states and validation evidence.
- `docs/fusion/fusion-design.md` matches implemented behavior.
- `docs/fusion/fusion-implementation-plan.md` checkboxes reflect completed work.
- Deployment notes mention `CRYPTO_SECRET`, `fusion_setting.enabled`, billing expression defaults, and private base URL risk.
- Git status separates Fusion changes from unrelated local files.

Validation:

```powershell
git status --short
git diff --check -- docs\fusion
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

## Final Definition Of Done

Fusion is done when all of these are true:

- A normal user can manage upstream keys and Fusion configs without seeing stored plaintext secrets.
- A normal API token can call `/v1/fusion/chat/completions` only when Fusion is globally enabled and the token may access the Fusion model alias.
- Platform service quota is charged before any upstream call and settled after completion.
- User-owned upstream keys are never usable as a direct free proxy.
- Failed candidate charging follows admin policy.
- SSRF protections prevent user-configured base URLs from reaching private/internal targets by default.
- Existing new-api relay behavior is unchanged.
- Backend tests, frontend checks for the active frontend, and security validation have been run and recorded.
