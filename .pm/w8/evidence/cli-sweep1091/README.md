# CLI sweep 1091 — workspace audit trail covers the loop's own actions (live, 2026-10-20)

One disposable free web service created and deleted (204); audit reads were read-only. No new bug filed.

- The workspace audit log records the loop's control-plane actions in near-real time: `CreateServerEvent`, `DeleteServerEvent`, `secrets.SetEnvVars`, `secrets.SetSecretFiles`, and `apps.AddDomain` all appear with timestamps and success status. A first probe searched the entries for the fixture's service id and found nothing — audit `resource` is workspace-scoped (`workspace:tea-…`), so events are keyed to the workspace, not the individual resource.
