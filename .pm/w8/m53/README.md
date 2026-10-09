# w8 · m53 — Accept numeric Blueprint environment values

**Worker:** worker8 **Goal:** a schema-valid numeric env value validates and becomes the intended process string across Blueprint entry points **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Convert numeric env literals at the typed Blueprint boundary | 45m | — |
| t002 | Verify the shared containers, seeds, references and apply callers | 30m | t001 |
| t003 | Verify the real CLI journey and an owned local runtime | 20m | t002 |
| t004 | Render parity across Blueprint surfaces | 15m | t003 |
| t005 | Simplify | 10m | t004 |
| t006 | Test coverage | 20m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- `bex blueprints validate numeric.yaml --confirm -o json` and the unmodified Render v2.27.0 CLI against the same Bex API both exit 0 with `valid:true` for env `value: 47`, `47.5` and `1e3`. Quote-only controls retain the same result.
- Numeric literals work in root/ungrouped/project-environment services and env groups. Canonical process values are `47`, `47.5` and `1000`; an accepted integer such as `9007199254740993` retains every digit. String literals retain their exact text, including whitespace and leading zeroes.
- An owned local free web fixture receives the numeric value as the expected process string. A second unchanged Blueprint sync has no env-induced diff or deploy. Delete and absence-verify every owned fixture and child.
- Omitted/empty values, sync:false seeds, generated values, group links and sibling envVarKey copies preserve their existing semantics. Boolean/null/array/object values remain located schema refusals; previewValue stays unsupported. Do not change unrelated number fields or the pinned schema.
- Shared validate/preview/create/sync/direct-deploy paths and generated-manifest/disconnect consumers have meaningful regression coverage; REST/GraphQL/MCP and the dashboard consume the same outcome. Relevant full backend checks and lint pass. Acceptance distinguishes live checks from source-only coverage.

## Source + Goal linkage

- **Source:** [CLI QA finding](finding.md), w8 loop sweep 22, production Bex v0.3.2 and unmodified Render v2.27.0, 2026-10-09 UTC. Numeric input fails in both; changing only the YAML value to a string passes. A private exact-source probe independently reproduces the typed-boundary failure.
- **Goal linkage:** ADR008 reliable hosting/deploy-from-chat, ADR049 strict Render YAML compilation and ADR018 CLI/API parity. The pinned schema and capability registry already allow and translate numeric env literals.
- **Expected outcome:** an ordinary numeric configuration value reaches validation and deployment without requiring a quoting workaround or exposing an internal Go decoder error.
- **Why now:** the shared decoder rejects every such stack before its resources can be planned/applied, including env-group and nested-project forms. The correction and the caller/container/runtime verification exceed an hour: 7 tasks, 2h30m. This is one implementation bug with separate shared-behavior verification.
- **Parity:** included because Blueprint input behavior is exposed through REST/GraphQL/MCP and the dashboard. No new CLI flags, schema fork or adapter workaround is requested.

Scheduled only; no product fix or ship was performed by this hunt.
