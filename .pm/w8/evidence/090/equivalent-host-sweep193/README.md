# Equivalent production API URL bypasses dashboard defaults — sweep193

**Severity: minor. Ownership: Bex launcher. Existing item: w8/090; no duplicate. Remaining implementation: 25m.** On 2026-10-13 UTC, current checkout `c5a0eb9dd15f6b417bf586fdf26510706497b032` still passes a relative filesystem path to the OS opener when the supported production API base is written without its trailing slash. This is an incomplete current-source fix, beyond the previously recorded need for a release.

With an isolated missing/version-only config and privately supplied authorized environment bearer, run the current Bex build with `BEX_HOST=https://api.bex.co/v1 bex services --confirm -o interactive`. Enter a metadata row and its Dashboard action. Native service loading succeeds, but the opener receives `/cron/srv-dap3v694dm7c7390q930` and no dashboard origin is saved. Adding only the trailing API slash, or an explicit local dashboard origin, opens the absolute Bex URL. The equivalent explicit `RENDER_HOST` override also fails. A saved same-origin host without the slash independently triggers the second veto even while the environment credential selects the slash-ended default API host; that case does not test saved-credential connection precedence.

| Native process | Effective API base | Opener argument | Duration |
| --- | --- | --- | --- |
| head-version-noslash | `https://api.bex.co/v1` | `/cron/srv-dap3v694dm7c7390q930` | 1.943s |
| head-missing-noslash | `https://api.bex.co/v1` | `/cron/srv-dap3v694dm7c7390q930` | 1.731s |
| head-stored-noslash | `https://api.bex.co/v1/` | `/cron/srv-dap3v694dm7c7390q930` | 1.55s |
| head-version-noslash-renderoverride | `https://api.bex.co/v1` | `/cron/srv-dap3v694dm7c7390q930` | 1.205s |
| head-version-noslash-control | `https://api.bex.co/v1` | `https://dashboard.bex.co/cron/srv-dap3v694dm7c7390q930` | 1.27s |
| head-version | `https://api.bex.co/v1/` | `https://dashboard.bex.co/cron/srv-dap3v694dm7c7390q930` | 1.17s |
| render-version-noslash | `https://api.bex.co/v1` | `/cron/srv-dap3v694dm7c7390q930` | 1.556s |

All seven native processes exit0 after ordinary Ctrl+C dismissal; none times out or panics. Four independent current-build processes fail, two current-build controls pass, and the unmodified same-pin Render neighbor also loads metadata but opens the relative path. The private OS `open` stub records the actual child argument without launching a browser. No native HTTP traffic is intercepted; no authenticated browser page or deploy-menu action is claimed.

## Failing boundary and scoped fix

The pinned `pkg/client/client_gen.go:80–82` appends a trailing slash before constructing requests, so these two API inputs have the same native request base. `pkg/cfg/cfg.go:20–26` exposes the unnormalized override. Bex `lego/cli/internal/bridge/dashboard.go:22` compares it directly to `DefaultHost`, and `:26` makes the same raw comparison for a saved nonempty host. Each can return before the dashboard default is persisted. `lego/cli/main.go:65` invokes the helper for the correctly resolved consumer; `opensDashboard:249–258` includes services, its service alias and deploys list. The existing shared dashboard consumer then appends its resource path to the empty origin. No server or upstream request-builder fix is needed.

Normalize only the optional missing terminal slash for both production-default comparisons, matching the pinned client's existing normalization. Keep different/custom API bases, saved origins, malformed configs, non-consuming commands and help/version behavior intact. Keep the environment bearer out of persistence. Avoid broad URL rewriting or an upstream fork. Regression checks should cover missing/version-only configs with slashless `BEX_HOST`, equivalent explicit `RENDER_HOST`, a same-origin saved host, the slash-ended control, and an explicit dashboard origin. Verify both consuming command callers and all retained controls. The shared constructor's seven resource and five deploy link shapes retain the previously researched blast radius.

The eight existing `TestEnsureDashboardURL` subtests plus parent pass, but their host cases cover only the slash-ended default and a genuinely different custom host. They omit this accepted equivalent base. This is a targeted current-source check, not the full CLI suite or a fixed-source regression. The normal default-host current-build passes from sweep191 remain valid; this new case reopens the same item for implementation before release. Dedupe retains w8/090 rather than creating another item; w8/089 is the separate nonempty OAuth-origin redirect.

## Cleanup and evidence

All seven temporary config directories, native PTYs and the opener stub are removed. None persisted the environment grant. Hosting mutations and resource creation are zero; quota remains six baseline services and zero terminating/datastore resources. The ongoing loop retains its isolated human grant and private diagnostic build. Known credential/email values and private paths are redacted, including decoded/re-encoded raw terminal. Non-secret opener arguments remain exact. No product source edit, commit, push or release was performed.

- [existing-helper-events.jsonl](existing-helper-events.jsonl)
- [existing-helper-test.json](existing-helper-test.json)
- [head-missing-noslash.json](head-missing-noslash.json)
- [head-stored-noslash.json](head-stored-noslash.json)
- [head-version-noslash-control.json](head-version-noslash-control.json)
- [head-version-noslash-renderoverride.json](head-version-noslash-renderoverride.json)
- [head-version-noslash.json](head-version-noslash.json)
- [head-version.json](head-version.json)
- [metadata-cleanup.json](metadata-cleanup.json)
- [native-driver.py.txt](native-driver.py.txt)
- [opener-stub.py.txt](opener-stub.py.txt)
- [render-version-noslash.json](render-version-noslash.json)
- [source.txt](source.txt)
