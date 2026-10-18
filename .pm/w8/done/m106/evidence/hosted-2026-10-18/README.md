# m106 hosted acceptance — 2026-10-18

[Results with build-log lines](results.json).

- **Where:** production `api.bex.co`, serving deploy `429d49ccf`, which contains the fix. QA workspace bex-canary.
- **Clients:** both native clients, the bex CLI v0.3.3 and the pinned Render CLI, each creating its own fresh Free static site with `--auto-deploy=false`.
- **Auth:** a temporary QA API key (client_credentials), revoked after the run. The device login needs the QA password, so it was not used.
- **Source:** render-examples/hugo-quick-start@48eaf95 (main), unchanged, Ananke theme as its pinned submodule.
- **Command:** Render's documented `hugo --gc --minify`; publish `public`.

Both native clients' fresh Free static sites built Render's Hugo quickstart unchanged. Hugo rendered 10 pages, and the site is Live with HTTPS 200 serving the Deploy Hugo on Render page (generator Hugo 0.111.3).

| client | fixture | first deploy | HTTPS | URL after delete |
| --- | --- | --- | --- | --- |
| bex | `qa-lx88637d-m106-bex` | live | 200 | 404 |
| render | `qa-lx88637d-m106-render` | live | 200 | 404 |

**Cleanup:** every fixture was deleted through its own client (`deleted: true`). Each is absent from the service list and its URL returns 404. The workspace inventory afterwards matches the pre-run baseline. No other resource was touched.
