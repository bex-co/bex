# m49 — Blueprint `scaling:` config

**Status:** DONE 2026-07-16

## Problem

The REST/GraphQL/MCP autoscaling API (`SetAutoscaling`/`get_autoscaling`/etc,
w1/m20) was complete. But the Blueprint parser (`bexService` struct in
`deploy.go` + `parseService()`) had no `scaling:` field. A `bex.yml` that
specified `scaling: {minInstances, maxInstances, targetCPUPercent}` silently
produced a service with autoscaling disabled — the YAML was parsed and then
discarded with no error or warning.

## What shipped

- **`bexScaling`** struct added to `deploy.go` with `minInstances`/`maxInstances`/
  `targetCPUPercent`/`targetMemoryPercent`.
- **`bexService.Scaling *bexScaling`** field added (between `InitialDeployHook`
  and `AutoDeploy`).
- **`scalingToAutoscalingRequest()`** helper converts a `*bexScaling` to a
  `*SetAutoscalingRequest`; `parseService()` now calls it and places the result
  in the returned request.
- **`CreateRequest`** (`service.go`) gains `Autoscaling *SetAutoscalingRequest`.
- **`autoscalingSpec()`** shared helper extracted from `SetAutoscaling`, removing
  ~12 lines of duplicated validation (range checks on min/max/CPU%/mem% + the
  "at least one target" rule).
- **`SetAutoscaling`** refactored to call `autoscalingSpec()` — behavior
  unchanged.
- **`specFromCreate()`** applies `req.Autoscaling` via `autoscalingSpec()`:
  a non-nil field enables autoscaling immediately, with the same validation as
  `SetAutoscaling`; nil leaves `spec.Autoscaling` unset (backward-compatible).
- **Tests** (`stack_test.go`):
  - `TestParseStackScalingBlockPopulatesAutoscaling` — verifies the YAML
    `scaling:` block flows through `parseStack` → `CreateRequest.Autoscaling`
    and that `specFromCreate` materializes `spec.Autoscaling` with
    `Enabled: true`.
  - `TestParseStackScalingBlockValidation` — verifies that a `scaling:` block
    with no target set (neither `targetCPUPercent` nor `targetMemoryPercent`)
    is rejected by `specFromCreate`, not silently accepted.
- **ADR018** gap-backlog row added for w2/m49.

## Commit

`feat(blueprint): wire scaling: block into CreateRequest.Autoscaling (w2/m49)`
