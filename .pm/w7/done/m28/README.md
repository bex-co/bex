# w7 · m28 — Build logs: ship `type=build` into the log store

**Worker:** worker7 **Goal:** Build Job output ships into the Loki pipeline attributed to the owning App, so a failed or in-flight build is debuggable through the existing logs API and `type=build` stops being "empty by design". **Status:** done

## Tasks (in order)

| id   | title                                                          | est | depends_on |
| ---- | -------------------------------------------------------------- | --- | ---------- |
| t001 | Label build Jobs/pods for log attribution                      | 40m | —          | — **DONE**
| t002 | Ship build-namespace logs through the shipper as `type=build`  | 40m | t001       | — **DONE**
| t003 | Backend: serve `type=build` via `QueryLogs` (store-less ⇒ 503) | 30m | t002       | — **DONE**
| t004 | Tenant scoping: build streams + label-discovery leak check     | 30m | t003       | — **DONE**
| t005 | Docs: close the ADR010/ADR018 build-logs divergence            | 20m | t004       | — **DONE**
| t006 | Render parity                                                  | 30m | t005       | — **DONE**
| t007 | Simplify                                                       | 30m | t006       | — **DONE**
| t008 | Test coverage                                                  | 45m | t006       | — **DONE**
| t009 | Closeout                                                       | 15m | t008       | — **DONE**

## Definition of done

A git-sourced deploy's build output is queryable via `GET /v1/logs?type=build`, GraphQL `logs`, and MCP `list_logs`, during and after the build. A caller from another workspace cannot read those lines, and `list_log_label_values` never surfaces another tenant's build labels. With `BEX_LOKI_URL` unset, a `type=build` query returns 503 — never a silently empty result.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker to work on until feature parity` 2026-07-14 (item 1); `docs/ADR018-render-parity.md` §Logs — "`type=build` stays empty by design (bex builds run in a separate plane — no shipper)".
- **Goal linkage:** GOAL #2 (basic obs) + vision pillar 4 — an agent debugging a failed deploy needs the build log; Render's deploy experience is substantially watching the build log stream.
- **Expected outcome:** `type=build` returns real build lines on all three API surfaces; the ADR018 divergence is closed; w5/m29's build-log pane has a backend.
- **Why now:** w7's queue is empty and this unblocks w5/m29. Placed under w7 (topical owner w3 has 4 open milestones) per the w7/m27 capacity-placement precedent. Render parity task included — REST/GraphQL/MCP surface change.
