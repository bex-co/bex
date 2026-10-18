# m90 hosted acceptance — 2026-10-18

[Results with build-log lines](results.json).

- **Where:** production `api.bex.co`, serving deploy `429d49ccf`, which contains the fix. QA workspace bex-canary.
- **Clients:** both native clients, the bex CLI v0.3.3 and the pinned Render CLI, each creating its own fresh Free static site with `--auto-deploy=false`.
- **Auth:** a temporary QA API key (client_credentials), revoked after the run. The device login needs the QA password, so it was not used.
- **Source:** render-examples/express-hello-world@e7a5c56 (master), its committed package.json + Yarn Classic yarn.lock.
- **Command:** an authored `node -e` build that instantiates Express and writes `_site/index.html` with its version; publish `_site`; `NODE_VERSION=24`.

Both native clients' fresh Free static sites built with the bare dependency-using command and no installer prefix. The log shows `==> Installing dependencies with Yarn (yarn.lock)` and `yarn install v1.22.22 … Done`, and the site is Live with HTTPS 200 serving `<p>lx express 4.18.2</p>` (the lock's Express). The `SKIP_INSTALL_DEPS=true` control (Render CLI) logs `==> Skipping dependency installation (SKIP_INSTALL_DEPS=true)`, then `Cannot find module 'express'`; the build fails and the URL returns 404.

| client | fixture | first deploy | HTTPS | URL after delete |
| --- | --- | --- | --- | --- |
| bex | `qa-lx88637d-m90-bex` | live | 200 | 404 |
| render | `qa-lx88637d-m90-render` | live | 200 | 404 |
| render | `qa-lx88637d-m90skip-render` | build_failed | HTTP Error 404: Not  | 404 |

**Cleanup:** every fixture was deleted through its own client (`deleted: true`). Each is absent from the service list and its URL returns 404. The workspace inventory afterwards matches the pre-run baseline. No other resource was touched.
