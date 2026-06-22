# Fusion Release Handoff

This handoff records the deployable Fusion MVP boundary after Stage 8. It is the operational checklist for enabling Fusion without changing the existing relay path.

## Release Status

Fusion is implementation-complete for the saved-config relay path:

- Users can store encrypted OpenAI-compatible upstream keys and saved Fusion configs.
- API callers use the normal OpenAI-compatible `POST /v1/chat/completions` or `POST /v1/responses` endpoint with a saved Fusion model alias such as `fusion:research`. `POST /fusion` and `POST /v1/fusion/chat/completions` remain available as chat compatibility aliases.
- Candidate calls run in parallel, then the configured Judge model synthesizes the final response.
- Platform service quota is pre-consumed before upstream calls and settled or refunded through the existing billing session path.
- User-owned upstream keys are never routed through administrator `Channel` records.
- Normal relay routes such as `POST /v1/chat/completions` and `POST /v1/responses` remain mounted under the existing distributor path; only `model=fusion:xxx` is diverted to Fusion before admin `Channel` selection.
- Administrators manage Fusion upstream templates in the database. Users select one template per upstream key and can save extra JSON config for gateway-specific request parameters.

Fusion is not enabled automatically. It remains disabled until an administrator configures the required settings and flips `fusion_setting.enabled=true`.

## Required Deployment Settings

Set these before enabling Fusion in any persistent environment:

```text
CRYPTO_SECRET=<stable long random secret>
fusion_setting.enabled=false
fusion_setting.billing_mode=expr
fusion_setting.minimum_quota=1
fusion_setting.billing_expr=max(min_quota, (cp + cc) * 0.20 + (jp + jc) * 0.50 + failed * failed_quota)
fusion_setting.charge_failed_candidates=false
fusion_setting.failed_candidate_quota=0
fusion_setting.stream_candidate_brief=true
fusion_setting.stream_candidate_max_tokens=1024
fusion_setting.response_state_ttl_seconds=86400
fusion_setting.response_state_max_payload_bytes=2097152
fusion_setting.allow_private_base_url=false
fusion_setting.allowed_base_url_ports=443
```

`CRYPTO_SECRET` must be stable across restarts. If it changes, existing Fusion API keys cannot be decrypted and users must re-enter those upstream keys.

Keep `fusion_setting.enabled=false` while configuring limits, billing expression, frontend access, and smoke tests. Turn it on only after the deployment has a deliberate rollout plan.

## Billing Notes

Fusion billing is a platform service fee. The user's upstream provider still bills the user directly through their own API key.

The expression in `fusion_setting.billing_expr` returns new-api quota units directly. It is not the normal provider-price-per-token expression path.

Supported Fusion billing variables include:

```text
cp, cc, jp, jc, candidate_prompt, candidate_completion, judge_prompt, judge_completion,
failed, failed_prompt, failed_quota, success, total, min_quota
```

Unsupported tiered-provider variables such as `p`, `c`, `cr`, and `img` must fail closed. If an invalid expression is saved or loaded, Fusion execution must not become free usage.

Failed candidate charging is controlled by:

```text
fusion_setting.charge_failed_candidates
fusion_setting.failed_candidate_quota
fusion_setting.billing_expr
```

When failed-candidate charging is disabled, failed candidates do not increase `failed` or `failed_prompt` in the expression environment.

## Base URL Risk

Users may configure arbitrary public OpenAI-compatible `base_url` values, so this setting is security-sensitive.

Default policy:

- Require HTTPS.
- Reject URL userinfo.
- Reject loopback, link-local, private, and internal resolved IPs.
- Reject non-allowed ports.
- Disable redirects for Fusion upstream calls.
- Disable HTTP proxy use for Fusion upstream calls.
- Revalidate DNS/IP at connection time to prevent DNS rebinding after save-time validation.

Keep `fusion_setting.allow_private_base_url=false` for normal hosted deployments. Turning it on is a high-risk administrator decision because it allows user-controlled upstream configs to reach private network targets.

Use `fusion_setting.allowed_base_url_domains` only when the operator wants to restrict users to a known public provider set.

## User-Facing Behavior

Supported in the MVP:

- Saved user-owned upstream keys.
- Administrator-managed upstream protocol templates.
- Saved Fusion configs selected by model alias, such as `fusion:research`.
- `strategy=synthesize`.
- Non-streaming OpenAI-compatible chat completions.
- OpenAI-compatible `stream=true` SSE for chat completions, with heartbeat comments during candidate aggregation and incremental final Judge text deltas once synthesis starts.
- OpenAI Responses text compatibility through `POST /v1/responses`, including heartbeat comments during candidate aggregation and incremental `response.output_text.delta` events for final Judge text when `stream=true`.
- Stream latency controls for pure text turns: candidates can return short briefs, candidate generation is capped, and Judge can start once `min_successes` is reached.
- Modern agent `tools` / `tool_choice` passthrough with first-success candidate `tool_calls` returned to the client.
- Responses agent tool-loop history preservation: `previous_response_id` restores encrypted Fusion response state, then current `function_call_output` items are converted to internal chat `role=tool` messages with matching canonical `call_id` before candidate calls.
- Chat Completions agent clients must send complete tool history themselves; Fusion rejects orphan `role=tool` messages before upstream calls.
- Internal upstream SSE for agent-like calls: stream/tool candidate and Judge requests use `stream=true` upstream where applicable, and Fusion reconstructs text/tool calls before applying the existing selection/Judge flow. In this path `timeout_ms` is the first-response / idle timeout; active streams may run longer if they keep producing events.
- Admin-configured service billing expression.
- Failed-candidate billing policy.
- Key test/probe that can suggest `upstream_config` JSON without saving it automatically.

Rejected or not implemented in the MVP:

- Candidate-token multiplexing. Candidate outputs are collected before Judge synthesis, so final answer text cannot be streamed during the candidate phase.
- Judge-based tool-call arbitration.
- Fusion-side tool execution. Agent clients remain responsible for executing returned tool calls and sending the tool result back on the next turn.
- Legacy `functions` / `function_call`.
- Complex non-text Responses modalities.
- Realtime, image, audio, video, or task endpoints.
- Direct single-call bring-your-own-key proxy.
- Per-request raw `api_key`, `base_url`, `key_id`, candidate key IDs, or Judge key ID.
- `best_of` or `vote` strategy execution.
- Admin `Channel` fallback when user keys fail.
- Billed live config test execution.

## Release Gaps

These are intentionally not part of the v1 release:

- `POST /api/fusion/configs/:id/test` is still a fail-closed stub. It returns `501` and does not decrypt keys, call upstream, or bill.
- Key test/probe is a configuration helper, not a public free proxy. It can call the configured upstream to detect template suggestions, but it does not save detected config automatically and does not enter Fusion billing.
- No live real-provider smoke test was run during Stage 7/8/9/10. Validation used fake upstream servers, focused tests, route registration checks, and source inspection.
- The classic frontend does not expose Fusion navigation.

The relay endpoint itself is separate from these test endpoints and is implemented through the billed Fusion execution path.

## Validation Evidence

Last recorded Fusion-focused validation:

```powershell
go test ./model ./service ./controller ./router ./middleware ./setting/fusion_setting -run "Fusion|Token|Distribute|FusionSetting|ValidateFusionChatRequest" -count=1
cd web/default
bun run typecheck
bun run build
git diff --check
```

Result:

```text
pass
```

Stage 7 security scan:

```text
Codex Security diff scan 30155b28-6914-4da2-953e-1f77415d4035 over 4d425e6b..28ebc96f: complete, findingCount=0
Report: C:\Users\imyyy\AppData\Local\Temp\codex-security-scans-DMfXa9\new-api\28ebc96f9a8ce81dd20dea76ae4f06d1af618327_20260621T104938Z_xe3y5oqs\report.md
```

Known non-Fusion validation failures:

```text
go test ./common ./model ./service ./controller ./router -count=1
go test ./... -count=1
```

Both broad commands can fail in `github.com/QuantumNous/new-api/service` because `service/channel_affinity_usage_cache_test.go` has package-level state isolation issues:

```text
TestObserveChannelAffinityUsageCacheByRelayFormat_MixedMode: expected int(2), actual int64(3)
TestObserveChannelAffinityUsageCacheByRelayFormat_UnsupportedModeKeepsEmpty: expected int(1), actual int64(4)
```

Each of those two tests passed when run individually during Stage 7.

Known frontend validation caveat:

```text
bun run lint
```

This can fail on existing non-Fusion lint debt outside the Fusion change set. Stage 6 targeted frontend checks for the Fusion files passed, and `bun run typecheck` / `bun run build` passed.

## Rollout Checklist

1. Deploy the database migration with the Fusion models available through GORM `AutoMigrate`.
2. Set a stable `CRYPTO_SECRET`.
3. Keep `fusion_setting.enabled=false`.
4. Configure candidate count, timeout, Judge input, candidate output, domain, and port limits.
5. Configure and validate `fusion_setting.billing_expr`.
6. Confirm token model access rules include the intended Fusion aliases.
7. Confirm the default `OpenAI Compatible` Fusion upstream template exists or create the intended upstream templates in the admin UI.
8. Run a controlled fake-upstream or staging upstream smoke for `/v1/chat/completions` and `/v1/responses` with `model=fusion:xxx`, including `stream=true` clients.
9. Set each Fusion config `MaxParallel` according to expected latency and upstream rate limits. `MaxParallel=1` runs candidate calls serially and can make agent clients feel slow.
10. For code or HTML generation workloads, keep `fusion_setting.stream_candidate_brief=true` and reduce `fusion_setting.stream_candidate_max_tokens` if the first visible final token is still too slow.
11. Keep `fusion_setting.response_state_ttl_seconds` long enough for expected agent tool loops, and keep `fusion_setting.response_state_max_payload_bytes` bounded for database safety.
12. Enable `fusion_setting.enabled=true` for a limited user group or controlled rollout.
13. Monitor consume logs with `other.fusion=true` and `channel_id=0`.

## Rollback

Set:

```text
fusion_setting.enabled=false
```

This blocks Fusion relay execution before config lookup, key decryption, billing, or upstream I/O. It does not affect stored user configs, stored encrypted keys, administrator `Channel` routing, or existing relay endpoints.
