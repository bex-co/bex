# CLI sweep 603 — Git-backed web service: full build → live → logs flight

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `1a49cb806`, 2026-10-19. Owned fixture `qa603-git` (`srv-dbb9npcgnrlc73bhsse0`, free, `--repo https://github.com/render-examples/express-hello-world --runtime node`), deleted at close: REST 404, URL 404, zero `qa603` names. Recent sweeps used image fixtures; this re-proves the Git path end to end.

- **Lifecycle states were truthful throughout:** the deploy walked `created → queued → build_in_progress → update_in_progress → live` over ~3 minutes while the public URL served the "first deploy is in progress" page, then the app's real page (`Hello from Render` Express sample).
- **Commit provenance:** the live deploy records the actual HEAD commit (`039c3477…`, message `Add Render deployment configuration`) — the omitted `--branch` resolved to the repo's default branch and the deploy built that ref.
- **Logs split correctly:** `logs --type build` returns the image-build stream (`Copying blob sha256:…`, `Your service is live 🎉`); `logs --type app` returns the process stream (`express-hello-world@1.0.0 start`, `Example app listening on port 3000!`).

## Unrun

- `--commit` deploy of an older SHA on this Git-backed fixture (image-side refusal proven in sweep 584; Git-side acceptance not re-run).
- Auto-deploy on push (needs a repo this loop controls).
- Build-filter paths suppressing a redeploy.
