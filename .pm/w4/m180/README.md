# w4 · m180 — Save and deploy reuses the serving artifact

**Worker:** worker4 **Goal:** an environment save requesting `deploy` applies the new configuration once using the serving artifact, without a source build. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Select the serving artifact for a deploying environment batch | 60m | — |
| t002 | Audit shared rollout callers, source kinds and missing-artifact policy | 45m | t001 |
| t003 | Render parity | 25m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage | 40m | t003, t004 |
| t006 | Closeout | 10m | t005 |

## Definition of done

Repeat these **observed production journeys** on a fresh owned Free Docker web service from `https://github.com/bex-co/bex`, branch main, root `examples/hello-go`, port 3000 and auto-deploy off. Use synthetic MESSAGE values and remove all owned resources afterwards.

- After the first deploy is Live, edit MESSAGE and choose **Save and deploy**. The wire request still uses `saveMode: "deploy"`; the response reports `rolledOut: true`; exactly one new `config_change` release reaches Live with the new value at its `.onbex.co` URL. Its selected image is the previously serving immutable artifact, its commit remains that artifact's commit, and no BuildKit source build runs. Reload and repeat. Also remove a service-owned MESSAGE over a linked group: the new group's MESSAGE reaches the process without rebuilding the source.
- Choose **Save only** with another MESSAGE. API readback contains the saved value, `rolledOut` is false, the public URL keeps the prior served value, and no new deploy appears. Cancel an unsaved edit and confirm it changes neither saved nor running configuration.
- Choose **Save, rebuild, and deploy**. One new source build/deploy runs, with actual build output, reaches Live, and serves the saved MESSAGE. No intermediate environment-only rollout is opened.
- Choose **Restart service** and confirm it. One release selects the previously serving image/configuration and reaches Live; this existing no-build control remains intact. Image-bearing deploy history stays honest about build lifecycle (coordinate with `w4/blocked/m156`, already fixed on main; do not duplicate its timestamp work).
- The regression audit in [finding.md](finding.md) carries **every clause** of w5/m44's original DoD. Complete its unchecked compatibility work in t002/t005 before closeout; inferred sibling behavior is not claimed as a live failure from this hunt.

## Source + Goal linkage

- **Source:** user-requested infinite `qa-find-bugs` loop using `muse.env`, w4, 2026-10-07 cycle 1. [finding.md](finding.md) contains exact request/complete response, repeat logs, source trace, controls and retained evidence paths.
- **Goal linkage:** ADR008 pillar 1 / Render-compatible hosting, ADR006 shared API contract, ADR013 saved/served secrets, ADR004 immutable serving artifact and generation-scoped release selection, ADR018 parity. Restore the explicit w5/m44 three-choice contract.
- **Expected outcome:** environment-only saves avoid an unnecessary source build and its failure risk while still deploying newly saved values once. The distinct rebuild choice retains its advertised effect.
- **Why now:** two fresh-page `deploy` requests ran real `go build` jobs (~32 s compilation each), while Restart already demonstrated current-artifact reuse. Every environment save currently pays for a build it did not request.
- **Scope:** filing only. Product fixes are pending. t002 owns the blast radius and unprobed families; no broad rewrite of artifact fingerprints or accepted env-group auto-deploy policy.
- **Standing closing tasks:** parity, simplify, coverage and closeout are included because this touches UI plus the REST/GraphQL/MCP environment-save semantics.
