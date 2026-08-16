# w8 · m19 — Blueprint spec-parity round 2: registry sweep, custom paths, schema drift

**Worker:** worker8 **Goal:** the ADR049 capability registry stops rejecting the three highest-frequency real-world render.yaml fields still fail-closed on the m63 audit baseline (static `buildCommand`, `dockerContext`, `registryCredential`/`image.creds`), accepts Render's Feb-2026 custom Blueprint filenames/paths, runs against a freshly re-pinned upstream schema with a verified drift cadence, and the ADR018 Blueprint row's ◐ cells are refreshed with official-Render-CLI evidence. **Status:** done

## Tasks (in order)

| id   | title                                                                                  | est | depends_on             |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Re-pin the upstream render.yaml schema + verify/establish the drift-report cadence — **DONE**      | 30m | —                      |
| t002 | Disposition sweep: replace the m63 "audit baseline" placeholder on all unsupported entries — **DONE** | 45m | t001                   |
| t003 | Promote static-site `buildCommand` to a translated handler — **DONE**                              | 45m | t002                   |
| t004 | Promote `dockerContext` (server + cron) to a translated handler — **DONE**                         | 45m | t002                   |
| t005 | Promote `registryCredential` / `image.creds` (private prebuilt images) to translated — **DONE**    | 60m | t002                   |
| t006 | Custom Blueprint filenames + subdirectory paths (relax `approvedBlueprintPath`) — **DONE**         | 45m | —                      |
| t007 | Official Render CLI blueprint evidence + refresh the ADR018 Blueprint row — **DONE**               | 45m | t003, t004, t005, t006 |
| t008 | Render parity check (REST/GraphQL/MCP/UI consistency for every promoted field) — **DONE**          | 30m | t007                   |
| t009 | Simplify (`/simplify` over the changed code) — **DONE**                                            | 30m | t008                   |
| t010 | Test coverage (registry dispositions, promoted handlers, path relaxation) — **DONE**               | 45m | t008                   |
| t011 | Closeout — **DONE**                                                                                | 15m | t010                   |

## Definition of done

A real-world render.yaml that uses a static site with `buildCommand`, a Docker service with `dockerContext`, and a prebuilt private image with `image.creds`/`registryCredential` — stored under a non-`render.yaml` filename in a subdirectory — validates, previews, and deploys through every entrypoint (REST validate/create, GraphQL, MCP, dashboard `blueprints/new` path field) instead of failing closed. `capabilities.json` carries zero entries whose reason is still the m63 placeholder ("audit baseline: fail closed until a reviewed handler is implemented") — every remaining `unsupported` entry states its true disposition (anti-goal citation or concrete semantic mismatch). The pinned schema digest matches the current upstream `https://render.com/schema/render.yaml.json`, any newly appeared fields are classified (fail-closed by default), and a scheduled drift check demonstrably runs. `docs/ADR018-render-parity.md` line ~140's Blueprint row is updated with captured `render-oss/cli` blueprint-validate evidence in `docs/cli-compatibility-checklist.md`, moving cells ◐→✅ only where the evidence supports it.

## Source + Goal linkage

- **Source:** blueprint Render-parity review session 2026-08-15 (ADR049 registry vs render.com/docs/blueprint-spec + infrastructure-as-code + api-docs.render.com + changelog): 50 registry entries are `unsupported` but nearly all carry the conservative m63 placeholder reason, and three of them (static `buildCommand` — set by almost every real Render static-site yaml; `dockerContext` — common in monorepos; `registryCredential`/`image.creds` — private prebuilt images) block otherwise-portable files despite bex already owning the needed mechanisms (BuildKit build Jobs, ADR022 w7/m36 per-App pull-credential machinery). Render shipped custom Blueprint filenames/paths 2026-02-09 while `approvedBlueprintPath` (`lego/backend/internal/apps/blueprint.go`) still requires basename `render.yaml`/`bex.yml`. The schema pin dates from 2026-08-02; Render now also publishes the schema on SchemaStore.
- **Goal linkage:** Render parity (docs/ADR018-render-parity.md Blueprint row, still ◐ across all four surfaces) + ADR049's own D7 rule that fail-closed entries be honest dispositions, not an unswept baseline.
- **Expected outcome:** the most common real-world render.yaml files import into bex unmodified; the unsupported set shrinks to genuine anti-goals and true semantic mismatches; parity-ledger cells reflect captured evidence.
- **Why now:** w1/m63 + w8/m18 are fresh — the compiler, registry, and preview payload this builds on just landed and the team context is warm; the schema pin ages daily; ADR018's ◐ row explicitly names "outstanding full cross-surface CLI evidence" as the blocker this milestone removes. Render parity task included: this is feature work changing validation behavior across REST/GraphQL/MCP and the dashboard path field.
- **DO_NOT_DO constraints honored:** previews (`previews.*`, `previewValue` — rejected 2026-07-12), persistent service disks, and per-resource `region` stay fail-closed `unsupported`; the sweep re-labels them as permanent anti-goals with citations, never promotes them. `autoDeployTrigger: checksPass` stays unsupported per ADR049 D7 (never collapsed to `commit`). No CLI is built or forked — t007 runs Render's own official CLI per the standing decision.
