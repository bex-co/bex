# w8 · m121 — A release in flight builds its own source after a later save

**Worker:** worker8 **Goal:** A release in flight builds and runs what it was requested with, even when a later save arrives before its build starts. That later save is an Update Source (repo, branch, or a switch to a prebuilt image) or a newer deploy. Render's contract is that Update Source saves now and the next deploy consumes it, so release N never builds the saved source under its own name, and never rolls N−1's image as `rev-N`. **Status:** in progress — t001–t004, t006, t007 done (a held release runs on its adopted view: own build ref, image, pre-deploy, snapshots, launch); next: deploy, t005 live parity, t008 closeout

## Tasks (in order)

| id   | title                                                                                     | est | depends_on                 |
| ---- | ----------------------------------------------------------------------------------------- | --- | -------------------------- |
| t001 | Design: carry a release's own build inputs from adoption to its build                     | 45m | — — **DONE** |
| t002 | Operator: capture a release's build inputs when it is adopted                             | 60m | w8/m121/t001 — **DONE** |
| t003 | Operator: a held release builds, and resolves its image, from its own inputs              | 90m | w8/m121/t002 — **DONE** |
| t004 | Envtests: an undispatched build, a repo-to-image switch, and a newer commit while queued  | 60m | w8/m121/t003 — **DONE** |
| t005 | Render parity: deploy history and served commit across REST, GraphQL, MCP, UI             | 45m | w8/m121/t004               |
| t006 | Simplify the changed code                                                                 | 20m | w8/m121/t005 — **DONE** |
| t007 | Test coverage: own-source builds and image selection for held releases                    | 40m | w8/m121/t005 — **DONE** |
| t008 | Close out the milestone                                                                   | 10m | w8/m121/t006, w8/m121/t007 |

## Definition of done

Live on production after deploy, with the pinned CLI and an owned fixture deleted by ID with a 404 absence proof:

- **Undispatched build.** A repo-backed web service's release N is still queued or waiting when an Update Source moves its branch to a branch with different code. N builds its own branch, serves N's commit as `rev-N`, and its row names that commit. The new branch serves only after the next deploy.
- **Repo to image.** An Update Source switches the service to a prebuilt image while repo-backed N is still building. N completes from its own repo, or closes with a named terminal. It never rolls N−1's image as `rev-N`. The image serves on the next deploy.
- **Newer deploy while queued.** A deploy of commit Y arrives while N, for commit X, is queued. N builds X, then Y builds and serves as its own release.
- **Controls.** A single deploy, a cancel of the pending deploy (w8/m117), the w8/124 started-build case, and m120's held settings keep their behavior.
- **Tests.** Operator envtests assert each case's applied image and build ref, and fail on the old code.

## Source + Goal linkage

- **Source:** inbox note [w8/126](../done/126.md), from the w8/124 review on 2026-10-19, promoted the same day. Triage confirmed the premise in code: in `prepareAppReleaseDecision`'s `sourcePending` branch an unpinned release takes `reusableArtifactImage`'s `status.image`, the previous image, while its generation stays `rev-N`. Triage also found that `buildFromSource` reads `effectiveDeployRef(app.Spec)` and every other build input from the saved spec. So the regular latest-pending pin (`release_identity.go:484`) builds a newer deploy's commit under N's fingerprint whenever N's build is still queued at the build cap.
- **Goal linkage:** ADR060 §D1a run-to-completion and Render's Wait overlap policy and Update Source contract (`AnnotationPendingSourceGeneration`, `app_types.go:50`). Deploy history must describe what ran ([ADR018](../../../docs/ADR018-render-parity.md) lifecycle rows). This continues w8/m119 → m120, which fixed a held release's runtime settings; this milestone fixes its build inputs.
- **Expected outcome:** no release ever builds or rolls another release's source under its own generation. The deploy row, `activeRevision` and the running image agree.
- **Why now:** w8/124 and m120 just made the held-release path the norm for overlaps. A build queued at the workspace cap is a routine state on busy workspaces, so the mislabeled build is reachable without a race. Render parity is included because deploy history and the served commit are tenant-visible on REST, GraphQL, MCP and the dashboard.
