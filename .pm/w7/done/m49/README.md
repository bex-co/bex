# w7 · m49 — Strict Render REST request validation with kin-openapi

**Worker:** worker7 **Goal:** put one offline, pinned Render OpenAPI contract in front of bex's authenticated Render-compatible REST handlers so malformed and unknown input is rejected before side effects, while bex-only routes, aliases, and documented extensions keep their current behavior **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin the complete Render spec + one kin-openapi loader — **DONE** | 45m | — |
| t002 | Classify the bex ∩ Render route and extension compatibility matrix — **DONE** | 45m | t001 |
| t003 | Add the safe shared REST request-validation gate — **DONE** | 1h | t002 |
| t004 | Reject unknown query/body fields and trailing JSON — **DONE** | 1h | t003 |
| t005 | Unify m30 conformance + the deliberate spec-refresh workflow — **DONE** | 45m | t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 30m | t006 |
| t008 | Test coverage — **DONE** | 45m | t007 |
| t009 | Closeout — **DONE** | 15m | t008 |

## Definition of done

An authenticated request whose method/path is implemented by bex **and** described by Render's pinned OpenAPI spec is validated before its REST handler can mutate state. Invalid required fields, known parameter/body types, enums, formats, patterns, ranges, media types, undeclared query parameters, unknown JSON fields, and trailing JSON return bex's Render-shaped `400` envelope without echoing request values or schema dumps. The handler receives the original bounded body unchanged after successful OpenAPI validation. Bex-only routes and aliases, public device/HMAC/deploy-hook routes, GraphQL, MCP, and SSE behavior remain unchanged; documented bex request extensions such as `dryRun`, `confirm`, `builder`, `port`, top-level `plan`, `domains`, `routes`, `headers`, and `idleTTLSeconds` remain accepted. Runtime response validation is not added.

## Research findings (2026-07-20)

### What exists now

- `w7/m30` produced `lego/backend/internal/api/testdata/render-openapi.json`, but the checked-in file is a hand-maintained **response-only subset**: OpenAPI 3.0.3, 12 paths/operations, 16 component schemas, and zero request bodies. Its test-only stdlib walker deliberately skips enum, format, and unknown-property checks. It cannot protect request handlers.
- `Server.Handler()` has one authenticated REST composition seam: `auth(rateLimit(restHandler))` in `lego/backend/internal/api/server.go`. The outer `withBodyLimit` already caps and restores non-GET bodies (2 MiB by default). Most feature adapters then call `json.NewDecoder(r.Body).Decode` without `DisallowUnknownFields`, so unknown keys are silently ignored today.
- `core.WriteErrStatus` already owns the one Render-compatible REST error envelope (`id`, `error`, `message`). The validator must delegate to it instead of introducing kin's default error body.

### Official spec spike

- Source: `https://api-docs.render.com/openapi/render-public-api-1.json`; Render warns that this spec is unversioned and subject to change, so production and CI must never fetch it live.
- The 2026-07-20 snapshot is 331,391 bytes, SHA-256 `2d27d5834d8bbc586e0aee62160cf996bb07f4be747112f15ec02c14fc11b315`, OpenAPI 3.0.2 / info version 1.0.0, with 130 paths, 207 HTTP operations, 58 request bodies, and 163 component schemas.
- kin-openapi v0.142.0 loads and validates that snapshot when the loader uses the narrow compatibility exception `openapi3.AllowExtraSiblingFields("description", "default")`. The spec currently has exactly three `$ref` objects with those non-extension siblings. No example-validation bypass is needed.
- The official server is `https://api.render.com/v1`. The middleware should clear `doc.Servers` and route a cloned request whose path has exactly one leading `/v1` removed; it must not mutate the request passed to the bex handler or bind validation to Render's hostname.

### Why kin alone is insufficient

- Of the official spec's 290 object schema nodes, 287 omit `additionalProperties` and 3 supply an additional-property schema; none set it to `false`. A real kin spike accepted a valid `POST /services` body containing `notInSpec`, and kin intentionally ignores unknown query parameters. Therefore `openapi3filter.ValidateRequest` enforces declared constraints but does **not** meet the unknown-input requirement by itself.
- The simplest second layer is not a generated overlay: derive allowed query keys from the matched operation plus a small, reviewed bex-extension allowlist, and make Render-covered JSON handlers decode into their existing Go request structs with one shared strict helper (`DisallowUnknownFields` + exactly one JSON value). Those structs are already the authoritative list of bex extensions.
- v0.142.0's `RejectWhenRequestBodyNotSpecified` currently rejects a valid body even when the matched operation defines one. Leave it off and perform the narrow `operation.RequestBody == nil && body-present` check in the bex wrapper, with a regression test.
- Raw kin request errors can include the submitted value and a full schema dump. Never return or log `err.Error()`; map typed parameter/schema errors to a bounded safe message (parameter name or JSON pointer only), fail fast, and never include body/header values.

### Recommended request path

`body limit → auth → rate limit → bex∩Render route match → kin declared-constraint validation → strict Go JSON decode → existing core/domain validation`

Use `openapi3filter.Options{AuthenticationFunc: NoopAuthenticationFunc, SkipSettingDefaults: true, MultiError: false, SchemaValidationOptions: []openapi3.SchemaValidationOption{openapi3.EnableFormatValidation()}}`. Do not let validation set defaults or normalize the request. Keep response validation in CI only: production response middleware would buffer streaming/SSE responses and is outside this input-hardening goal.

## Source + Goal linkage

- **Source:** user request, “research how to integrate kin-openapi with Render's OpenAPI specs into our bex REST APIs and hand off to `$pm`,” 2026-07-20; follows up `w7/done/m30` rather than duplicating it.
- **Goal linkage:** `.pm/GOAL.md` #7 (security review) and `docs/ADR008-vision.md` Pillar 1 / `docs/ADR006-bex-api.md` (a deterministic Render-compatible public API with thin adapters).
- **Expected outcome:** malformed or undeclared input is rejected at the public REST boundary before any store/Kubernetes side effect, and one full pinned spec becomes the shared contract asset for request protection and response conformance tests.
- **Why now:** the public API already accepts many mutation bodies with lenient `encoding/json`; m30 gives CI confidence about a small response subset but leaves request input completely outside the contract. Adding more clients and service fields compounds that silent-input risk.
- **Render parity — INCLUDED (t006):** valid official Render CLI/API requests must continue to work. Stricter rejection of undeclared input is a documented bex security extension because Render's own spec leaves objects open; GraphQL/MCP are not changed by a REST-only contract gate.

## Explicit non-goals

- Generating or replacing bex handlers/types from Render's spec.
- Runtime response validation, validation of SSE streams, or validation of GraphQL/MCP with a REST contract.
- Automatically fetching Render's unversioned spec at startup or in ordinary CI.
- Canonicalizing/trimming strings, Unicode normalization, rejecting duplicate JSON keys, or replacing existing domain validation. This milestone validates and rejects; it does not rewrite user input.

## Resolution

Completed 2026-07-20. One SHA-pinned complete Render contract now protects the authenticated 119-operation bex ∩ Render REST intersection after auth/rate limiting, with safe strict query/JSON handling and exact compatibility boundaries. The same document drives kin-based response conformance in CI; the old subset and custom walker are gone. Documentation records the route/extension matrix, deliberate requiredness compatibility, stricter-than-Render unknown-input policy, and manual refresh workflow. Full backend tests and backend lint pass; no production response validation, live fetch, GraphQL/MCP change, or automatic normalization was added.
