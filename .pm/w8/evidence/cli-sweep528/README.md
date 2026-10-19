# CLI sweep 528 — repo-source failure dialects

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD build `218dffcd1`, 2026-10-19T09:32–09:51Z. Two doomed fixtures were created on purpose, watched to their terminal failure, and deleted; no `5b6da0`-nonce resources remain.

| probe | observed result |
| --- | --- |
| `--repo https://github.com/bex-co/qa528-does-not-exist-zzz` | Refused **at create**, `400`: "repository is not accessible through this workspace's GitHub connections or as public Git; connect its owner and grant repository access, or use a public repository". Nothing created. |
| `--repo not-a-url` (with build/start commands) | `400: repo must be an https/ssh/git URL`. Nothing created. (Without the commands, the client first refuses `"--build-command and --start-command are required when runtime is native"` — upstream input validation ordering.) |
| `--repo https://gitlab.com/foo/bar` (nonexistent) | **Accepted** as generic public Git (the DO_NOT_DO GitLab exclusion is the provider *integration* — App connection, repo listing, webhooks — not the public-clone path). Deploy queued behind the workspace's 2/2 build slots (build log says so), then `build_failed`: `failureReason` and build logs name the clone step (`fatal: could not read Username for 'https://gitlab.com': terminal prompts disabled` — GitLab's uniform answer for nonexistent/private). Deleted after. |
| Real public repo, `--branch qa528-no-such-branch-zzz` | Accepted at create; build starts from `…@qa528-no-such-branch-zzz` and fails `build_failed` with `fatal: couldn't find remote ref qa528-no-such-branch-zzz` in both `failureReason` and build logs. Critically, it does **not** fall back to the default branch. Deleted after. |
| Repeat create with an in-use name | `409 (CONFLICT): name "…" is already in use` (hit incidentally when a probe reused a name). |

The three rejection layers are consistent: what bex can prove at create (GitHub-side accessibility, URL shape) refuses up front; what only the clone can prove (generic-host existence, branch existence) fails fast at build with the verbatim git reason — bounded, named, and never silently re-targeted.

## Safety and unrun checks

- Cleanup: `srv-dbau7efb7irc73chfvbg` and `srv-dbauaj2ot0kc73chus7g` deleted by ID (deploy reads then 404); service count back to this sweep's baseline; no nonce-named resources remain.
- One transient local DNS failure (`lookup api.bex.co: no such host`) hit a single read mid-sweep and cleared on retry — recorded as harness noise, not a finding.
- Unrun: SSH-form repo URLs, a private GitHub repo without connection grant (same create-refusal class as the nonexistent-repo probe), repo moved/renamed redirects.
- Deploy lag means deployed revisions can precede local `main`.
