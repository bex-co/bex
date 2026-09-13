# w4 · m104 — `PUT /v1/services/{id}/autoscaling` is unusable: the pinned contract and the handler want different bodies

**Worker:** worker4 **Goal:** a Render-compatible client can actually set autoscaling over REST, and the class of defect that makes a validator-gated route unusable is audited away rather than fixed one route at a time — this is its third recorded instance. **Status:** todo

## Tasks (in order)

| id   | title                                                                         | est | depends_on   |
| ---- | ----------------------------------------------------------------------------- | --- | ------------ |
| t001 | Make the autoscaling PUT accept a body, and say which shape is canonical         | 60m | —            |
| t002 | Audit every validator-gated write route against its handler's decoder            | 75m | w4/m104/t001 |
| t003 | Stop answering a schema-rejected body with a bare `bad request`                  | 40m | —            |
| t004 | Regression tests: one per route shape, plus the GraphQL/MCP control pair          | 45m | w4/m104/t001, w4/m104/t002 |
| t005 | Render parity sweep over the changed surfaces                                    | 30m | w4/m104/t004 |
| t006 | Simplify pass over this milestone's changes                                      | 25m | w4/m104/t005 |
| t007 | Test coverage for the shipped behavior                                           | 40m | w4/m104/t005 |
| t008 | Closeout                                                                        | 15m | w4/m104/t007 |

## Definition of done

- **A Render-shaped body sets autoscaling.** `PUT /v1/services/<web-srv-id>/autoscaling` with exactly the body Render's pinned schema requires — `{"enabled":true,"min":1,"max":1,"criteria":{"cpu":{"enabled":true,"percentage":60},"memory":{"enabled":false,"percentage":0}}}` — returns 200, and `GET …/autoscaling` then reads `enabled: true` with the min/max it was given. Today that exact body returns `400 {"error":"bad request"}` and the GET still reads `{"enabled":false,"minInstances":0,"maxInstances":0}`.
- **Whichever shape is canonical is documented and the other is handled or refused clearly.** If the handler adopts Render's `min`/`max`/`criteria`, `docs/ADR006-bex-api.md` records it and the internal `minInstances`/`maxInstances` form is either accepted as a documented bex alias or rejected with a message naming the accepted keys. "Both shapes silently half-work" is not an acceptable end state.
- **No body is rejected with an unpointed error.** For every rejected body the response names the offending field, as `w4/038` and `w4/041` already established for the projects routes. Today: bex's own shape → `invalid request body at /enabled`; Render's shape → bare `bad request`; the union of both → bare `bad request`.
- **The GraphQL/REST control pair agrees.** The same intent expressed through `setAutoscaling` (GraphQL) and `PUT …/autoscaling` (REST) produces the same stored config. Today GraphQL succeeds and REST cannot, on the same service, seconds apart — verified below.
- **Every validator-gated write route is checked, with a count.** t002 enumerates bex's `DecodeBody[…]` REST sites (today **24**, across `apps`, `envgroups`, `secrets`, `environments`, `projects`, `notifications`) against the **58** Render paths carrying a `requestBody`, and reports for each implemented route whether the schema and the decoder agree, leaving the check behind as a test. **A clean result is the expected outcome**: a 9-route sample (pass 10 — projects, env-groups, service PATCH, env-vars, scale, deploys, headers, routes, custom-domains, including the two previously-fixed controls) found every one working with a schema-conformant body, so autoscaling is an outlier rather than the tip of a pattern. The task's value is the standing guard against a fourth instance, not an expected pile of fixes.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-13 pass 9, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`, against production `api.bex.co`. Fixtures `qa-0913i-st` (`srv-dajd7togsm7s73f63p40`, static site) and `qa-0913i-web` (`srv-dajd930gsm7s73f63p60`, free web service), both created and deleted in the run. Every probe and response is in t001.
- **This is the third instance of one pattern, not a new bug.** `w7/m49` (done) put a strict, pinned Render OpenAPI validator in front of the authenticated REST handlers. Two routes have since been found where the validator's schema and the handler's decoder disagree, each surfacing as an unactionable 400: `w4/041` — "`POST /v1/projects` opaquely rejects the Render-contract `environments` field … bex's own pinned contract advertises `environments` as acceptable, so this is a REST/Render-parity gap plus an opaque error" — and `w4/038`, the same on `GET /v1/projects`. Both are fixed. Autoscaling is the same class and strictly worse: not merely opaque, but with **no** body that satisfies both layers, so the endpoint cannot be used at all. That recurrence is why t002 audits the surface instead of patching one route.
- **Goal linkage:** `docs/ADR006-bex-api.md` (REST is Render-compatible; the three adapters must not drift) and `docs/ADR018-render-parity.md`. The autoscaling row is currently backed by a GraphQL implementation that works and a REST implementation that does not.
- **Expected outcome:** the official Render CLI, an SDK generated from Render's OpenAPI, or any CI script can set autoscaling over REST. Today only the dashboard can, because only the dashboard uses GraphQL.
- **Why now:** it is a documented endpoint that returns 400 to every well-formed request, the defect class has already recurred twice, and the audit that would have caught all three costs less than finding them one at a time from QA.
- **Render parity task included:** yes — this is a REST wire-contract fix with GraphQL and MCP siblings.

## Evidence, and what is not claimed

- **Verified live** (full transcripts in t001): three body shapes × two service types, all 400; the same GraphQL mutation succeeding on the same web service with `GET …/autoscaling` then reading `{"enabled":true,"minInstances":1,"maxInstances":1,"targetCPUPercent":60}`; and the pinned schema's exact requirements read out of `lego/backend/internal/api/openapi/render-public-api-1.json` (`required: [enabled, min, max, criteria]`, `criteria.required: [cpu, memory]`, each with `required: [enabled, percentage]`).
- **The mechanism is partly unexplained, and t001 must settle it before fixing.** The two layers clearly disagree — `render_openapi.go:455-490` produces the `invalid request body at /<pointer>` messages from the pinned schema, and `apps/rest.go:1112` decodes `SetAutoscalingRequest` = `{minInstances, maxInstances, targetCPUPercent?, targetMemoryPercent?}` (`apps/service.go:4349-4354`) — but that does **not** predict the **union** body failing: it carried valid `minInstances`/`maxInstances` alongside Render's keys, so it should have satisfied the schema *and* decoded to a usable spec, yet it returned a bare `bad request`. Something further rejects it (a strict decoder refusing unknown fields, or a response-side check). Do not build the fix on the two-layer story alone until that is instrumented.
- **`autoscalingSpec`'s own bounds are not the cause**: `min:1/max:1` is within the free plan's cap, and the same values succeed through GraphQL on the same service.
- **Scoped by sampling (pass 10):** nine validator-gated write routes were probed with schema-conformant bodies and **all nine succeeded** — full table in t002. The historical recurrence (three instances) is real but the current surface looks clean apart from autoscaling, which is why t002 is framed as a guard rather than a fix list.
- **Not probed:** MCP's autoscaling surface (`apps/mcp.go:865` exposes only the delete; whether a setter exists through the folded settings tool is unconfirmed — `w4/m101/t001` already owns that question), and whether any of the other 57 Render request-bodied paths mismatch. t002 owns the latter.
