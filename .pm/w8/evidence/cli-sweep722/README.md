# CLI sweep 722 — build-failure retention on a git service (live, 2026-10-19)

Same fixture as sweep 721, deleted after this sweep and verified absent (REST 404).

- `--build-command` on the docker runtime is refused client-side: `--build-command and --start-command are only supported for native runtimes`; the redeploy that followed (unchanged source) went `live` normally.
- Pointing `--root-directory` at a nonexistent directory and redeploying: deploy reached `build_failed` within ~1 minute; the previously live version kept serving (`OK`) throughout.
- The event feed closed the lifecycle correctly: `build_started` → `build_ended` → `deploy_ended` (the w6/m128 canceled/failed-build closure).

No bug filed.
