# m105 hosted acceptance — 2026-10-18

[Results with build-log lines](results.json).

- **Where:** production `api.bex.co`, serving deploy `429d49ccf`, which contains the fix. QA workspace bex-canary.
- **Clients:** both native clients, the bex CLI v0.3.3 and the pinned Render CLI, each creating its own fresh Free static site with `--auto-deploy=false`.
- **Auth:** a temporary QA API key (client_credentials), revoked after the run. The device login needs the QA password, so it was not used.
- **Source:** render-examples/jekyll@3b8ab7b (master), unchanged.
- **Command:** Render's documented `bundle exec jekyll build`; publish `_site`.

Both native clients' fresh Free static sites built Render's Jekyll quickstart unchanged. The log shows `==> Using Ruby 3.3 (from .ruby-version)`, `==> Installing dependencies with Bundler (Gemfile.lock)` and `Bundle complete! 4 Gemfile dependencies, 35 gems now installed.`, and the site is Live with HTTPS 200 serving the Jekyll on Render page (generator Jekyll v4.3.3).

| client | fixture | first deploy | HTTPS | URL after delete |
| --- | --- | --- | --- | --- |
| bex | `qa-lx88637d-m105-bex` | live | 200 | 404 |
| render | `qa-lx88637d-m105-render` | live | 200 | 404 |

**Cleanup:** every fixture was deleted through its own client (`deleted: true`). Each is absent from the service list and its URL returns 404. The workspace inventory afterwards matches the pre-run baseline. No other resource was touched.
