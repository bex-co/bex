# CLI sweep 638 — `autoDeploy`/`autoDeployTrigger` PATCH dialects: string enums, empty means unchanged

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixture `qa638-web` (`srv-dbbb4tlfsg2s73ako6m0`, free hello-app), deleted at close: REST 404, zero `qa638` names. REST boundary.

- **`autoDeploy` is a string enum, not a boolean:** `"yes"`/`"no"` are accepted and stored; a JSON `true` and `"banana"` both fail with `invalid request body at /autoDeploy` (a rendered 400, not a silent coercion).
- **`autoDeployTrigger` enum:** `off` and `commit` accepted; the full grammar is named on refusal — `autoDeployTrigger must be "off", "commit" or "checksPass"` — which also refuses the plausible-but-wrong `"manual"`.
- **Empty string is the PATCH no-op:** `{"autoDeployTrigger": ""}` returned 200 and left the stored value (`commit`) untouched, consistent with the patch handler's documented empty-means-unchanged rule for string fields.

## Unrun

- `checksPass` behavior against a repo with failing checks (needs a controlled repo).
- Auto-deploy on push (needs a writable repo; this loop has none).
