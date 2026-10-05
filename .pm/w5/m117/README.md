# w5 · m117 — Every create field reaches the service through REST, GraphQL and MCP, and Blueprint export round-trips

**Worker:** worker5 **Goal:** A guard test fails whenever a transport silently drops or mis-maps a create field, and Blueprint Generate → parse reproduces what a real create stored. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Give GraphQL service create its own mapping function | 30m | — |
| t002 | Guard that every create field is used or refused per transport | 1h | t001 |
| t003 | Fix whatever the guard finds | 45m | t002 |
| t004 | Drive the Blueprint round-trip from real creates | 1h | t001 |
| t005 | Render parity | 20m | t003, t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- For REST, GraphQL and MCP × runtime (docker, image, native, builder-only), every service-create wire field reaches `CreateRequest` or is refused with 400, and adding an unmapped field fails the guard.
- Create → GenerateBlueprint → parse → `specFromCreate` reproduces every exportable field for the probe set and the type × runtime × builder matrix; non-exportable fields are listed.
- Reverting w4/188 or w4/193 locally makes the respective guard fail.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05. w4/188 (`4fe79515b`) fixed REST dropping `dockerCommand`, but MCP still drops it (w5/073). w4/193 (`24482892a`) fixed Generate Blueprint for plain Dockerfile services, after w6/m114 had shipped a similar bug past the hand-built round-trip test.
- **Goal linkage:** ADR006/ADR018 Render-compatible API; ADR049 render.yaml parity.
- **Expected outcome:** No transport silently loses a field, and Generate Blueprint exports what was created.
- **Why now:** Two transport/export bugs in 24 hours, and w5/m116's plan seam makes the guard cheap.
- **Render parity included:** this milestone is a parity guard for create and export.
- **Depends on w5/073** (MCP `dockerCommand`); best done after w5/m116.
