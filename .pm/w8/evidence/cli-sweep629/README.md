# CLI sweep 629 — secret file lifecycle on a live service: update and delete land without a redeploy

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixture `qa629-web` (`srv-dbbatb5fsg2s73ako5f0`, free busybox re-rendering `SF=[$(cat /etc/secrets/qa629.txt)]` every 2s), deleted at close: REST 404, zero `qa629` names. Contents were the probe strings `v1`/`v2`, nothing sensitive.

- **Create and read:** `PUT /secret-files/qa629.txt {content: v1}` → `201 {name, content}`; the live page read `SF=[v1]` once the first rollout settled.
- **Update is live, not a redeploy:** re-PUT with `v2` (again `201`, upsert semantics) showed on the page within ~15 seconds — the mounted file changed under the running process. Contrast with env vars, which roll the service (sweeps 577/585): secret files behave as a synced mount.
- **Delete is equally live:** `DELETE` → `204`, and the page read `SF=[]` within ~15 seconds; the file was gone from the mount, service unharmed.
- **Dialects:** a missing file's GET → `404 secret file not found` (named); a name containing a slash (`bad/name.txt`) doesn't match the route at all (plain `404 not found`).

## Unrun

- Secret file larger than a few bytes (size ceilings) and binary content.
- File update while the service is suspended (mount state on resume).
