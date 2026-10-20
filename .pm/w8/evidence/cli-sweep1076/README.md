# CLI sweep 1076 — undeployedChanges absent before first live deploy (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- On a still-deploying service, the read carries no `undeployedChanges` key, even after a docker-command PATCH (which → 200). The flag belongs to the post-first-deploy state (the m119/m120 held-release semantics); its absence pre-live is recorded without a claim about the post-live value.
