# CLI sweep 1074 — the environment subresource is the env-config save endpoint (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `PATCH /services/{id}/environment` exists (distinct from the refused `environmentId` body field in sweep 1073): it is the environment-configuration save endpoint, and its first validation is coded — `saveMode must be "save_only", "deploy" or "rebuild"`. Deeper env-save semantics were covered by the earlier env sweeps and are not repeated here.
