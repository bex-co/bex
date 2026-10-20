# CLI sweep 1024 — cross-workspace direct access is membership, not isolation failure (live, 2026-10-20)

One disposable free web service created in `bex-canary`, deleted during the probe sequence (canary GET 404 and name list empty afterwards). No new bug filed.

- First pass looked alarming: reading, PATCHing, and DELETEing the canary service while sending the *other* workspace's header (`tea-d982…`, "bex") all succeeded.
- Verification dissolved it: the QA user is an `ADMIN` member of **both** workspaces (both member lists show the same user as ADMIN). The session is account-level; authorization is evaluated against the resource's owning workspace, where the caller is an admin — so direct-id access is membership working as designed (the same model as Render's account-wide API keys). The workspace header scopes *lists* (the canary service does not appear in the other workspace's list) and default-owner resolution; it is not itself the authz boundary.
- The real tenant boundary was already proven the other way: owners the user does not belong to answer 403 on detail, members, and audit logs (sweeps 958/973/974).
