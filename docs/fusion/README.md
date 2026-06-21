# Fusion Documentation

This directory contains the design and implementation plan for the Fusion multi-model aggregation feature.

Read in this order:

1. `fusion-progress.md` - current status, stage board, locked MVP decisions, and completion criteria.
2. `fusion-design.md` - product, architecture, data, security, billing, and compatibility decisions.
3. `fusion-implementation-plan.md` - staged execution plan with file ownership, tests, and validation commands.
4. `fusion-release-handoff.md` - deployment checklist, release gaps, validation evidence, and rollback notes.

Fusion is intentionally designed as an additive capability. Existing relay routes such as `/v1/chat/completions`, admin `Channel` routing, token authentication, normal billing, and usage logs must continue to work without semantic changes.
