# w4 · m629 — Claude Code, Codex and Cursor connect to bex's MCP with no flags: a DCR client's RFC 8707 `resource` becomes its token's audience

**Worker:** worker4 **Goal:** `claude mcp add --transport http bex https://api.bex.co/mcp` (and Codex's and Cursor's equivalents) sign in through bex's OAuth with no client id, no flag and no API key, as `docs/ADR025-connect-an-agent.md` promises and as no spec-following client has ever done. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | A DCR client's RFC 8707 `resource` becomes its token's audience | 2h | w31/001 |
| [t002](t002.md) | A client that registers without `scope` still gets bex's capability scopes after consent | 30m | t001 |
| [t003](t003.md) | The OAuth e2e harness registers and authorizes like a real MCP client | 40m | t001 |
| [t004](t004.md) | Docs say what actually works for Claude Code, Codex and Cursor | 30m | t001 |
| [t005](t005.md) | Live: Claude Code and Codex connect to production with no flags | 1h | t002, t003, t004, w31/001 |
| [t006](t006.md) | Render parity | 25m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 30m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 6h**.

## Definition of done

- On production, once the pin carries this milestone and w31/001: a fresh Claude Code with no client id and no API key completes OAuth against `https://api.bex.co/mcp` (discovery → DCR → consent → token) and `list_workspaces` answers; a call after the 15-minute access token expires still works.
- Codex does the same with `codex mcp login`, or its absence on the worker host is recorded.
- A token minted for a DCR client that sent `resource=https://api.bex.co/mcp` carries that audience; a token for any other `resource` does not, and `BEX_OAUTH_REQUIRE_AUDIENCE=1` and the w1/m67 F1 rule are unchanged.
- `scripts/auth-oauth21-e2e.sh` has a leg that registers and authorizes like a real MCP client and passes.
- ADR025, ADR012 §7, the bex-mcp skill and ADR018 describe what works; no stale Cursor claim remains.
- No test DCR client is left in production Hydra, including the empty one a verification probe created on 2026-11-25 at about 07:20:16 UTC.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one more aspect of render parity for w4`, 2026-11-24 (the aspect: how a coding agent connects to bex, as Render's agent docs describe), each item re-verified by an independent agent on 2026-11-24/25 before filing; filed by `/pmx them all to w4` at `origin/main` `50a5ddfdfc`. It replaces the proposal's first draft (pre-registering Render's client ids), which was withdrawn after the user pointed out that bex already offers OAuth; checking why that OAuth path was never proven found this.
- **Evidence (verified 2026-11-24/25 at `2f964da9e8`):**
  - Hydra v26.2.0 publishes `registration_endpoint` only from `webfinger.oidc_discovery.client_registration_url`, which bex never set, so the MCP TypeScript SDK (Claude Code 2.1.322 throws "Incompatible auth server: does not support dynamic client registration") and Codex's rmcp ("Dynamic client registration not supported") stop at registration. That half is **w31/001** (filed 2026-11-25 from `/qa-find-bugs-mcp w31`), which this milestone depends on rather than duplicates.
  - The next leg then fails: real clients send RFC 8707 `resource=`, Hydra reads only `audience`, the consent acceptor grants only the requested audience (`hydra-consent.ts:586-587`), and production's `BEX_OAUTH_REQUIRE_AUDIENCE=1` makes `auth.go:648-662` refuse the audience-less human token.
  - The proof that "DCR works" (w4/m454 t004, 2026-11-17) ran `scripts/auth-oauth21-e2e.sh`, which hard-codes the registration URL (`:280`) and sends `audience` at registration and authorize (`:293`, `:447`). w4/m16 t004 recorded that the real Claude Code/Cursor walkthrough was never done.
- **Goal linkage:** ADR008 AI-native (agents are first-class users) and the MCP surface of Render parity.
- **Expected outcome:** every discovery-driven MCP client connects to bex with OAuth and no stored credential.
- **Why now:** it blocks every OAuth agent user today, and w4/m631 (plugin) and w4/m632 (`agents.md`) point agents at this path.
- **Render parity:** included (MCP and OAuth surface). Render instead pre-registers client ids and offers no DCR; bex keeps DCR as its documented path (ADR018 bex ahead). Pre-registered ids are not part of this milestone.
