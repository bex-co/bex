# CLI sweep 995 — autoDeployTrigger semantics on image services (live, 2026-10-20)

Two disposable free web services created and deleted (204s). No new bug filed.

- `autoDeployTrigger: "checksPass"` on a prebuilt-image service → coded 400 explaining itself: `needs a Git repository; a prebuilt image has no commits to check`. State unchanged afterwards.
- `autoDeployTrigger: "commit"` → 200 (stored, `autoDeploy: yes`); a bogus trigger value → 400; setting `off` → 200. Whether `commit` should also refuse on image services is not pinned by the Render contract, so the acceptance is recorded, not filed.
