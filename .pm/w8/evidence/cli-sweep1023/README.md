# CLI sweep 1023 — branch/repo source switch on an image service (live, 2026-10-20)

Two disposable free web services created and deleted (204s). No new bug filed.

- `rootDir` on a prebuilt-image service → 400 (repo-only field, refused).
- `branch` PATCH alone → 200 but not rendered on the read (no repo yet, so there is nothing for a branch to qualify). A subsequent `repo` PATCH → 200 converts the service to a repo source (`runtime: docker`) and the previously-set branch surfaces alongside the repo. The two-step "Update Source" switch keeps the branch in the spec and renders it once it has a repo to bind to — recorded as designed behavior, not filed.
