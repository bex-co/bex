# CLI sweep 529 — SSH-form repo URLs accept, then always fail to clone

**Verdict: one bug filed** as [w8/130](../../130.md). Same isolated session, `bex-canary`, HEAD build `218dffcd1`, 2026-10-19T09:53–09:56Z. Continues sweep 528's repo-source matrix into the SSH forms it left unrun.

| step | observed result |
| --- | --- |
| `create --repo git@github.com:bex-co/bex.git` (public repo) | Exit `0`, service created; deploy resolves the head SHA (`@218dffc`) then `build_failed` in the clone step: `Host key verification failed. / fatal: Could not read from remote repository.` — verbatim in build logs and `failureReason`. |
| Code trace | Create admission deliberately accepts SCP/`ssh://` forms (`store.ValidRepo`); the submodule phase rewrites GitHub SSH to HTTPS and its header claims the parent repo follows the same rule (`execution/gitsubmodule.go:25-27,41-42`), but neither parent clone phase applies it — build plane `build/build.go:1493-1499`, static publish plane `publish/publish.go:585-587` both fetch `$REPO` verbatim. No board coverage found. |

Full record, blast radius, fix shape, and acceptance: [w8/130](../../130.md). The filing is uncommitted (no `/ship` authorization in this loop request).

## Safety and unrun checks

- Cleanup: exact-owned `srv-dbaugn7b7irc73chfvo0` deleted by ID; no nonce-named resources remain in the workspace.
- Unrun (in 130's acceptance instead): `ssh://git@github.com/…` live form, the static-publish plane live leg, private-GitHub-repo-in-SSH-form (would need a private fixture repo the QA identity doesn't have).
- Deploy lag means deployed revisions can precede local `main`; the missing rewrite is absent at HEAD in both planes, so lag does not explain it.
