# CLI sweep 567 — blueprint `scaling:` dialects (validate-only)

**Verdict: no bug filed.** Same isolated QA session (`puncsky@gmail.com`), `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T18:54–18:56Z. Validate-only; nothing created or mutated. Companion to sweep 566 (the REST subresource): the blueprint path shares the same `autoscalingSpec` validator through `specFromCreate`, and live validate agrees.

Thirteen manifests (image web service unless noted) through `bex blueprints validate -o json`:

- **Valid:** `{min 1, max 1, targetCPUPercent 70}` on free; the `90` schema boundary; memory-only target (either criterion suffices).
- **Schema layer (vendored Render blueprint spec):** `targetCPUPercent: 95` → "maximum: got 95, want 90"; `minInstances: 0` → "minimum: got 0, want 1"; wrong JSON type; extra key; the REST dialect (`enabled/min/max`) refused as additional properties with all three keys named; `scaling` on a cron refused by its schema branch.
- **Semantic layer (shared with REST):** bounds without any target → "at least one of targetCPUPercent or targetMemoryPercent is required"; `min > max` named (probed on `starter` to isolate from the free cap); `maxInstances: 3` on free → the plan-cap message located at `services[0].plan`, identical wording to the REST refusal in sweep 566; empty `scaling: {}` → "maxInstances must be ≥ 1" (zero-value semantics, since the schema requires no keys).

Surface divergence, inherited rather than bex-invented: the blueprint schema bounds `minInstances ≥ 1` and targets `≤ 90`, while the REST subresource admits `min 0` and targets to `100` (sweep 566). Both bounds come from Render's own two published contracts (vendored blueprint JSON schema vs. its unbounded REST integers with bex's coded 1–100 policy), so the asymmetry mirrors upstream; recorded here so a future reader doesn't re-litigate it.

**Workspace note:** concurrent loops are active in `bex-canary` — during this sweep the service list held 9–10 rows (`qa-20261019-loop3-web`, plus a new Key Value `qa-20261019-loopx-kv4` now occupying the free KV slot). Observed read-only; this loop's own footprint remains zero.

## Unrun

- Blueprint **apply** of a `scaling:` block (validate exercises the same validator; apply would need an owned fixture and proves storage, not new validation).
- `numInstances` interplay with `scaling` in one manifest (manual count vs autoscaler floor).
