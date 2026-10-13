# Current CLI native dashboard replay — sweep191

On 2026-10-13 UTC, a fresh private build of `c5a0eb9dd15f6b417bf586fdf26510706497b032` exercises the existing w8/090 launcher fix against production with the ongoing isolated human QA grant. No new bug or implementation is asserted.

| Native process | Opener argument | Duration |
| --- | --- | --- |
| head-version | `https://dashboard.bex.co/cron/srv-dap3v694dm7c7390q930` | 3.747s |
| head-missing | `https://dashboard.bex.co/cron/srv-dap3v694dm7c7390q930` | 1.592s |
| head-missing-alias | `https://dashboard.bex.co/cron/srv-dap3v694dm7c7390q930` | 1.225s |
| head-control | `https://dashboard.bex.co/cron/srv-dap3v694dm7c7390q930` | 1.456s |
| release-version | `/cron/srv-dap3v694dm7c7390q930` | 1.539s |
| render-version | `/cron/srv-dap3v694dm7c7390q930` | 1.482s |

Missing and version-only current-build configs, and the `service` alias, acquire the absolute Bex origin. The explicit-origin control retains it. Published Bex v0.3.2 and the unmodified same-pin Render client against Bex still pass a relative filesystem path to the opener. All six exit0 after ordinary Ctrl+C dismissal, with no timeout or panic. The environment bearer is absent from every persisted temporary config.

The actual native service-list palette reads baseline metadata and selects only Dashboard. A temporary `open` stub records its child argument; no browser is launched and no hosting resource is mutated. The shared pinned action list is sorted with Dashboard first. HTTP bytes are not intercepted; authenticated browser arrival, deploy-menu navigation, all twelve link shapes and a new released binary are not certified by this replay. w8/090 remains blocked on release and remaining acceptance.

Every temporary config directory, native PTY and the opener stub is removed. Baseline quota is six services, no terminating service, no Postgres and no Key Value. The ongoing loop retains its isolated human grant and private diagnostic build. Known credentials/email values and private paths are redacted; raw terminal is decoded, sanitized and re-encoded before archival.

Evidence:
- [build.json](build.json)
- [head-control.json](head-control.json)
- [head-missing-alias.json](head-missing-alias.json)
- [head-missing.json](head-missing.json)
- [head-version.json](head-version.json)
- [metadata-cleanup.json](metadata-cleanup.json)
- [native-driver.py.txt](native-driver.py.txt)
- [opener-stub.py.txt](opener-stub.py.txt)
- [release-version.json](release-version.json)
- [render-version.json](render-version.json)
