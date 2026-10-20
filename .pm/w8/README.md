# w8 — project workstream (worker8)

**Worker:** worker8. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Local dev environment

Develop against `.pm/w8/dev-8/`, this worker's own isolated stack on the shared local kind/CAPD cluster — never the shared cluster's default `auth`/`bex-system` namespaces or standard ports (5173/4433/4445/8090/8091/5432), which any other worker's session may also be using. `dev-8` gets its own Kratos + Hydra + Mailpit (namespace `dev-8-auth`) and app namespace (`dev-8`), reusing the shared cluster's CNPG operator and bex operator, plus a locally-built `bex-api` on dedicated ports derived from N=8 (`dev-8/ports.env`) so it never collides with any other workstream's `dev-N`.

- `bash scripts/dev-env.sh 8 up` — bring it up (idempotent — safe to re-run)
- `bash scripts/dev-env.sh 8 status` — health check (processes, pods, HTTP) + a pass/fail verification inventory
- `bash scripts/dev-env.sh 8 down` — tear it down (leaves the shared cluster and every other workstream's `dev-N` untouched)
- `bash scripts/dev-env.sh 8 clean` — reclaim `logs/` and `bin/` (refuses while the environment is up)

`up` prints the dashboard command to point at it once bex-api is running. One shared implementation serves every workstream since `w1/m72`; `.pm/w8/dev-8/` keeps only `ports.env` (a generated record of the derivation), this README, and `.gitignore`.

## Milestones

- [x] **m121** — [A release in flight builds its own source after a later save](done/m121/README.md) (8 tasks;370m) ← promoted from `126` on 2026-10-19 — **DONE (2026-10-19):** shipped `00a00c9d6`. A held release runs on its adopted spec's view (own build ref and inputs, image, pre-deploy, snapshots, launch). Live 13/13: a queued release builds its own commit past a newer deploy and past a branch Update Source, and a repo build finishes past a switch to a prebuilt image.

- [x] **m119** — [An overlapping deploy serves while recorded canceled; Restart then reverts to an older release](done/m119/README.md) (9 tasks;400m) — **DONE (2026-10-19):** shipped `69e925197`. Verified live 12/13; the Restart bullet (`undeployedChanges` stays true) passed after w8/m120.

- [x] **m120** — [A release held through rollout keeps its own settings](done/m120/README.md) (8 tasks;350m) ← w8/m119 live DoD 2026-10-19 — **DONE (2026-10-19):** shipped `608f76011`. Live: a held Docker release runs and records its own start command and env, Rollback restores them, and Restart keeps `undeployedChanges` true until a standard deploy applies the saved change.

- [ ] **m118** — [Redirect tenant HTTP to HTTPS](blocked/m118/README.md) (5 tasks;180m) ← promoted from `121` on 2026-10-18 — **BLOCKED (2026-10-18: release pipeline deploys the operator; QA runs t005 on production: `curl -sI http://<slug>.onbex.co/x?y=1` on a fresh owned web service → 301 `Location: https://<slug>.onbex.co/x?y=1`, HTTPS still 200; the same for a static site; a custom domain added after the deploy still issues its certificate over HTTP-01; then delete the fixtures)**. t001–t004 done: every App Ingress leads its middleware chain with an owned `<app>-https` RedirectScheme (permanent, Render parity), `BEX_HTTPS_REDIRECT=false` turns it off; kind check on pinned Traefik v3.7.5: HTTP→301/308 with path+query, HTTPS 200, ACME solver path unredirected.

- [ ] **m117** — [Canceling the newer of two open deploys strands the older](blocked/m117/README.md) (6 tasks;245m) — **BLOCKED (2026-10-18: release pipeline deploys bex-api and the operator; QA runs t006 on production: create a free web service, trigger a second deploy, `bex deploys cancel` it — the first deploy leaves `created`/`queued` within 5 minutes and goes `live` (URL serves 200) while the second stays `canceled`; repeat on an established service; then delete the fixtures)**. t001–t005 done: Cancel of the latest-pending deploy names the older in-flight deploy (`app.bex.co/surviving-release-generation`, saved commit restored to its row's); the operator carries that release through build and rollout and settles the cancel on it; every later cancel restates the survivor (suspend / cancel of the older one ends it, no resurrection); a cancel that leaves nothing running stops the older build and closes its row "Canceled along with dep-…" at once instead of at the ~35m gate timeout.

- [ ] **m116** — [Return backward log pages most-recent first](blocked/m116/README.md) (6 tasks;170m) — **BLOCKED (2026-10-18: release pipeline deploys bex-api and the dashboard (and the mobile app for its log view); QA runs t006: a fresh owned free web service with two planted markers — `bex logs --direction backward -o json` lists newest first, `--direction forward` oldest first, a cursor walk visits each once with no empty page after `hasMore`; then deletes the fixture)**. t001–t005 done: `queryLogPage` reverses backward pages after oldest-first selection/cursors; a one-line probe (plus one read past the page when a timestamp group swallows it) makes `hasMore` exact; an edge-emptied page resumes at the scan edge; dashboard history/deploy merges sort to the nanosecond; mobile reverses backward history; tests fail before / pass after.

- [x] **m115** — [Surface edge-blocked requests in request logs](done/m115/README.md) (6 tasks;180m) ← from `/qa-find-bugs-cli` sweep458 2026-10-18: an inbound-IP-allowlist 403 is enforced at the edge but never reaches the service's `type=request` history (Render: blocked requests do appear); shipper attribution drops middleware-rejected lines — **DONE (2026-10-18: hosted fail-before/pass-after; native Free allowlist403 visible in 27.3s; all readers + UI agree; fixtures absent)**

- [x] **m114** — [Move environment-group metadata from OpenBao to the control-plane store](done/m114/README.md) (12 tasks;590m) ← user decision 2026-10-18 after the m113 diagnosis; supersedes 119 — **DONE (2026-10-18: production cut over and pruned; pre-cutover snapshot comparison passed with metadata and values equal (run 39173043198); final image verified; migrator, cutover gate and temporary audit removed)**

- [x] **m113** — [Services list times out (524/502) for the bex workspace](done/m113/README.md) (6 tasks) ← from `/qa-find-bugs-cli` sweep 2026-10-18: `bex services` in workspace bex exceeds the edge timeout (110s→502, 125s→524) while small workspaces and sibling lists answer in ~1s — **DONE (2026-10-18: two human-device-login CLI runs 0.811/0.875 s; REST/GraphQL/MCP return the same 5 service IDs)**

- [ ] **m112** — [Make documented time-series tables usable in managed Postgres](blocked/m112/README.md) (8 tasks;300m) — **BLOCKED (2026-10-17: needs the user to decide how managed Postgres supplies Render's extensions — [decision](blocked/m92/design.md): operand image (CNPG PostGIS-only vs a bex-built image per major; no CNPG image carries TimescaleDB), activation (pre-install like vector vs on-demand allowlist) and a rolling restart of every managed Postgres pod, possibly with m65 — TimescaleDB also needs shared_preload_libraries, Apache mode only; no task started)**

- [ ] **m111** — [Make documented text similarity usable in managed Postgres](blocked/m111/README.md) (7 tasks;220m) — **BLOCKED (2026-10-17: needs the user to decide how managed Postgres supplies Render's extensions — [decision](blocked/m92/design.md): operand image (CNPG PostGIS-only vs a bex-built image per major), activation (pre-install like vector vs on-demand allowlist) and a rolling restart of every managed Postgres pod, possibly with m65; no task started)**

- [ ] **m110** — [Make incremental views usable in managed Postgres](blocked/m110/README.md) (7 tasks;285m) — **BLOCKED (2026-10-17: needs the user to decide how managed Postgres supplies Render's extensions — [decision](blocked/m92/design.md): operand image (CNPG PostGIS-only vs a bex-built image per major), activation (pre-install like vector vs on-demand allowlist) and a rolling restart of every managed Postgres pod, possibly with m65; no task started)**

- [ ] **m109** — [Make documented DuckDB analytics usable in managed Postgres](blocked/m109/README.md) (7 tasks;290m) — **BLOCKED (2026-10-17: needs the user to decide how managed Postgres supplies Render's extensions — [decision](blocked/m92/design.md): operand image (CNPG PostGIS-only vs a bex-built image per major), activation (pre-install like vector vs on-demand allowlist) and a rolling restart of every managed Postgres pod, possibly with m65; no task started)**

- [ ] **m108** — [Preserve ordinary uploads beyond the ingress body-read deadline](blocked/m108/README.md) (6 tasks;210m) — **BLOCKED (2026-10-17: needs the user to choose the public entrypoints' read budget — [decision](blocked/m108/design.md): Traefik v3.7.5's readTimeout also bounds headers and the pre-TLS peek, so Render's 100 minutes lets any client hold an edge connection for 100 minutes; options 100m, a bounded 10m (recommended), or an upstream readHeaderTimeout first; no task started)**

- [ ] **m107** — [Supply ZIP creation in native builds and runtimes](blocked/m107/README.md) (6 tasks;190m) — **BLOCKED (2026-10-17: needs the user to choose how native builds supply Render's native tools table — [decision](blocked/m91/design.md): A apt-get in every build (+242 s cold, ~+550 MB per app image), B relocatable tools image (fragile), C per-line bex native base images published by CI (recommended), or D documented divergence; no task started)**

- [x] **m106** — [Supply Hugo for the documented static quickstart](done/m106/README.md) (6 tasks;195m) — **DONE 2026-10-18:** Both native clients' fresh Free static sites built Render's Hugo quickstart unchanged; exact cleanup ([evidence](done/m106/evidence/hosted-2026-10-18/README.md)).

- [x] **m105** — [Make the Jekyll static quickstart build normally](done/m105/README.md) (7 tasks;260m) — **DONE 2026-10-18:** Both native clients' fresh Free static sites built Render's Jekyll quickstart unchanged; exact cleanup ([evidence](done/m105/evidence/hosted-2026-10-18/README.md)).

- [ ] **m104** — [Supply pg_partman for ordinary partition migrations](blocked/m104/README.md) (6 tasks;200m) — **BLOCKED (2026-10-17: needs the user to decide how managed Postgres supplies Render's extensions — [decision](blocked/m92/design.md): operand image (CNPG PostGIS-only vs a bex-built image per major), activation (pre-install like vector vs on-demand allowlist) and a rolling restart of every managed Postgres pod, possibly with m65; no task started)**

- [ ] **m103** — [Supply native Python packaging and development tools](blocked/m103/README.md) (8 tasks, 335m, 3 major omissions) — **BLOCKED (2026-10-17: needs the user to choose how native builds supply Render's native tools table — [decision](blocked/m91/design.md): A apt-get in every build (+242 s cold, ~+550 MB per app image), B relocatable tools image (fragile), C per-line bex native base images published by CI (recommended), or D documented divergence; no task started)**

- [ ] **m102** — [Supply pigz for native asset builds](blocked/m102/README.md) (6 tasks, 190m, major) — **BLOCKED (2026-10-17: needs the user to choose how native builds supply Render's native tools table — [decision](blocked/m91/design.md): A apt-get in every build (+242 s cold, ~+550 MB per app image), B relocatable tools image (fragile), C per-line bex native base images published by CI (recommended), or D documented divergence; no task started)**

- [ ] **m101** — [Supply Webpack for native production bundles](blocked/m101/README.md) (6 tasks, 225m, major) — **BLOCKED (2026-10-17: needs the user to choose how native builds supply Render's native tools table — [decision](blocked/m91/design.md): A apt-get in every build (+242 s cold, ~+550 MB per app image), B relocatable tools image (fragile), C per-line bex native base images published by CI (recommended), or D documented divergence; no task started)**

- [ ] **m100** — [Supply SWIG for native C-binding builds](blocked/m100/README.md) (6 tasks,200m,major) — **BLOCKED (2026-10-17: needs the user to choose how native builds supply Render's native tools table — [decision](blocked/m91/design.md): A apt-get in every build (+242 s cold, ~+550 MB per app image), B relocatable tools image (fragile), C per-line bex native base images published by CI (recommended), or D documented divergence; no task started)**

- [ ] **m99** — [Supply TypeScript for native builds and runtime](blocked/m99/README.md) (6 tasks,225m,major) — **BLOCKED (2026-10-17: needs the user to choose how native builds supply Render's native tools table — [decision](blocked/m91/design.md): A apt-get in every build (+242 s cold, ~+550 MB per app image), B relocatable tools image (fragile), C per-line bex native base images published by CI (recommended), or D documented divergence; no task started)**

- [ ] **m98** — [Supply jq and rsync for native asset builds](blocked/m98/README.md) (7 tasks,270m,2 major omissions) — **BLOCKED (2026-10-17: needs the user to choose how native builds supply Render's native tools table — [decision](blocked/m91/design.md): A apt-get in every build (+242 s cold, ~+550 MB per app image), B relocatable tools image (fragile), C per-line bex native base images published by CI (recommended), or D documented divergence; no task started)**

- [ ] **m97** — [Supply Ghostscript for native PDF rendering](blocked/m97/README.md) (6 tasks,215m,major) — **BLOCKED (2026-10-17: needs the user to choose how native builds supply Render's native tools table — [decision](blocked/m91/design.md): A apt-get in every build (+242 s cold, ~+550 MB per app image), B relocatable tools image (fragile), C per-line bex native base images published by CI (recommended), or D documented divergence; no task started)**

- [ ] **m96** — [Supply GNU gettext for native message compilation](blocked/m96/README.md) (6 tasks,195m,major) — **BLOCKED (2026-10-17: needs the user to choose how native builds supply Render's native tools table — [decision](blocked/m91/design.md): A apt-get in every build (+242 s cold, ~+550 MB per app image), B relocatable tools image (fragile), C per-line bex native base images published by CI (recommended), or D documented divergence; no task started)**

- [ ] **m95** — [Supply libvips in native builds and runtime](blocked/m95/README.md) (6 tasks,230m,major) — **BLOCKED (2026-10-17: needs the user to choose how native builds supply Render's native tools table — [decision](blocked/m91/design.md): A apt-get in every build (+242 s cold, ~+550 MB per app image), B relocatable tools image (fragile), C per-line bex native base images published by CI (recommended), or D documented divergence; no task started)**

- [ ] **m94** — [Supply SQLite CLI in native builds and runtime](blocked/m94/README.md) (6 tasks) — **BLOCKED (2026-10-17: needs the user to choose how native builds supply Render's native tools table — [decision](blocked/m91/design.md): A apt-get in every build (+242 s cold, ~+550 MB per app image), B relocatable tools image (fragile), C per-line bex native base images published by CI (recommended), or D documented divergence; no task started)**

- [ ] **m93** — [Supply Pandoc in native builds and runtime](blocked/m93/README.md) (6 tasks) — **BLOCKED (2026-10-17: needs the user to choose how native builds supply Render's native tools table — [decision](blocked/m91/design.md): A apt-get in every build (+242 s cold, ~+550 MB per app image), B relocatable tools image (fragile), C per-line bex native base images published by CI (recommended), or D documented divergence; no task started)**

- [ ] **m92** — [Make PostGIS usable in managed Postgres](blocked/m92/README.md) (7 tasks;250m) — **BLOCKED (2026-10-17: needs the user to decide how managed Postgres supplies Render's extensions — [decision](blocked/m92/design.md): operand image (CNPG PostGIS-only vs a bex-built image per major), activation (pre-install like vector vs on-demand allowlist) and a rolling restart of every managed Postgres pod, possibly with m65; no task started)**

- [ ] **m91** — [Supply PostgreSQL clients in native builds and runtime](blocked/m91/README.md) (6 tasks;205m) — **BLOCKED (2026-10-17: needs the user to choose how native builds supply Render's native tools table — [decision](blocked/m91/design.md): A apt-get in every build (+242 s cold, ~+550 MB per app image), B relocatable tools image (fragile), C per-line bex native base images published by CI (recommended), or D documented divergence; no task started)**

- [x] **m90** — [Install Yarn dependencies before static builds](done/m90/README.md) (6 tasks;140m) — **DONE 2026-10-18:** Both native clients' fresh Free static sites built with the bare dependency-using command and no installer prefix; exact cleanup ([evidence](done/m90/evidence/hosted-2026-10-18/README.md)).

- [x] **m89** — [Materialize pinned Git submodules before source builds](done/m89/README.md) (7 tasks) — **DONE 2026-10-17:** both original native first deployments Live, exact pinned file/source proofs and six HTTP200 responses without authored fetch; exact cleanup in sweep412.

- [x] **m88** — [Preserve graceful shutdown through native npm startup](done/m88/README.md) (8 tasks) — **DONE 2026-10-17:** original both-client npm restart replay passes; both old40s POSTs200,44 short reads200, native signal/drain logs and exact cleanup in sweep411.

- [ ] **m87** — [Make psql Ctrl+C stop external PostgreSQL queries](blocked/m87/README.md) (8 tasks,435m) — **BLOCKED (2026-10-16: t001 done — [design](blocked/m87/design.md): psql's legacy PQcancel carries only PID+key with no SNI, so it can be routed only if `pg-sni-proxy` terminates client TLS with each database's own CNPG serving pair to observe BackendKeyData; needs the user to accept that trust-boundary change (internet-facing proxy holds every tenant DB server key and sees plaintext; ADR009 passthrough revised; security re-review) or decide to keep passthrough and document modern-libpq-only cancellation)**

- [x] **m86** — [Install automatic npm dependencies for static builds](done/m86/README.md) (6 tasks) — **DONE 2026-10-16:** sweep379 accepts both original native first default deployments plus an explicit-skip/manual control; exact source/dependency/page hashes, nine shared readbacks and complete three-parent/child cleanup pass.

- [x] **m85** — [Honor committed Ruby lockfile and tool-version pins](done/m85/README.md) (6 tasks) — **DONE 2026-10-16:** sweep383 accepts both first original lock assertions on reviewed Ruby3.4.11 with refreshed Discourse source/lock hashes, Node24, six shared reads and exact two-parent/child cleanup.

- [ ] **m84** — [Make packaged pgvector usable in managed Postgres](blocked/m84/README.md) (6 tasks) — **BLOCKED:** t001–t005 done; fresh PG13/14/15/16/17/18 paths pass with both native migrations and real vector/HNSW/data controls, most recently [sweep378](evidence/postgres-native-cursors-sweep378/README.md). Exact fixture/credential cleanup passes. t006 still needs hosted pre-fix existing-parent backfill; actual serving revision is unverified.

- [ ] **m83** — [Supply automatic compression for web responses](blocked/m83/README.md) (6 tasks) — **BLOCKED (2026-10-16: needs the user to choose — (1) ship Traefik `compress` alone (streaming-safe, but `no-transform` responses get compressed and strong ETags stay strong, which this milestone rejects), (2) authorize an upstream traefik/yaegi `Flusher` wrapper (or Traefik `compress` option) PR first, or (3) keep identity; the guard-plugin design is correct on headers but Yaegi v0.16.1 drops `Flush`, buffering every stream on the route (0 s → 3 s first event). Implementation patch, neighbor matrix, flush timing and Brotli cost in [evidence](blocked/m83/evidence/edge-compression-attempt/README.md); nothing shipped, no task done)**

- [x] **m82** — [Supply FFmpeg in native Python builds and runtime](done/m82/README.md) (6 tasks) — **DONE 2026-10-16:** sweep382 accepts both first original MP3 builds and real runtime/probe/exact decode, live x86_64 plus local pinned aarch64, six shared reads and exact two-parent/child cleanup.

- [x] **m81** — [Provide uv for native Python lockfile projects](done/m81/README.md) (6 tasks) — **DONE 2026-10-16:** sweep381 accepts both first uv-requiring native deployments, default0.12.10/explicit0.11.3, real locked sync/run/project console/FastAPI HTTP200, six shared reads and exact two-parent/child cleanup.

- [x] **m80** — [Resolve omitted Git branches in Blueprint services](done/m80/README.md) (6 tasks) — **DONE 2026-10-16:** sweep388 accepts both first independent-default Git deploys with exact source/artifact/literals, delayed/no-op/canonical/six shared reads and exact2-metadata/2-service/6-child cleanup.

- [x] **m79** — [Supply automatic Brotli for static sites](done/m79/README.md) (5 tasks) — **DONE 2026-10-16:** two native Free sites serve12 actual compressed text/JS/CSS representations with exact decoded content,24 base/28 neighbor HTTP checks and17 maintained serving tests pass in sweep340; all owned parents/children cleaned.

- [x] **m78** — [Respect per-service Git sources in persistent Blueprints](done/m78/README.md) (6 tasks) — **DONE 2026-10-16:** fresh persistent/inline first Git source/commit/package/literal acceptance, measured215-second convergence and72-second no-op stability pass in343; six validations/eight native/six shared readbacks and17 tests/55 named events pass. Both parents/five children/one Blueprint cleaned.

- [x] **m77** — [Supply JavaScript tools in native Ruby and Python builds and runtime](done/m77/README.md) (6 tasks) — **DONE 2026-10-16:** both first native Ruby journeys pass in342, completing341 Python acceptance and actual all-Ruby-line/Python3.12 generated-image execution. Required tools/primary markers/shared reads and all exact-owned cleanup pass.

- [x] **m76** — [Honor a GitHub repository's default branch on create](done/m76/README.md) (6 tasks) — **DONE 2026-10-16:** fresh unchanged native first-create/default-source and final build/runtime acceptance, shared reads/export and source execution pass in sweep338; two parents/four children cleaned. Bex timing-probe qualifications retained.

- [x] **m75** — [Validate Blueprints with an implicit repository](done/m75/README.md) (6 tasks) — **DONE 2026-10-15:** implementationa1f1d7288; nine qualified implicit files, all36 name controls,80 native calls, eight native-wire replays and34 GraphQL/MCP comparisons pass. Legitimate disk/source refusals remain; no allocations.

- [x] **m74** — [Preserve Blueprint host-reference origins and target checks](done/m74/README.md) (7 tasks) — **DONE 2026-10-16:** all17 original documents/51 native calls, eight unchanged-wire replays,34 shared API comparisons and nine maintained tests/80 named events pass in sweep339. Two Free targets/two deploy children cleaned; baseline/quota retained.

- [x] **m73** — [Supply pnpm in native Node builds and runtime](done/m73/README.md) (6 tasks) — **DONE 2026-10-16:** fresh unchanged native first-create/default-source and final build/runtime acceptance, shared reads/export and source execution pass in sweep338; two parents/four children cleaned. Bex timing-probe qualifications retained.

- [x] **m72** — [Provide Poetry in native Python builds and runtime](done/m72/README.md) (7 tasks) — **DONE 2026-10-16:** both native first Poetry1.8.5/JavaScript/Python3.12 journeys and an omitted-setting default2.1.3 control pass in sweep341, with isolation/shared/real-image checks and all three parents/children cleaned. m77 Ruby hosting accepted in sweep342.

- [x] **m71** — [Make explicit Bun opt-in usable in native builds and runtime](done/m71/README.md) (7 tasks,3h20m) — **DONE:** sweep327 accepts both native first Bun deployments Live/HTTP200 with build/runtime Bun1.3.14 for requested1.3.4, a fresh no-opt-in Node control, current source/generated-image checks and complete three-parent/child cleanup.

- [x] **m70** — [Make Blueprint source switches deploy their declared artifact](done/m70/README.md) (6 tasks) — **DONE 2026-10-15:** source-coupling fix eebd8ce27; first inline and persistent Git artifacts/no-ops, three native clients and REST/GraphQL/MCP readbacks pass with complete cleanup. Different containing/service sources remain [m78](done/m78/README.md).

- [ ] **m69** — [Preserve native cron startup semantics](blocked/m69/README.md) (6 tasks, 2h50m) — **BLOCKED (2026-10-15: t001, t002, t004, t005 done — a native cron's CLI `startCommand` now stays its baked Bash start command on create, update (REST/GraphQL/MCP), Blueprint edits incl. a docker→native switch, readback/clone and the dashboard Command row; explicit Bex `command` stays a /bin/sh override; legacy duplicates repair on their next command write. Needs the backend + dashboard deployed, then the hosted two-client native cron create on an explicitly approved paid fixture + finite manual run with the exact marker, update/readback/clone checks and owned cleanup (t003/t006))** Sweep302:24 focused source passes; both fresh live runs still fail in /bin/sh, native logs confirm; all own fixtures cleaned.

- [ ] **m68** — [Preserve executable command text](blocked/m68/README.md) (7 tasks, 3h15m) — **BLOCKED (2026-10-15: t001–t003, t005, t006 done — backend and dashboard keep nonblank build/start/pre-deploy/cron/hook command text byte for byte (whitespace-only still clears) with failing-first REST/GraphQL/MCP/Blueprint/dashboard regressions; upstream CLI patch + request-construction regressions prepared in `blocked/m68/evidence/t001-upstream/upstream-command-text.patch`. Needs the user to authorize external submission to render-oss/cli after a fresh dedupe, then upstream acceptance/release, a Bex CLI pin/release and the deployed fix for the hosted two-client `7461696c20` replay with cleanup (t004/t007))** Sweep299:41 focused source passes; live API pre-deploy still executes the shortened argument, both native readers confirm; own fixture cleaned.

- [ ] **m67** — [Keep native live logs running after a transient disconnect](blocked/m67/README.md) — **BLOCKED (2026-10-14: t001 done — Bex built on v2.29.0 reconnects and resumes A/B exactly once, but v2.29 also retries normal 1000 closes, which Bex uses to end a no-running-build tail (w8/020), so `logs --type build --tail` would loop forever. Needs the user to choose a non-retryable refusal close code (non-zero exit), accepting the retry, or waiting for upstream; then t002–t006 and an explicitly requested CLI release)**

- [x] **m66** — [Keep Blueprint service updates in canonical intent](done/m66/README.md) (6 tasks) — **DONE 2026-10-15:** canonical-intent fix62c74aae6; inline sweep288 and persistent sweep330 pass first-artifact/runtime/source, three native waited controls, delayed no-ops, source checks and complete cleanup. Different containing/service sources remain [m78](done/m78/README.md).

- [ ] **m65** — [Retain PostgreSQL process logs during bursts](blocked/m65/README.md) — **BLOCKED (2026-10-13: t001 done — the fix is upstream CNPG #11131 in operator 1.30.1 / chart 0.29.1, we run 1.30.0; needs the user to approve a CNPG upgrade rollout that restarts every managed Postgres pod (control plane + tenant Free single-instance DBs), or decide to enable in-place instance-manager updates first; then the credentialed chart mirror, live 300/1005-record burst acceptance and cleanup)**

- [ ] **m64** — [Keep Blueprint environment copies consistent across applies](blocked/m64/README.md) — **BLOCKED (implementation shipped; sweep231 accepts observed live group, own-store refusal and literal/seed paths, with ten native wire captures and complete fixture cleanup; t006 still needs remaining variants, shared live/snapshot surfaces and the complete inherited DoD)**

- [ ] **m63** — [Keep completed initial hooks out of later deploys](blocked/m63/README.md) — **BLOCKED (2026-10-13: t001–t005 done — the hook has its own spec field, the operator completes it on success and later releases run only the regular command; needs the operator/API deployed to production plus a hosted owned-fixture run with native Bex/Render manual deploys and cleanup before t006)**

- [ ] **m62** — [Reject unlaunchable environment values](blocked/m62/README.md) — **BLOCKED:** sweeps295–296 pass72 native and36 GraphQL/MCP validator calls plus34 focused named check events; sweeps297–298 add38 direct-writer passes and hosted binary-file preservation;109 bulk routing and dashboard feedback retain t003/t006 gates. The original MCP save error is unassigned and not reproduced by seven fresh controls. No production config applied.

- [x] **m61** — [Honor application PATH during native builds](done/m61/README.md) (8 tasks,230m) — **DONE 2026-10-15:** prior native helper/image fixes plus sweep289 four-case pre-deploy live acceptance and complete cleanup; direct output and surface limits recorded.

- [ ] **m60** — [Preserve accepted arguments when copying CLI commands](blocked/m60/README.md) — **BLOCKED (2026-10-12: t001/t002 prepared and tested with `blocked/m60/evidence/upstream-copy-command-formatter.patch` + caller audit; needs the user to authorize external submission to render-oss/cli after a fresh dedupe, then upstream acceptance/release, a Bex CLI pin/release, and native Bex/Render receipts; no fork/backend workaround per scope/DO_NOT_DO)**

- [x] **m58** — [Preserve Unicode matches in log history](done/m58/README.md) — **DONE 2026-10-13** (fixes t001 + t007 `8fb0afa1a`; live App/saturated/KV/PG receipts, REST/GraphQL/MCP agreement; live GraphQL/dashboard and saturated managed-data recorded as boundaries)
- [ ] **m59** — [Return static sites to direct publication after clearing their build command](blocked/m59/README.md) — **BLOCKED (2026-10-13: t001/t002/t004 done; native build-created clears live-accepted in sweep184 and REST/GraphQL/MCP code parity tested; GraphQL/MCP writes live-accepted in sweep257; needs dashboard writes (t003) and legacy/strategy-neighbor acceptance (t005) on owned fixtures, then t006)**

- [ ] **m57** — [Preserve literal string lists in interactive CLI forms](blocked/m57/README.md) — **BLOCKED (2026-10-12: t001/t002 prepared and tested with `blocked/m57/evidence/upstream-lossless-string-list.patch` + caller audit; needs the user to authorize external submission to render-oss/cli after a fresh dedupe, then upstream acceptance/release, a Bex CLI pin/release, and native Bex/Render receipts; no fork/backend workaround per scope/DO_NOT_DO)**

- [x] **m56** — [Publish complete service settings before rolling a release](done/m56/README.md) (7 tasks; 4h15m) ← functional CLI QA, 2026-10-10; both clients execute a cleared pre-deploy command during a successful compound save.

- [x] **m55** — [Preserve literal commas in service name filters](done/m55/README.md) (7 tasks; 3h45m) ← functional CLI QA, 2026-10-09; both clients fail named update/clone, actual pinned query encoding and runtime controls recorded. — **DONE 2026-10-10** (server fix `1899e517d`; live two-client replay passed)

- [x] **m54** — [Accept one-sided CLI build filters](done/m54/README.md) (7 tasks; 2h20m) ← live CLI QA + exact-source gate probe, 2026-10-09 UTC; upstream serializer/schema mismatch documented. — **DONE 2026-10-09**

- [x] **m53** — [Accept numeric Blueprint environment values](done/m53/README.md) (7 tasks; 2h30m) ← live CLI QA + exact-source typed-decoder probe, 2026-10-09 UTC; source fix landed; production acceptance recorded. — **DONE 2026-10-09**

- [x] **m52** — [Keep a new service out of stale orphan cleanup](done/m52/README.md) (7 tasks; 3h40m) ← live CLI QA + deterministic source probe, 2026-10-08; exact live deletion actor unobserved. — **DONE 2026-10-09**

- [x] **m51** — [Native builds honor the requested runtime version (Render's PYTHON_VERSION / NODE_VERSION / version-file contract)](done/m51/README.md) (9 tasks) — **DONE 2026-10-08**: `.python-version`/`.nvmrc` live builds pass on `bex v0.3.2` (`examples/runtime-version`); intermittent narration drop filed as w8/069.
- [x] **m50** — [Enforce Postgres tier capacity across write paths](done/m50/README.md) (9 tasks) ← promoted from `035` on 2026-10-02.

- [x] **m49** — [Reject invalid image deploy inputs with actionable errors](done/m49/README.md) (7 tasks) ← promoted from `033` on 2026-10-02. Completed 2026-10-02; full suites/lint and 57 live checks passed.

- [x] **m48** — [Close the log filtering and Blueprint validation gaps](done/m48/README.md) (6 tasks) ← from `032`; completed 2026-10-02 with full backend/lint checks, five pinned Alloy cases, and 50 live local acceptance checks.

- [x] **m47** — [A renamed service keeps answering to its old name: rename skips the uniqueness rule, and name lookup ignores the name you see](done/m47/README.md) (7 tasks) **DONE 2026-10-02**. Live on production with `bex v0.2.1` in `bex-canary`, which is not the account default: rename-to-taken returns 409, concurrent renames produce one winner, and renamed `deploys list`/`services instances`/by-name restart act on the renamed service. Creation names stay reserved, so the swap is impossible. The pinned `restart` takes ids only, so by-name restart was verified on the server route.
- [x] **m46** — [Concurrent deploy ordering and image attribution](done/m46/README.md) (8 tasks) — **DONE 2026-10-02**: 30 qualifying live rounds passed (10 image/image, 10 hook/hook, 10 image/restart); every latest request live, earlier canceled, correct per-row images.
- [x] **m45** — [A service name resolves in the caller's default workspace, not the selected one, so name-based verbs can act on another workspace's service](done/m45/README.md) (8 tasks) (8 tasks) — **DONE 2026-10-08**: live two-workspace closeout on `bex v0.3.2` — selection wins for names (reads, instances, deploy), no selection → 409, by-id cross-workspace reads unchanged.
- [x] **m44** — [An unpullable image hides behind a 10–18 minute timeout and a "check the logs" message](done/m44/README.md) (7 tasks) **DONE 2026-10-03**. Live on production with `bex v0.2.1` in `bex-canary`: a nonexistent-tag override fails in 100 s without pre-deploy and 110 s with it, each with `image pull is failing`, and the prior release keeps serving. The next config change deploys the saved image, including after a **canceled** override (verified live 2026-10-03, `t007`). A failing health check over a prior release closes at ~15 min with the probe diagnosis. Adjacent finding, not yet filed: a hibernated free service whose rollout fails stays `503 service hibernated` after waking (the `update_failed` sibling of w1/m156).
- [x] **m43** — [Postgres HA plan enforcement](done/m43/README.md) (8 tasks) — **DONE 2026-10-02**: user accepted no HA in the current catalog; create/update free-plan refusals verified live, HA remained off, production audit 0/3 HA databases, fixture deleted.
- [x] **m42** — [Unstick production deploys: fix the flaky gates holding `deploy.yml` red since 2026-09-22](done/m42/README.md) (7 tasks) — **DONE 2026-10-07**: three consecutive successful deploy.yml runs (`8c2ec3b4d`, `fc4bbbd38`, `cd65ad10a`), gate dedup live (`faaaba0ce`), flakes root-caused (t001–t003), red-streak detector fixed (t004).

- [x] **m39** — [Complete Blueprint execution fencing](done/m39/README.md) (8 tasks; 3h20m implementation, 5h20m total) ← approved 2026-09-09 pm-brainstorm proposal 1 — done 2026-09-10
- [x] **m40** — [Make Blueprint ownership reliable during apply](done/m40/README.md) (8 tasks; 3h20m implementation, 5h20m total) ← approved 2026-09-09 pm-brainstorm proposal 2 — done 2026-09-11
- [x] **m41** — [Bind manual Blueprint sync to its reviewed source](done/m41/README.md) (8 tasks; 2h50m implementation, 4h50m total) ← approved 2026-09-09 pm-brainstorm proposal 3 — done 2026-09-11

**Execution order:** m39 → m40 → m41 done; m38 unblocked (depends on verified m39+m40 closeouts).

- [x] **m36** — [Blueprint Git source integrity](done/m36/README.md) (8 tasks; 3h15m implementation, 5h total) ← approved 2026-09-08 pm-brainstorm proposal 1 — done 2026-09-08
- [x] **m37** — [Blueprint lifecycle consistency](done/m37/README.md) (9 tasks; 4h30m implementation, 6h30m total) ← approved 2026-09-08 pm-brainstorm proposal 2 — done 2026-09-08 (dev-8 two-client walkthrough outstanding: Docker Desktop VM crash-loops headlessly; see done/m37/README.md evidence)
- [x] **m38** — [Resource-independent Blueprint auto-sync](done/m38/README.md) (9 tasks; 4h implementation, 6h total) ← approved 2026-09-08 pm-brainstorm proposal 3 — done 2026-09-11

- [x] **m1** — Metering pipeline: hourly usage rollups into the control-plane store (9 tasks) ← from `/pm-brainstorm w8` 2026-07-09
- [x] **m2** — Usage API: month-to-date usage over REST · GraphQL · MCP (9 tasks) ← from `/pm-brainstorm w8` 2026-07-09, needs m1
- [x] **m3** — Dashboard Usage page (workspace-scoped, Render-consistent) (8 tasks) ← from `/pm-brainstorm w8` 2026-07-09, needs m2
- [x] **m4** — Usage data retention: compact hourly detail into monthly aggregates (9 tasks) ← from `/pm-brainstorm think of new milestones for w8` 2026-07-09
- [x] **m5** — Meter managed Postgres & Key Value instance-seconds (9 tasks) ← from `/pm-brainstorm more milestones for w8` 2026-07-10
- [x] **m6** — Usage history: GraphQL period support + dashboard multi-month view (9 tasks) ← from `/pm-brainstorm more milestones for w8` 2026-07-10, needs m2
- [x] **m7** — Price sheet + estimated spend (Render-equivalent billing) (12 tasks) ← from `/pm-brainstorm for more` 2026-07-13 (user request fires `.pm/FUTURE-MAYBE.md`'s "Pricing & spend estimation" trigger; 30% off Render's compute/Postgres/KeyValue/build-minute prices, 90% off bandwidth; estimate-only, no payment collection — user-confirmed scope boundary)
- [x] **m8** — Service display name: rename without breaking the immutable resource id (8 tasks) ← from `/pm-brainstorm more` 2026-07-13 (`docs/ADR018-render-parity.md` "Change instance plan / type" row note — `name` PATCH field not editable). Originally proposed under `w2`, materialized under `w8` per user direction
- [x] **m9** — Meter managed Postgres & Key Value storage separately from compute (9 tasks) ← promotes `002` 2026-07-13 (Render drift follow-up from m5, named directly in `docs/ADR018-render-parity.md`'s usage-metering row)
- [x] **m11** — Reliable usage windows: durable zeroes + gap-free per-meter cursors (7 tasks) ← prerequisite split from `001` 2026-07-14; cap enforcement stays gated — done 2026-07-14
- [x] **m12** — Managed Postgres major-version upgrade (9 tasks) ← from `/pm-brainstorm more milestones for each worker` round 2, 2026-07-14 (`database_types.go:35` — `Version` exists at create, no upgrade verb anywhere; Render ships version upgrades as a first-class flow; rides CNPG's declarative major-upgrade path, verified first by t002). Placed under w8 for capacity per the m8 precedent; numbered m12 not m11 — a concurrent session claimed m11 mid-rebase — done 2026-07-15
- [x] **m13** — Datastore list pagination: Postgres + Key Value (7 tasks) ← from `/pm-brainstorm more milestones for each worker` round 3, 2026-07-14 (`core.PageParams` in `apps/rest.go:309` but nowhere in `postgres/rest.go`/`keyvalue/rest.go`; Render's `GET /postgres` + `GET /key-value` both page); datastore-family placement per the m12 precedent
- [x] **m14** — Postgres disk autoscaling (8 tasks) ← from `/pm-brainstorm` round 7, 2026-07-14 (systematic field-diff: `enableDiskAutoscaling`/`diskAutoscalingEnabled`, zero hits; the control loop between grow-only `storageGB` and w3/m10's already-scraped kubelet volume stats) — done 2026-07-15
- [x] **m15** — Complete outbound-bandwidth accounting: HTTP + WebSocket + direct + datastore TCP (14 tasks) ← prerequisite split from `001` 2026-07-14; replaces the HTTP-only counter before bandwidth caps can be promoted
- [x] **m16** — Managed-datastore polish chores: create ipAllowList parity + pg_stat_statements backfill + KV version assessment (8 tasks; t008 added by round 19 — Render's inline PATCH `parameterOverrides` silently ignored) ← groups round-14 consistency find (postgres create `ipAllowList` is REST-only while keyvalue has all three surfaces) with notes `003` + `006`, 2026-07-15 (the w7/m37 chores pattern); coordinates with w4/m24 (descriptions) and w9/m38 (error bodies)
- [x] **m17** — Disk-autoscaling hardening: loud sample-failure signal + single-sourced 16 TB cap (6 tasks) ← from `/pm-brainstorm` round 16, 2026-07-15 (consistency-mine over the day-old m14 diff: `database_autoscale.go:176-179` swallows sample-unavailable at Info level and silently no-ops — the feature's whole purpose defeated with no Event/condition; the 16 TB cap literal duplicated across operator, dashboard, MCP description, and docs) — done 2026-07-16
- [x] **m18** — Blueprint estimated pricing panel (Render blueprint/new parity) (8 tasks) ← from deep-research session 2026-08-04 (bex `blueprints/new` review shows names only vs Render's Estimated pricing panel; server-side estimate from the m7 price sheet through the ADR049 preview payload)
- [x] **m19** — Blueprint spec-parity round 2: registry sweep, custom paths, schema drift (11 tasks) ← from blueprint Render-parity review 2026-08-15 (the m63 audit-baseline `unsupported` set never swept: static `buildCommand`/`dockerContext`/`registryCredential` block real-world yamls; Render's 2026-02-09 custom Blueprint paths; schema pin aging; ADR018 Blueprint row ◐ pending CLI evidence)
- [x] **m20** — Blueprint grouping hardening: transactional writes, quota, audit, disconnect reclaim (8 tasks) ← promotes `w1/049` #5 residuals 2026-08-16 (security-scan round 7 deferred register; fix-shape precedent `w7/done/m72/t004`); sequenced after m19 in the same workstream since both touch `blueprint.go`
- [x] **m21** — Blueprint dashboard completion: sync:false prompts, pre-sync diff, settings edit (7 tasks) ← from blueprint lifecycle-semantics verification 2026-08-16 (backend `envVarValues` channel complete but the dashboard's `CreateBlueprint` mutation lacks the variable, so dashboard-created blueprints deploy secret placeholders empty; detail-page Sync applies blindly; name/path PATCHable but not editable in UI)
- [x] **m22** — Generate Blueprint: export existing resources as render.yaml (7 tasks) ← from blueprint lifecycle-semantics verification 2026-08-16 (Render's select-services→download render.yaml has no bex equivalent; the m19 registry allowlist + verified adoption-by-name make it a serialization exercise with a no-op round trip)
- [x] **m23** — Blueprint resource ownership: stop silent cross-blueprint overwrite (6 tasks) ← from blueprint lifecycle-semantics verification 2026-08-16 (no ownership marker — two blueprints naming the same resource silently clobber each other; Render warns "last sync wins", bex refuses with takeover confirmation); best sequenced after m21/m22, which reuse the same review/confirmation surfaces
- [x] **m24** — Webhook event hydration: retrieve a payload's `data.id` (9 tasks) ← from authenticated Bex↔Render webhook parity audit 2026-08-17; closes the event-detail residual after w2/m70
- [x] **m25** — Webhook immutable attempt history + manual Resend (13 tasks) ← from authenticated Bex↔Render webhook parity audit 2026-08-17, needs m24
- [x] **m26** — Webhook management UX + drift-proof dashboard parity (10 tasks) ← from authenticated Bex↔Render webhook parity audit 2026-08-17, needs m25
- [x] **m27** — Granular OAuth capability scopes and authorization-decision audit (9 tasks) ← proposal 1 from `/pm-brainstorm for w8`, selected by user 2026-08-18
- [x] **m28** — Polish `/agents` as a prompt-first workspace (8 tasks) ← designer review of `dashboard.bex.co/agents` 2026-08-18, user handoff to w8
- [x] **m29** — bex CLI help chrome: strip Render branding without forking (7 tasks) ← from CLI branding research handoff 2026-08-19; Layer-1 `RootCmd` overlay only — DO_NOT_DO no-fork honored
- [x] **m30** — Restore paid KeyValue backup protection (9 tasks) ← from the 2026-08-25 backup verification (`docs/drills/2026-08-25-backup-verification.md`), user handoff to w8 2026-08-25; supersedes `w5/049`
- [x] **m31** — In-place GitHub "Credentials" menu on the service source picker (Render /web/new parity) (6 tasks) ← from `/pm-brainstorm` 2026-08-28, user handoff to w8; UI-only composition over the shipped multi-account surface (`w5/m74`/ADR075) — adds Render's `/web/new` connect/disconnect/manage-in-place credentials dropdown to the `ServiceSourcePicker` GitHub tab; backend unchanged. Done 2026-09-03 (t001–t005 code/docs/tests 2026-08-29; t006 live DoD on `dashboard.bex.co`: Credentials (1) for `bex-co` with 73 repos + Open/Configure/Disconnect/Connect-another/Claim on `/services/new` and `/blueprints/new`), moved to `done/m31/`
- [x] **m32** — Static-site create wizard offers an invalid Docker-image source — gate source by service type (UI + API + CRD) (8 tasks) ← from live QA hunt `/qa-find-bugs for w8` 2026-09-03 (`/services/new?type=static_site` offers an "Existing Image" source and enables Deploy for `static_site`+`image`+`publishPath`; two-layer gap — UI `services.new.tsx:135-142` + backend `service.go:2395-2403` + CRD `app_types.go`; adjacency to m31's `ServiceSourcePicker` work)
- [x] **m33** — Upstream Render drift re-baseline round 3 + CLI pin v2.26.0 (7 tasks) ← from `/pm-brainstorm for w8` 2026-09-07 #1, approved same day (w2/m79 precedent, last run 2026-08-19; upstream `render-oss/cli` shipped v2.25.0 + v2.26.0 past the pinned v2.24.0, and the checklist's pin-conditional bullets are due a re-check). Done 2026-09-07: pin v2.24.0 → v2.26.0 (Go 1.27), launcher disables upstream analytics-by-default, ADR018 + CLI checklist round-3 re-baseline, spec-based plan-name divergence filed `011.md`; moved to `done/m33/`
- [x] **m34** — Disk event wire vocabulary: rename to Render's `disk_created`/`disk_deleted` (8 tasks) ← from `/pm-brainstorm for w8` 2026-09-07 #2, approved same day, ratifying `w6/068` option 1 (note retired to `w6/done/068.md`; two of bex's four emitted disk event types are unselectable through the Render-shaped `?type=` filter) — done 2026-09-08
- [x] **m35** — Datastore deleting-read parity: fold Postgres/Key Value onto consistent-absence (8 tasks) ← from `/pm-brainstorm for w8` 2026-09-07 #3, approved same day, promoting `w3/034` option 1 (note retired to `w3/done/034.md`; the same list-drops-but-by-id-shows-deleting incoherence w3/m81 fixed for services, standing on the two datastore families)

## QA evidence for existing work

- [Native chunked/gzip uploads](evidence/native-chunked-gzip-uploads-sweep400/README.md) — sweep400:ten exact small-body uploads across both first native apps, paced/chunked/fixed-gzip/public-H2 controls and ten native log correlations pass. Two parents/children/URLs cleaned; larger396 candidate remains inconclusive. No new issue.

- [Native large Key Value pipe imports](evidence/native-keyvalue-large-pipe-sweep398/README.md) — sweep398:four accepted1MiB imports, success/type-error counts and later writes,12 exact values verified by the opposite client. Original short-wait partial attempt and prelaunch setup retained;15 exact key names/cache/credentials cleaned, history metadata unchanged. No new issue or085 closeout.

- [Native5MiB downloads and bounded upload observations](evidence/native-file-transfers-sweep396/README.md) — sweep396:both first native fixtures serve exact full/range/header/conditional controls (16 passes); three smaller uploads match native logs. Five5MiB upload attempts time out or499near61s, with variable observed progress; cause/compatibility unconfirmed, no new issue. Exact cleanup passes.

- **New major [117](done/117.md)** — native Ruby silently rewrites stale dependency locks; both first deployments serve the changed gem, build-only deployment-mode controls correctly refuse, prior boots remain serving and exact cleanup passes ([sweep394 evidence](evidence/117/README.md)).

- [Native Poetry installed application environments](evidence/native-poetry-project-sweep392/README.md) — sweep392:default2.1.3/explicit1.8.5 real dependency installs, generated lock and isolated project interpreter persist through build/pre-deploy/runtime; both first children serve original Flask HTTP200,8 shared/env reads and exact cleanup pass. No new issue.

- [Native PostgreSQL18 locking and deadlock recovery](evidence/postgres-native-locking-sweep391/README.md) — sweep391:45 native stages pass for NOWAIT/SKIP LOCKED, observed two-client deadlock with different local/live victims, session/transaction/shared advisory semantics and natural-exit release; exact owned cleanup, no new issue.

- [Native Gunicorn custom/empty and CLI precedence](evidence/native-python-gunicorn-overrides-sweep390/README.md) — sweep390:both first three-phase values and start-only CLI controls pass with original HTTP/log proof,16 shared/env reads and exact cleanup; updates113, which stays blocked on remaining variants.

- [Native Key Value RESP3 and cache invalidation](evidence/native-keyvalue-resp3-sweep389/README.md) — sweep389:74 command checks and20 push/quiet assertions pass across both clients, with typed replies, NOLOOP/OPTIN/own-prefix tracking, exact own cleanup and unchanged history; no new issue.

- [Blueprint omitted independent default branch](evidence/blueprint-default-branch-sweep388/README.md) — sweep388:both first Git children select independent master/current commit with exact artifacts, delayed no-op/canonical/shared proof and cleanup. Closes m80; API sync stays labeled separately from14 native validations.

- [Native PostgreSQL18 extended-query pipelines](evidence/postgres-native-pipelines-sweep387/README.md) — sweep387:both native sessions pass17 exact frames each, typed/empty bound values, named Parse/Bind/Close, partial reads/Sync, expected errors and rollback/recovery on stable distinct backends. Exact parent/endpoint/credential cleanup passes; no new issue.

- [Native Elixir original production pipeline](evidence/native-elixir-production-sweep385/README.md) — sweep385:both first unprefixed production-dependency/compiler builds and runtime Mix/prod artifacts pass on exact original source. Eight native logs,6 shared reads and cleanup pass; closes112, no new issue.

- [Native Node environment phases and explicit values](evidence/native-node-environment-phases-sweep384/README.md) — sweep384:both first native defaults, pre-deploy and staging/empty redeploys pass with exact source/render/cache proof and cleanup. Updates111; paid cron stays untested, no new issue.

- [Native Ruby committed-lock first deployments](evidence/native-ruby-lock-selection-sweep383/README.md) — sweep383:both first original assertions select reviewed Ruby3.4.11 over the broader Gemfile range; exact source/markers/Node24,6 shared reads and cleanup pass. Closes m85; no new issue.

- [Native Pydub MP3 build/runtime conversion](evidence/native-media-conversion-sweep382/README.md) — sweep382:both first native builds and runtime conversions pass six ffprobe/twelve exact PCM checks, source/artifact retention and6 shared reads; live x86_64/local aarch64 and all cleanup pass. Closes m82; setup qualifications retained.

- [Native uv locked-project first deployments](evidence/native-uv-project-sweep381/README.md) — sweep381:both native first default/explicit versions pass actual locked sync, installed project console, project .venv and upstream FastAPI HTTP200;6 shared reads and exact cleanup pass. Closes m81; no new issue.

- [Static npm automatic dependencies](evidence/static-auto-deps-sweep379/README.md) — sweep379:both native original first builds and an explicit-skip/manual control reach Live/HTTP200 with exact source/dependency/page hashes;9 shared reads and complete cleanup pass. Closes m86; no new issue.

- [Native PostgreSQL17 cursors and prepared statements](evidence/postgres-native-cursors-sweep378/README.md) — sweep378:32 stages pass across two stable native backend PIDs, held snapshots/positions, portal lifecycle, expected errors and prepared execution;15 vector controls and exact cleanup pass. Fresh m84 majors13–18 accepted; existing-parent backfill stays blocked.

- [Native PostgreSQL16 binary COPY](evidence/postgres-native-binary-copy-sweep377/README.md) — sweep377:fifteen data commands/27 controls pass for64 exact typed rows, duplicate-key atomicity and two byte-identical binary exports;15 vector neighbor controls and exact cleanup also pass. No new issue; m84 backfill gate remains blocked.

- [Native PostgreSQL15 isolation and error recovery](evidence/postgres-native-isolation-sweep376/README.md) — sweep376:27 stages pass across stable native backend PIDs, real snapshot/lock/savepoint/serialization outcomes and independent data reads;15 vector neighbor controls and exact endpoint/credential cleanup also pass. No new issue; m84 backfill gate remains blocked.

- [Native two-client Key Value WATCH transactions](evidence/native-keyvalue-watch-sweep375/README.md) — sweep375:30 native stages pass with distinct stable client IDs, conflict/expiry aborts, ordered commit, UNWATCH/DISCARD, exact key deletion and native exits; full resource/credential cleanup, no new issue. Local history setup mistake is corrected and documented.

- [Native static encoded asset names](evidence/static-encoded-assets-sweep374/README.md) — sweep374:twelve exact named files and30 HTTP2/HTTP1.1 data, HEAD, conditional/range/negative controls pass; own first deployment/parent/public route cleanup verified. No new issue.

- [Native PostgreSQL14 large-object files](evidence/postgres-large-objects-sweep373/README.md) — sweep373:both native imports, five exact exports including empty, committed byte patches, atomic rollback and four expected errors pass;28 native commands/58 controls, exact object/file/database/credential cleanup. Earlier harness deadlines and read-only prelaunch recovery are retained, no new issue.

- [Incoming-only WebSocket activity, idle and wake](evidence/115/README.md) — sweep372:18 exact messages over180seconds with no received application data/pings, native log match, eventual quiet hibernation and normal HTTP wake; exact cleanup. Minor descriptor drift is115, no new idle algorithm defect.

- [Native web-service uploads](evidence/native-request-uploads-sweep370/README.md) — sweep370:nine HTTP1/HTTP2 data/method checks and eight matching native Render log events pass for multipart UTF8/empty/binary data, valid chunked and gzip requests; exact parent/child/runtime cleanup, no new issue.

- [Native hosted WebSocket traffic](evidence/websocket-native-sweep369/README.md) — sweep369:two native Free creates/first deployments, six wss sessions,20 exact echoes,10 pongs, fragmentation/compression/normal closes and42 matching native log events pass; both former routes404, full exact cleanup, no new issue.

- [Native PostgreSQL notifications](evidence/postgres-native-notifications-sweep368/README.md) — sweep368:both listener directions pass ordered UTF8/empty payloads, sender-PID correlation, duplicate handling, rollback/unsubscribe suppression and natural exits; default-name vector replay also passes, full exact cleanup, no new issue.

- [Native queue consumer handoff](evidence/keyvalue-streams-sweep366/README.md) — sweep366:20 completed native checks pass for blocked delivery, exact payloads, pending ownership transfer, acknowledgment, empty timeout and type-error preservation; full own cleanup, no new issue.

- [Native PostgreSQL CSV files — sweep360](evidence/postgres-native-csv-sweep360/README.md):both clients preserve37 rows through three tables, NULL/empty/literal-marker values and identical ordered exports; invalid final row leaves0 rows.14 native SQL checks behave as expected; own database/files cleaned. No new issue.

- [Key Value logical databases and conditional writes — sweep359](evidence/keyvalue-database-selection-sweep359/README.md):18 native checks pass across both clients;40 values stay separate in databases1/2, matched/stale writes and default-db0/scan controls pass; own cache/endpoint cleaned. No new issue.

- [Native Key Value Pub/Sub — sweep356](evidence/native-pubsub-sweep356/README.md): both clients pass exact/pattern subscriptions,12 publishers and12 exact Unicode/CRLF/NUL/empty messages; own Free cache and endpoint cleaned. No new issue.

- [PostgreSQL logical schema round trip — sweep355](evidence/postgres-schema-roundtrip-sweep355/README.md): native Free18 create,9 native SQL checks, external custom dump/atomic restore and both restored fingerprints pass;64 rows/384KiB bytea and exact metadata/sequence preserved, own fixture/files cleaned. No new issue.

- [Native Redis pipe imports and exact bytes — sweep349](evidence/native-redis-pipe-sweep349/README.md): both clients complete50-command imports with0 errors/50 replies; opposite-client GET/hash checks verify all100 empty/CRLF/NUL/Unicode values. PTY byte preflight and cached teardown pass; own Free cache cleaned. No new issue.

- [Native psql file forwarding and atomic transactions — sweep348](evidence/psql-file-transactions-sweep348/README.md): both clients forward two spaced file paths, fail/roll back the whole transaction, and commit/read exact five-row empty/null/multiline/Unicode controls.9 native SQL comparisons pass; own Free PG18, cached endpoint and private inputs cleaned. No new issue.

- [Initial group precedence and empty values — sweep347](evidence/initial-group-precedence-sweep347/README.md): both native validators accept; one API-only Free Node/group create reaches its first Live deploy with exact build/runtime group and service precedence/empty/absence values; delayed same-boot/child, two native/three shared deploy reads and5 tests/19 named events pass. All owned resources cleaned; no new issue.

- [Large authored gzip and automatic Brotli — sweep346](evidence/static-large-precompressed-sweep346/README.md): captured native Free create201, two native inventory controls and13 actual HTTP representation/validator/range/no-transform comparisons pass; saved gzip79687bytes is preserved,3 source tests pass, own parent/child cleaned. No new issue.

- [Native Key Value persistence resets and identical saves — sweep345](evidence/keyvalue-persistence-reset-sweep345/README.md): journal→off→snapshot clears the expected old markers; cross-client data checks, actual runtime modes/process identities, delayed identical saves,3 shared API reads and4 pinned-engine transitions pass. Own store cleaned; no new issue.

- [Elixir dependency bootstrap and runtime tools — sweep336](evidence/elixir-dependency-bootstrap-sweep336/README.md): current generated default image installs Hex/Rebar through the documented dependency command; fresh Docker build and both runtime checks pass. Source-only, no hosted resources or new issue.

- [Custom PostgreSQL identity and display-name continuity — sweep335](evidence/postgres-custom-identity-sweep335/README.md): one Free13/custom-name fixture, seven actual SQL identity/data comparisons and two native renames pass with one process; comparison-client setup exclusions retained, cleaned. No new issue.

- [Blueprint service-type field applicability — sweep334](evidence/service-type-validation-sweep334/README.md): five service types/eight fields,90 native validity/exit comparisons and three maintained source tests pass; no resources created or new issue.

- [Native cron schedule validation — sweep333](evidence/cron-schedule-validation-sweep333/README.md):24 calendar/format vectors through both native clients,48 validity/exit comparisons and three maintained source tests pass; no resources created or new issue.

- [Native commit selection — sweep332, partial](evidence/commit-selection-sweep332/README.md): three exact artifact/commit comparisons and two native waits pass; later full-SHA deploy stays queued to the harness bound, is canceled and all four children cleaned. Remaining rollback/cache cases untested; no new confirmed issue.

- [Native Key Value policies and name-only continuity — sweep329](evidence/keyvalue-policy-sweep329/README.md): Free/off fixture, both native clients execute real redis-cli; cache/queue/raw-policy runtime checks and delayed rename data/process continuity pass, expected refusals retained; cleaned. No new issue.

- [PostgreSQL settings, native renames and clearing — sweep325](evidence/postgres-settings-sweep325/README.md): one Free18 fixture; both rename clients preserve settings, API replacement and clearing converge in real native SQL, two focused source tests pass; cleaned. No new issue.

- [PostgreSQL data across two suspend/resume cycles — sweep320](evidence/postgres-data-sweep320/README.md): one Free18 fixture, Bex/Render lifecycle by ID/name, real Bex psql, all committed/rollback/sequence snapshots and both process-change checks pass; cleaned. No new issue.

- [First saved variable through manual lifecycle](evidence/suspend-pending-first-sweep278/README.md) — sweep278: both clients preserve unset build/runtime through API-only suspend/resume and native Restart, four suspended native409 refusals allocate no rows, then standard deploy consumes V2; four distinct processes per fixture, both cleaned. No new finding.

- [Native request-log selector acceptance](evidence/request-log-selectors-sweep269/README.md) — sweep269: both clients pass32 exact row-ID comparisons for comma/encoded paths, methods, status classes, host/text and AND/OR/empty controls; six HTTP requests independently recorded, fixture cleaned. No new finding; interactive upstream defects remain separate.

- [Native Dashboard menu routing acceptance](evidence/089/live-native-menu-sweep267/README.md) — sweep267: all four Bex/Render Services/deploy Dashboard actions exit0 with unchanged OAuth-origin configs; exact captured cron URLs302, aliases307 and issuer200 pass. No hosted mutation;089 remains blocked for full login/browser/prefix acceptance.

- [Saved environment through sleep/wake and deploy-history settlement](evidence/sleep-pending-env-sweep266/README.md) — sweep266: saved/deployed markers survive sleep and restart; later standard deploy consumes V2. New106 owns served-then-parked history settlement; queued native wait excluded, all three fixtures cleaned.

- [Blueprint empty-variable reapply acceptance](evidence/095/live-empty-reapply-sweep263/README.md) — sweep263: actual group-only first/reapply retains new empty keys and preserves no-op/generated/omitted controls;36 decisive native validations, fixture cleaned. Existing095 moved to done; one relay timeout retained as inconclusive.

- [OAuth dashboard routing partial acceptance](evidence/089/live-routing-sweep262/README.md) — sweep262: owned/deleted web resource and deploy URLs now redirect302 with exact path/query, dashboard aliases307 and issuer metadata200. Read-only;089 remains blocked for complete native/browser/login acceptance.

- [Existing environment-group name acceptance](evidence/094/live-existing-group-sweep261/README.md) — sweep261: all48 native/captured plan cases pass across three locations, including padded/exact existing names and create/refusal controls; the empty owned group is unchanged then cleaned. Existing094 moved to done.

- [Managed-data live-tail refusal acceptance](evidence/083/live-refusal-sweep259/README.md) — sweep259: both clients now exit1/named400 before upgrade for Postgres/Key Value; twelve transport cases, historical and supported App tails pass; all three fixtures cleaned. Existing083 moved to done.

- [Static rewrite/SPA conditional requests](evidence/static-conditional-sweep258/README.md) — sweep258: all46 HTTP checks pass on one CLI-created site, including ETag conditions, HEAD metadata, exact prefix/suffix ranges and If-Range controls; API-only header/route setup, fixture cleaned. Extends done w4/272 without a new issue.

- [Static command-clear GraphQL/MCP acceptance](blocked/m59/evidence/live-static-adapters-sweep257/README.md) — sweep257: API-only clears on two CLI-created static sites publish exact recorded-commit source bytes and survive later native waited deploys; both fixtures cleaned. m59 remains blocked for dashboard and full t005 acceptance.

- [Failed-version native deploy waits and recovery](evidence/version-failure-wait-sweep255/README.md) — sweep255: both clients return1 for exact failed18 targets while old22 snapshots/instance IDs stay unchanged, then return0 for new Live24 targets; four save-only patches open no deploys, both fixtures cleaned. No new finding.

- [Cron capped-history CLI replay](evidence/cron-history-sweep254/README.md) — sweep254: native Go cron still reproduces existing w4/m210 at run11 and recovery; REST/GraphQL/MCP agree, both CLIs preserve all12 execution messages; source fix present/deployed revision unknown, fixture cleaned. Existing milestone remains blocked.

- [Native Node version precedence](evidence/runtime-precedence-sweep253/README.md) — sweep253: both clients honor .nvmrc22, explicit24/bounded-range overrides run24, unsupported18 fails by name; build/runtime markers and file bytes match; all five fixtures cleaned. Extends m51; no new finding.

- [w4/m199 native CLI timestamp replay](evidence/m199-native-cli/README.md) — sweeps128/132: both clients preserve quiet web and scheduled-cron records, timestamps and IDs in history/Live; cron tails remain open through idle metadata and changing instances. Both fixtures cleaned; existing milestones retain full verification scope.

- [w4/m200 native CLI corroboration](evidence/m200-native-cli/README.md) — sweep111: both clients corrupt the mixed stdout/stderr pair in Live while all five historical messages remain exact; complete native evidence and cleanup retained. Implementation stays with w4/m200.

- [Static filename publication acceptance](evidence/static-filenames-sweep301/README.md) — sweep301: installed Bex and pinned Render each publish13 marker files;58 exact GET/HEAD/control passes cover spaces, Unicode, percent and URL punctuation. Both own Free sites/two deploys cleaned; no new finding.

- [Generated-copy native validation](evidence/generated-copy-validation-sweep303/README.md) — sweep303:16 native calls expose existing w1/m175 source/live difference;8 qualified controls,4 masked negative outcomes,4 generated-copy refusals. Current exact source cases pass9 named events. No new issue or hosted mutation.

- [Cron preemption and existing startup-log loss](evidence/cron-preemption-sweep304/README.md) — sweep304: one complete graceful handoff with5,838ms gap; second interval remains unverified because existing w4/m174 loses the replacement start record (native/REST agree). Two parents/two deploys/seven runs cleaned; no duplicate issue.

- [Native Blueprint file encodings](evidence/file-encoding-sweep305/README.md) — sweep305: installed/published Bex and pinned Render pass30 LF/CRLF/BOM/UTF-16 validations, including15 identical located plan refusals; no hosted mutation or new issue.

- [Static hostname case and resume recheck](evidence/static-resume-hostcase-sweep308/README.md) — sweep308:24 case-preserving HTTP reads retain w4/309; four API-only resume cycles/56 samples retain w4/307 with2 absent-host404 samples. Four focused source checks pass. Both own Free sites/two deploys cleaned; no duplicate issue.

- [PostgreSQL target-floor dry-run recheck](evidence/postgres-upgrade-floor-sweep309/README.md) — sweep309: REST/MCP still accept14/15 previews (existing w4/308); ten other preview controls and both native omitted-version renames pass.24 focused source checks pass. Own Free database cleaned; no duplicate issue.

- [Key Value deadline and data retention acceptance](evidence/keyvalue-deadlines-sweep310/README.md) — sweep310: both native clients complete preview/suspend/resume; independent TLS checks pass30 times across two new processes, preserving exact expiry deadlines and structured data. Own Free instance/credentials cleaned; no new issue.

- [Mixed-family historical log acceptance](evidence/mixed-family-logs-sweep311/README.md) — sweep311:30 native query passes across both clients; two API cursor chains each recover the same33 records from own App/Postgres/Key Value.13 focused source checks pass; three parents/one deploy cleaned. No new issue.

- [Cron plan policy validation recheck](evidence/cron-plan-validation-sweep312/README.md) — sweep312: all three native clients still accept Free cron (existing048);42 neighboring controls and32 final focused/diagnostic source checks pass. Validation only; no hosted mutations.

- [Blueprint per-kind name recheck](evidence/blueprint-name-kinds-sweep313/README.md) — sweep313:42 actual native validations/36 final comparisons; four valid cross-kind cases remain refused per client (existing w1/126),24 validity/exit controls and20 final source checks pass. No resources or new issue.

- [Existing workspace host-target checks](done/m74/evidence/cli-sweep316/README.md) — sweep316:28 native validations expose14 false accepts/14 controls; four source failures and two invalid fake-client applies,33 existing checks pass. Scheduled with314 origin loss in m74; two Free fixtures cleaned.

- [Static byte-range acceptance and scoped rollout recheck](evidence/static-ranges-sweep317/README.md) — sweep317:62 text/binary/multipart/conditional/HEAD HTTP checks and three source controls pass; all three clients now refuse Free cron and accept the shared service/database name (six validations). Two sites/two deploys cleaned; no new issue.

## Inbox

- [ ] **143** — [Web services receive caller-forged `True-Client-IP` / `CF-Connecting-IP`, the client-IP headers Render tells apps to trust](143.md) ← `/qa-find-bugs-cli` sweep 1241 2026-10-20: major, 60m

- [ ] **142** — [Blueprint `ipAllowList` refuses a bare IP address, so Render's own example `render.yaml` fails validation](142.md) ← `/qa-find-bugs-cli` sweep 1240 2026-10-20: Render's schema says “IP address or CIDR block”; minor, 30m

- [ ] **141** — [Deploy-hook 400s name REST body fields the caller never sent (`commitId`, `imageUrl`) instead of the hook's `ref`/`imgURL`](141.md) ← `/qa-find-bugs-cli` sweep 1239 2026-10-20: minor, 25m

- [ ] **140** — [Render's “allow any IP” datastore rule `0.0.0.0/0` locks out every IPv6-capable client](140.md) ← `/qa-find-bugs-cli` sweeps 1230/1232 2026-10-20: with exactly `0.0.0.0/0`, `bex psql` and `redis-cli` by hostname die with a bare TLS EOF over IPv6 while IPv4 works. Decision-independent piece of [w9/m164](../w9/blocked/m164/README.md)

- [x] **126** — [A source update before a release's build starts still rolls the previous image as that release](done/126.md) — **PROMOTED (2026-10-19)** to `m121`.

- [x] **125** — [Backward log pages without a startTime hand out inverted cursors past the first hour](done/125.md) ← w8/123 code review 2026-10-19: the cursor start defaults to 1h while the read looks back 7 days — **DONE (2026-10-19)**

- [x] **136** — [Audit every Render-gated create/update body against its bex handler](done/136.md) ← w8/135 fix 2026-10-19: the schema/handler body conflict class — **DONE (2026-10-19)**

- [x] **135** — [`POST`/`PATCH /registrycredentials` are unreachable: the Render-schema validator and the bex handler demand contradictory bodies](done/135.md) ← `/qa-find-bugs-cli` sweep 676 2026-10-19 — **DONE (2026-10-20)**

- [x] **137** — [Static-site single-rule REST operations are missing: `POST`/`DELETE …/headers`, `POST`/`PATCH`/`DELETE …/routes` answer 405/404](done/137.md) ← `/qa-find-bugs-cli` sweep 801 2026-10-20 — **DONE 2026-10-20** (w5/165): implemented in `e8d28e88e`, deployed, live acceptance passed on `bex-canary`.

- [x] **138** — [Per-service notification-override `GET`/`PATCH` omit the schema-required `serviceId`](done/138.md) ← `/qa-find-bugs-cli` sweep 806 2026-10-20 — **DONE (2026-10-20): fixed `d98da2870`, live-verified at pin `9e9c28e82d0f`**

- [x] **139** — [Key Value REST refuses the hyphenated enum spellings its handler, MCP schema, and ADR021 all say are accepted](done/139.md) ← `/qa-find-bugs-cli` sweep 864 2026-10-20 — **DONE 2026-10-20** (w5/165): fixed in `9e9c28e82`, deployed, live acceptance passed on `bex-canary`.

- [x] **134** — [`POST /blueprints` answers every fetch failure with a bare 500, hiding its own classified reason](done/134.md) ← `/qa-find-bugs-cli` loop 2026-10-19 — **DONE (2026-10-19)**

- [x] **133** — [`POST /blueprints/generate` answers malformed JSON with a bare 500](done/133.md) ← `/qa-find-bugs-cli` loop 2026-10-19 — **DONE (2026-10-19)**

- [x] **132** — [Log filters `task`, `taskRun`, and `sandbox` are accepted but silently ignored](done/132.md) ← `/qa-find-bugs-cli` loop 2026-10-19 — **DONE (2026-10-19)**

- [x] **131** — [A nonexistent Docker Hub image burns the full 15-minute rollout budget instead of the 90-second pull-failure settle](done/131.md) ← `/qa-find-bugs-cli` sweep 532 2026-10-19 — **DONE (2026-10-19)**

- [x] **130** — [An SSH-form repo URL is accepted at create but every build dies on SSH host-key verification](done/130.md) ← `/qa-find-bugs-cli` loop 2026-10-19 — **DONE (2026-10-19)**

- [x] **129** — [Postgres reads omit the Render-required `suspenders` and `role` fields](done/129.md) ← `/qa-find-bugs-cli` loop 2026-10-19 — **DONE (2026-10-19)**

- [x] **128** — [Restart/Rollback refusals name the internal App name instead of the service the caller asked for](done/128.md) ← `/qa-find-bugs-cli` sweep 503 2026-10-19: Restart/Rollback refusals name the internal App CR name — **DONE (2026-10-19)**

- [x] **127** — [An Update Source during a release in flight rolls that release on the previous image](done/127.md) (filed as `124`, renumbered 2026-10-19 after a number collision) ← w8/m119 code review 2026-10-19: `sourceUpdatePending` is decided before `releaseInFlight`, so a source update saved mid-build/pre-deploy reuses the prior image under the new revision — **DONE (2026-10-19)**

- [x] **124** — [qa-login.sh --login-challenge fails when the seeded session needs no refresh](done/124.md) ← `/qa-find-bugs-cli` solo loop 2026-10-19; reopened the same day (board repair: it had been closed with another item's fix) — **DONE (2026-10-19)**

- [x] **123** — [A far-future log endTime with no startTime still returns 500 (or times out)](done/123.md) ← from `/qa-find-bugs-cli` solo loop run 2 sweep r1, 2026-10-19 — **DONE (2026-10-19:** a far-future `endTime` with no `startTime` reads up to now plus a 10-minute clock grace on every surface; the query-hours cap measures that window)

- [x] **122** — [Production CI runner's Docker cannot apply memory or CPU limits](done/122.md) — **DONE (2026-10-18: cgroup v2 nesting shipped in block-eden-mono `8abb38b0d5` + `d62abafdde`; all 10 runners rolled onto the new image while idle; a `--memory 512m --cpus 1 --pids-limit 128` container runs in a bex-production runner)**

- [x] **121** — [Platform-subdomain services answer plain HTTP instead of redirecting to HTTPS](done/121.md) — **PROMOTED (2026-10-18)** to `m118`.

- [x] **120** — [Logs query with a future start and open end returns 500](done/120.md) — **DONE 2026-10-18:** a `startTime` after now with no `endTime` returns an empty last page on every surface, where Loki used to be sent an inverted range and the read failed with a 500. An `endTime` alone, even a future one, now counts its lookback and cursors back from the earlier of the end and now (`456090929`).

- [x] **119** — [Env-group tombstones accumulate forever and every scan still reads them](done/119.md) — **CLOSED AS ALREADY MET (2026-10-18):** m114 removed the scan and fences; final cutover code removed in `2bd432ac2`, value-only real-store coverage added in `0f456f069`; findings preserved as superseded history

- [ ] **118** — [Forward stdin to noninteractive psql imports](blocked/118.md) (55m, major) — **BLOCKED (2026-10-16: upstream patch + real-child regressions prepared in `blocked/118-upstream.patch` (and a variant stacked on 079 in `blocked/118-upstream-stacked-on-079.patch`): the pinned CLI's non-interactive psql never sets the child's stdin, so `\copy … FROM pstdin` reads EOF and imports 0 rows; needs the user to authorize external submission to render-oss/cli after a fresh dedupe, then upstream acceptance/release, a Bex CLI pin/release and the two-client import replay with cleanup; no fork per DO_NOT_DO)**

- [x] **117** — [Native Ruby enforces committed dependency locks](done/117.md) (55m, major) — **DONE 2026-10-17:** both original stale first builds correctly reject without prefix; matched-lock-only replacements Live with exact locked gem and six HTTP200 responses, shared reads and full cleanup (sweep417).

- [x] **116** — [Native Python preserves HTTPS URLs and redirect completion](done/116.md) (55m, major) — **DONE 2026-10-17:** both original first native deployments Live, twelve ordinary HTTPS requests with correct URLs/redirects and build/runtime defaults; exact cleanup/shared reads in sweep415.

- [x] **115** — [Correct MCP idle field omission and WebSocket-direction instructions](done/115.md) (30m, minor) — **DONE 2026-10-18:** live MCP tools/list carries the corrected omission/0/both-directions description; an owned Free fixture previews omitted as the stored 60 and explicit 0 as 0, and was deleted ([evidence](evidence/115/hosted-2026-10-18/README.md)).

- [x] **114** — [Native Ruby defaults Rails to production](done/114.md) (55m, major) — **DONE 2026-10-17:** both original native first deployments Live, real production build/runtime and six HTTP200 responses without authored override; exact cleanup and shared reads in sweep413.

- [ ] **113** — [Native Python omits Gunicorn application access logging](blocked/113.md) (55m, minor) — **BLOCKED:** implemented; hosted defaults3.10–3.14, group override subset451 and actual generated web/static default/custom/empty subset452 pass. Remaining Python-line/source/type/phase gates stay open.

- [x] **112** — [Native Elixir production build/runtime MIX_ENV](done/112.md) (55m, major) — **DONE 2026-10-16:** sweep385 accepts both original unprefixed first production-dependency/compiler deployments and runtime Mix/prod artifacts, exact source,8 native log reads/6 shared reads and full exact-owned cleanup; source artifact/override/exclusion gates inherited.

- [ ] **111** — [Native Node runtime production default](blocked/111.md) (55m, major) — **BLOCKED (2026-10-16: source fixed; sweep384 passes both original first Free-web deployments, build/runtime phase split, pre-deploy and native staging/empty redeploys, twelve native log proofs/six shared reads and exact cleanup; prior paid-only cron observation remains unverified)**

- [x] **109** — [Environment-group bulk writes are rejected as a missing single-key parameter](done/109.md) — **DONE 2026-10-15:** hosted sweep324 accepts the bulk short route, atomic value refusal and strict single-key/unknown/method neighbors; shared value readbacks and exact unlinked-group cleanup pass.

- [ ] **108** — [Same-type clones silently ignore explicit empty overrides](blocked/108.md) — **BLOCKED (2026-10-15: upstream patch + clone regressions prepared in `evidence/108/upstream/108-upstream.patch` (stacked variant for m68+075); needs the user to authorize external submission to render-oss/cli after a fresh dedupe, then upstream acceptance/release, a Bex CLI pin/release and the two-client live empty-override clone replay with cleanup; no fork/request repair per DO_NOT_DO)** Sweep292 adds137 local applicability/control captures; same issue and estimate.
- [ ] **107** — [A valid output flag cannot override an invalid output default](blocked/107.md) — **BLOCKED (2026-10-15: upstream root-init patch + real-detector regression prepared in `evidence/107/upstream-output-priority.patch`; needs the user to authorize external submission to render-oss/cli after a fresh dedupe, then upstream acceptance/release, a Bex CLI pin/release and the sweep279 native replays; no fork/bridge workaround per DO_NOT_DO)**

- [x] **106** — [Served deploys settle Live after auto-hibernation](done/106.md) — **DONE2026-10-15:** sweep287 passes the parked two-client live gate with exact-owned cleanup; source fix and regressions already landed.

- [ ] **103** — [CLI trims a local file path and silently uploads its neighbor](blocked/103.md) — **BLOCKED (2026-10-14: upstream parser patch + regressions prepared in `evidence/103/upstream-secret-file-path.patch`; needs the user to authorize external submission to render-oss/cli after a fresh dedupe, then upstream acceptance/release, a Bex CLI pin/release and the twelve native receipts; no fork/backend workaround per DO_NOT_DO)**

- [x] **102** — [Blueprint environment copies use a literal shadowed by a reference](done/102.md) — **DONE 2026-10-13** (fix `13a2fd5eb`; sweep212 16-case native matrix passes; DeployStack zero-write and GraphQL/MCP agreement in code; hosted apply recorded as a boundary)

- [x] **101** — [Blueprint static rule errors point to the service instead of the invalid rule](done/101.md) — **DONE 2026-10-13** (fix `2fb562c4a`; sweep212 native matrix passes with exact field locations; GraphQL/MCP details agree in code)

- [x] **100** — [Blueprint reference refusals locate their own authored entry](done/100.md) — **DONE2026-10-15:** sweep287 passes the parked two-client live gate with exact-owned cleanup; source fix and regressions already landed.

- [x] **098** — [Blueprint validation accepts Key Value envVarKey refs that resolve to an empty key](done/098.md) — **DONE 2026-10-13** (fixes `fcd0d1124`/`0daac2275`; sweeps 207/212 native matrices pass; REST/GraphQL/MCP agreement in code; hosted apply recorded as a boundary)

- [x] **097** — [Literal alert/emergency severities become unknown and disappear from CLI level filters](done/097.md) — **DONE 2026-10-13** (fix `b250f2acb`; live App severity labels and all 20 native filters pass; platform/GraphQL/MCP/dashboard and pre-rollout history recorded as unexercised boundaries)

- [x] **096** — [Blueprint validation ignores a supplied service-reference type](done/096.md) — **DONE 2026-10-13** (fix `e8129f739`; sweep207 32-case native matrix passes; refusal now located at `fromService.type`; hosted apply, non-REST surfaces and out-of-file hosts recorded as boundaries)

- [x] **095** — [Blueprint reapply skips a new empty variable in an existing environment group](done/095.md) — **DONE 2026-10-15 UTC:** sweep263 accepts actual group-only first/reapply, presence/no-op/invalid controls,36 decisive native validations and exact cleanup; deployed revision unknown.
- [x] **094** — [Blueprint plans call an existing environment group a create when its name is padded](done/094.md) — **DONE 2026-10-15 UTC:** sweep261 accepts48 native current-state validation cases with full plan captures, controls and exact owned-group cleanup; deployed revision unknown.

- [ ] **093** — [CLI scope selectors cannot resolve accepted ID-shaped project and environment names](blocked/093.md) — **BLOCKED (2026-10-12: upstream patch prepared and tested — [`093-upstream.patch`](blocked/093-upstream.patch); needs the user to authorize external submission to render-oss/cli after a dedupe, then upstream acceptance and a Bex pin refresh + CLI release; no fork per DO_NOT_DO)**

- [x] **092** — [Blueprint parser loses explicit empty env-value presence](done/092.md) — **DONE 2026-10-13** (fix `a7f6c1531`; sweep207 44-check native matrix passes; empty seed seeded once and retained through DeployStack; hosted apply and non-REST surfaces recorded as boundaries)

- [ ] **091** — [SSH project filters start connection preparation before choosing a service](blocked/091.md) — **BLOCKED (2026-10-12: upstream patch prepared and tested — [`091-upstream.patch`](blocked/091-upstream.patch); needs the user to authorize external submission to render-oss/cli after a dedupe, then upstream acceptance and a Bex pin refresh + CLI release; no fork per DO_NOT_DO)**

- [ ] **090** — [CLI Dashboard opens a relative path from a fresh config](blocked/090.md) — **BLOCKED (source fix is on main; current-build service/deploy Dashboard actions pass in sweeps209/211, including equivalent API bases; published 0.3.2 still opens relative paths; CLI release, real browser arrival and complete shared-link acceptance remain)**

- [ ] **089** — [CLI dashboard actions open the OAuth host and return 404](blocked/089.md) — **BLOCKED (routing and native Services/deploy menu legs pass in sweeps262/267; fresh device login, authenticated page arrival and remaining prefix/full acceptance still required)**

- [x] **088** — [Git source changes retain image runtime metadata and break CLI cloning](done/088.md) — **DONE 2026-10-15** (fix `17a128ed8`; sweep252 both native clients save/clone/deploy successfully, HTTP200, shared projections/exports agree, four-kind source controls pass; all fixtures cleaned)

- [ ] **087** — [Clearing a datastore project filter leaves its active scope in place](blocked/087.md) — **BLOCKED (2026-10-12: upstream patch prepared and tested — [`087-upstream.patch`](blocked/087-upstream.patch); needs the user to authorize external submission to render-oss/cli after a dedupe, then upstream acceptance and a Bex pin refresh + CLI release; no fork per DO_NOT_DO)**

- [ ] **086** — [Empty project filters show every workspace resource](blocked/086.md) — **BLOCKED (2026-10-12: upstream patch prepared and tested — [`086-upstream.patch`](blocked/086-upstream.patch); needs the user to authorize external submission to render-oss/cli after a dedupe, then upstream acceptance and a Bex pin refresh + CLI release; no fork per DO_NOT_DO)**

- [ ] **085** — [Interactive datastore errors omit their underlying detail](blocked/085.md) — **BLOCKED (2026-10-12: upstream patch prepared and tested — [`085-upstream.patch`](blocked/085-upstream.patch); needs the user to authorize external submission to render-oss/cli after a dedupe, then upstream acceptance and a Bex pin refresh + CLI release; no fork per DO_NOT_DO)**

- [ ] **084** — [Postgres restart reports success when the API refuses it](blocked/084.md) — **BLOCKED (2026-10-12: upstream patch prepared and tested — [`084-upstream.patch`](blocked/084-upstream.patch); needs the user to authorize external submission to render-oss/cli after a dedupe, then upstream acceptance and a Bex pin refresh + CLI release; no fork per DO_NOT_DO)**
- [x] **083** — [Refuse unsupported managed-data live tails before WebSocket upgrade](done/083.md) — **DONE 2026-10-15 UTC:** sweep259 accepts both native clients/families, twelve diagnostic transport refusals, historical/App-tail controls and exact cleanup; deployed revision unknown.
- [ ] **082** — [Applying the CLI log filter silently removes fractional timestamp bounds](blocked/082.md) — **BLOCKED (2026-10-12: upstream patch prepared and tested — [`082-upstream.patch`](blocked/082-upstream.patch); needs the user to authorize external submission to render-oss/cli after a dedupe, then upstream acceptance and a Bex pin refresh + CLI release; no fork per DO_NOT_DO)**

- [x] **081** — [Blueprint validation accepts port links to services with no network address](done/081.md) (55m) ← functional CLI QA, 2026-10-09; both clients, multipart API controls and actual parser values; shared server addressability guard. — **DONE 2026-10-10** (fix `5b91fcdab`; live two-client validation passed)

- [ ] **079** — [Noninteractive psql silently drops warnings on successful SQL](blocked/079.md) — **BLOCKED (2026-10-09: upstream patch prepared and tested — [`079-upstream.patch`](blocked/079-upstream.patch); needs the user to authorize external submission to render-oss/cli after a dedupe, then upstream acceptance and a Bex pin refresh + CLI release; no fork per DO_NOT_DO)**

- [x] **078** — [Empty workspace flags bypass current-workspace name resolution](done/078.md) (55m) ← functional CLI QA, 2026-10-09; release and HEAD validation failures, exact offline captures across three commands; launcher fallback correction. — **DONE 2026-10-10** (launcher fix `1ccd5bed6`; live control passed; ships to users in the next CLI release)

- [ ] **077** — [Repeated environment flags silently convert interior CRLF to LF](blocked/077.md) — **BLOCKED (2026-10-09: upstream patch prepared and tested — [`077-upstream.patch`](blocked/077-upstream.patch); needs the user to authorize external submission to render-oss/cli after a dedupe, then upstream acceptance and a Bex pin refresh + CLI release; no fork per DO_NOT_DO)**

- [ ] **076** — [Unicode descriptions hit a byte limit advertised as 255 characters](blocked/076.md) — **BLOCKED (2026-10-09: upstream patch prepared and tested — [`076-upstream.patch`](blocked/076-upstream.patch); needs the user to authorize external submission to render-oss/cli after a dedupe, then upstream acceptance and a Bex pin refresh + CLI release; no fork per DO_NOT_DO)**
- [ ] **075** — [Clones inherit fields forbidden by requested type/runtime](blocked/075.md) — **BLOCKED (2026-10-15: type + runtime/source-kind applicability patch prepared in `evidence/075/upstream-v2/075-upstream-v2.patch` (16 failing-first clone cases); Git-only root/build-filter/auto-deploy on native→image recorded as a remaining limitation; needs the user to authorize external submission to render-oss/cli after a fresh dedupe, then upstream acceptance/release, a Bex CLI pin/release and live two-client clone acceptance; no fork/server workaround per DO_NOT_DO)**
- [x] **074** — [Service secret-file upserts return 200 instead of the pinned 201](done/074.md) (35m) ← functional CLI QA + API-only diagnostics, 2026-10-09; actual pinned typed success result is absent. — **DONE 2026-10-10** (fix `1d95b6b8f`; live production check passed)
- [x] **073** — [Empty secret-file responses omit required content](done/073.md) (55m) ← functional CLI QA + API-only diagnostics, 2026-10-09; mounted bytes and GraphQL pass, item/bulk response fields do not. — **DONE 2026-10-10** (fix `8cf443bcc`; live production check passed)
- [ ] **072** — [Image cron command updates fail in the pinned upstream CLI](blocked/072.md) — **BLOCKED (2026-10-09: upstream patch prepared and tested — [`072-upstream.patch`](blocked/072-upstream.patch); needs the user to authorize external submission to render-oss/cli after a dedupe, then upstream acceptance and a Bex pin refresh + CLI release; no fork per DO_NOT_DO)**

- [x] **071** — [Namespace prune shares m52's stale-snapshot shape](done/071.md) ← filed 2026-10-09 from w8/m52 t002 — **DONE 2026-10-09** (fresh `GetTenant` + UID-bound delete)
- [ ] **070** — [Upstream `pgcli --help` advertises flags that real pgcli refuses](blocked/070.md) — **BLOCKED (2026-10-09: submitted as [render-oss/cli#57](https://github.com/render-oss/cli/pull/57); waiting on upstream review/merge and a release, then a Bex pin refresh + CLI release)**
- [ ] **069** — [A native build's "==> Using <runtime> <line>" narration is sometimes missing from the build log](blocked/069.md) — **BLOCKED (2026-10-08: not reproduced in 3 fresh first builds — narration present in both kubelet and Loki; needs the next occurrence captured with build-Job events before the service is deleted; lead: a replaced build pod takes its log files with it)**
- [ ] **068** — [Upstream `logs -o yaml` merges multiple entries into an invalid mapping](blocked/068.md) — **BLOCKED (2026-10-09: submitted as [render-oss/cli#56](https://github.com/render-oss/cli/pull/56); waiting on upstream review/merge and a release, then a Bex pin refresh + CLI release)**
- [ ] **067** — [Upstream token refresh deletes the saved refresh credential on a temporary 503 or 429](blocked/067.md) — **BLOCKED (2026-10-09: submitted as [render-oss/cli#55](https://github.com/render-oss/cli/pull/55); waiting on upstream review/merge and a release, then a Bex pin refresh + CLI release)**
- [ ] **066** — [Upstream `psql -c` launches with an empty database target when IP discovery fails](blocked/066.md) — **BLOCKED (2026-10-09: submitted as [render-oss/cli#54](https://github.com/render-oss/cli/pull/54); waiting on upstream review/merge and a release, then a Bex pin refresh + CLI release)**
- [x] **065** — [Postgres CA preparation blocks `psql`/`pgcli` help with a database selector on a connection-info 503](done/065.md) (~40m, minor) ← `/qa-find-bugs-cli` w8 loop, 2026-10-07 offline sweep 2; checkout regression, production confirmation outstanding. — **DONE 2026-10-08**
- [x] **064** — [A stale `BEX_WORKSPACE` name blocks help, version and workspace recovery before dispatch](done/064.md) (~50m, minor) ← `/qa-find-bugs-cli` w8 loop, 2026-10-07 offline sweep 1; checkout regression, production confirmation outstanding. — **DONE 2026-10-08**
- [x] **062** — [Static `routes`/`headers` REST answers bare items: no `{route|header, cursor}` envelope and no `id`/`priority`, unlike Render's contract](done/062.md) (~50m, minor) ← `/qa-find-bugs-cli` w8 loop, 2026-10-06 sweep 43. — **DONE 2026-10-07**
- [x] **063** — [Env-group linking and service secret files drift from Render's REST contract: link → 204 no body, `serviceLinks` bare ID strings, bulk `PUT …/secret-files` → 405](done/063.md) (~55m, minor) ← `/qa-find-bugs-cli` w8 loop, 2026-10-06 sweep 44. — **DONE 2026-10-07**
- [ ] **048** — [Cron jobs are paid-only, as on Render](blocked/048.md) — **BLOCKED (2026-10-15: user decided "match Render"; implemented — `paidOnlyServiceType` + dashboard paid-only set include `cron_job`, free-cron Blueprint overlay removed, legacy free cron export refused by name, docs/examples/scripts updated. Needs the deploy, then the hosted CLI acceptance (free create refused, plan-less → starter, `--plan free` update refused, Blueprint free cron flagged, free web control) and the production free-cron inventory query + owner notice per ADR030 §7)** Hosted free side 2026-10-18: both clients refuse a free cron create and flag a free cron Blueprint ([evidence](evidence/048/hosted-2026-10-18/README.md)); the paid-fixture checks and the inventory decision remain.

- [x] **061** — [Render's `/metrics/{memory,cpu,…}` answers a bare `404 not found` for a live Key Value or Postgres](done/061.md) — **DONE 2026-10-05**: Key Value `memory` serves `kv_memory`; other datastore pairs are a coded 400 naming the pair; a missing id stays 404.
- [x] **060** — [Service events answer `service_hibernated`/`service_woken` on the Render REST feed, outside Render's event-type enum](done/060.md) — **DONE 2026-10-05**: kept as declared bex extensions per ADR018; a gate now fails on any REST event type outside Render's enum that is not declared with a reason.
- [x] **059** — [Every Render-route 400 for an unknown query parameter says "request contains an unsupported query parameter" without naming it](done/059.md) — **DONE 2026-10-05**: the refusal names every undeclared parameter (sorted, quoted, capped) and the operation.
- [x] **058** — [Deploy `trigger` answers `create` and `config_change`, values outside Render's enum](done/058.md) — **DONE 2026-10-05**: REST/MCP map onto Render's enum (`bexTrigger` keeps bex's value); conformance seeds every trigger.
- [x] **057** — [Postgres REST wire drift: `plan` answers `basic-256mb`, and create refuses `parameterOverrides`](done/057.md) — **DONE 2026-10-05**: REST answers Render's `basic_256mb`; create accepts `parameterOverrides`; decode errors name the field.
- [x] **056** — [REST create skips the per-type setting gates that update and Blueprints enforce](done/056.md) — **DONE 2026-10-05**: one applicability rule per setting shared by create, update and Blueprints.
- [x] **055** — [`blueprints validate` passes reserved domains that apply refuses, and points hostname errors at `services[n].name`](done/055.md) — **DONE 2026-10-05**: validate/preview run apply's read-only host-claim gate; hostname refusals locate at `domains[i]`.
- [x] **054** — [Blueprint refusal messages ignore the registry's recorded reason](done/054.md) — **DONE 2026-10-05**: Postgres 10–12 uses the API's wording and list; cron/static preDeployCommand say what to do; fallbacks explain via sanitized registry reasons.
- [x] **053** — [A root directory missing from the repo fails the build with BuildKit's `invalid local: resolve : lstat /source/…`](done/053.md) — **DONE 2026-10-05**: the clone step refuses a missing root directory, Docker context or Dockerfile by name as tenant input.
- [x] **052** — [A failed native build reports bex's internal env-decoding wrapper instead of the user's build command and exit code](done/052.md) — **DONE 2026-10-04**: the env loader moved to a COPY heredoc, and the failure summary reads `build command '<cmd>' exited with code N`.
- [x] **051** — [`blueprints validate` reports only one dangling `fromDatabase`/`fromService` reference, and which one changes from run to run](done/051.md) — **DONE 2026-10-04**: every dangling reference gets its own located entry, deterministically; apply names the sorted first.
- [x] **050** — [Platform internals ship as tenant log lines: kubelet "failed to try resolving symlinks" and CNPG's Go TLS-handshake line](done/050.md) — **DONE 2026-10-04**: exact shipper drops on App and Postgres, logger-less instance-manager JSON dropped, pod path re-types the body.
- [x] **049** — [Postgres and Key Value accept another resource's ID as their name](done/049.md) — **DONE 2026-10-04**: create, rename and Blueprint refuse an ID-shaped datastore name with `NAME_RESOURCE_ID_RESERVED`.
- [x] **047** — [Every tenant's request log carries Traefik's platform-wide request counter, pod IPs and internal router names](done/047.md) — **DONE 2026-10-04**: Traefik drops `RequestCount`/`ServiceAddr`/`ServiceURL`/`RouterName` from the access log at the source.
- [x] **046** — [Create dry-runs preview a service/Postgres/Key Value past the plan's count cap](done/046.md) — **DONE 2026-10-04**: every create dry run reads the workspace `tenant-quota` and refuses past the count cap with the real create's message.
- [x] **045** — [Service create dry-run runs every read-only check the real create runs](done/045.md) — **DONE 2026-10-03**: billing (402 parity), duplicate name, reserved env, repo access, registry credential, and the custom-domain gate (reserved `dashboard.bex.co`, caps, claimed hosts) now run before the preview. Postgres/Key Value dry runs run the billing gate too. Count caps are filed as 046.

- [x] **037** — [`bex logout` revokes only the presented grant](done/037.md) — **DONE 2026-10-03**: per-token Hydra RFC 7009 revoke replaces the consent-session wipe; proven against Hydra v26.2.0 (other device keeps access + refresh).
- [x] **038** — [`BEX_WORKSPACE=<name>` resolves to the workspace id](done/038.md) — **DONE 2026-10-03**: launcher resolves a name once via `GET /v1/owners?name=` (unknown/duplicate names exit 1 with named errors); ships to users with the next `bex-cli/v*` release.
- [x] **039** — [A first deploy stalled on its health check keeps the probe diagnosis at the deadline](done/039.md) — **DONE 2026-10-03**: settle falls back to the current-generation Ready stall diagnosis when the late restart hides it from the pod scan.
- [x] **040** — [Restart is refused for cron jobs, like Render](done/040.md) — **DONE 2026-10-03**: named 400 on every surface before any mutation; dashboard offers no Restart on a cron job.
- [x] **041** — [`blueprints validate` on a zero-byte file returns a validation result](done/041.md) — **DONE 2026-10-03**: empty manifests get `valid:false` "one YAML document"; malformed requests name the missing field.
- [x] **042** — [`imageUrl` / deploy-hook `imgURL` must keep the configured repository](done/042.md) — **DONE 2026-10-03**: Render's host/repository/name rule enforced in the shared trigger validator; named 400 on REST, GraphQL, MCP and the hook; production had no cross-repository overrides.
- [x] **043** — [`logs --end <t>` includes the line stamped exactly `t`](done/043.md) — **DONE 2026-10-03**: Loki receives the caller's End + 1 ns; log windows accept `start == end`; 150-line paging verified gap-free both directions.
- [x] **044** — [Log lines are ordered chronologically, not by timestamp string](done/044.md) — **DONE 2026-10-03**: `core.TimestampLess` at every sort site in `logs` and `datastorelogs`; found while fixing 043.
- [x] **032** — [Promoted to m48](done/032.md) (2026-10-02); original evidence retained, implementation tracked in the milestone.
- [x] **033** — [Promoted to m49](done/033.md) (2026-10-02); original production findings retained.
- [x] **034** — [Validate service display names](done/034.md) — **DONE 2026-10-02**: shared named refusals, Unicode/identity controls, full backend/lint checks, 135 live checks and fixture cleanup passed.
- [x] **036** — [Closed as already fixed](done/036.md) (2026-10-02): `280377a6b` routes release-snapshot Secret I/O through the uncached client; current code and passing namespace regression verified. Original blast-radius findings and the 2026-09-30 production re-check are retained; survivor `w4/171`.
- [x] **035** — [Completed through m50](done/035.md) (2026-10-02); original production findings retained.

> `031.md` filed 2026-09-27 (`/qa-find-bugs-cli` sweep 45) and fixed 2026-09-27 — moved to `done/`. `bex logs --level warning|notice|critical|alert|emergency` now matches: queries map Render's level names onto the stored buckets, and the shipper emits `warning` (not `warn`) from now on.

> `030.md` filed 2026-09-27 (`/qa-find-bugs-cli` sweep 44) and fixed 2026-09-27 — moved to `done/`. Postgres logs drop CNPG's instance-manager chatter and unwrap `logger=postgres` records into PostgreSQL's own line shape with a `level`, in the shipper (verified with a local Alloy v1.20.0 run) and in both direct-pod fallbacks.

> `029.md` filed 2026-09-27 (`/qa-find-bugs-cli` sweep 41) and fixed 2026-09-27 — moved to `done/`. Binary secret files are altered in transit by the upstream client and the JSON-string contract. `docs/bex-cli.md` and the CLI checklist document it with a base64 recipe, and the dashboard upload refuses a non-UTF-8 file with that hint instead of saving a mangled copy.

> `028.md` filed 2026-09-27 (`/qa-find-bugs-cli` sweep 40) and fixed 2026-09-27 — moved to `done/`. Scheduled and manual cron runs carry `backoffLimit: 0`: a failing run executes once and ends `failed`, instead of re-running the command up to 7× over ~11 minutes while every tick in between was skipped.

> `027.md` filed 2026-09-26 (`/qa-find-bugs-cli` sweep 35) and fixed 2026-09-27 — moved to `done/`. Create and Blueprint validate refuse env var names every env write refuses (`1BAD`, `BAD KEY`), and pre-fix debris on `spec.Env` reads unmanaged and is removable by DELETE or a whole-set PUT. Production had none.

> `026.md` filed 2026-09-26 (`/qa-find-bugs-cli` sweep 33) and fixed 2026-09-27 — moved to `done/`. `blueprints validate` now reaches its own 10 MiB cap (the route is exempt from the global 2 MiB body limit, and an oversize file gets its own 413), and an out-of-file `fromService … property: host` target that exists nowhere is a located validation error instead of `valid: true`.

> `025.md` filed 2026-09-26 (`/qa-find-bugs-cli` sweep 32) and fixed 2026-09-27 — moved to `done/`. The kubelet's untimestamped `unable to retrieve container logs for containerd://…` placeholder is now a platform `==> logs for this instance are no longer available: its container was removed` line instead of the tenant's `app` output.

> `024.md` filed 2026-09-26 (`/qa-find-bugs-cli` sweep 23) and fixed 2026-09-27 — moved to `done/`. `bex logs -r A,B --tail` followed only the first resource. Subscribe now authorizes and slots every resource, then merges all of them onto one stream (WS/SSE/NDJSON).

> `023.md` filed 2026-09-26 (`/qa-find-bugs-cli` sweep 14) and fixed 2026-09-27 — moved to `done/`. A deploy trigger (API, restart, hook), rollback, cancel, job create or lifecycle write on a service being deleted now answers `404 not found` before any side effect, like every read, instead of reaching the terminating App and surfacing a 500.

> `021.md` filed 2026-09-23 (`/qa-find-bugs-cli` sweep 7) and fixed 2026-09-25 — moved to `done/`. `core.ErrNotFound` now reads `not found`, store misses name their entity through `store.MapError` (`project not found`, `deploy not found`), and deploys lookups return `deploy not found`; status codes and `id: not_found` are unchanged.

> `020.md` filed 2026-09-23 (`/qa-find-bugs-cli` sweep 6) and fixed 2026-09-25 — moved to `done/`. A build tail with no pod now waits while its deploy row is still `created`/`queued`/`build_in_progress`, and the WebSocket path ends a refused tail with a Log-shaped `==> no running build is available to follow` line instead of closing silently.

> `019.md` filed 2026-09-23 (`/qa-find-bugs-cli` sweeps 5 and 8) and fixed 2026-09-25 — moved to `done/`. `blueprints validate` now reports every per-service semantic refusal (not the first), each located to its field or `envVars[i]` entry with line/column, and never with an inner `bad request:`; apply-path errors are unchanged. Compiler-plus-semantic aggregation (`b.yaml`) was left out as the note allowed.

> `022.md` filed 2026-09-23 (`/qa-find-bugs-cli` sweep 7) and fixed 2026-09-25 — moved to `done/`. An `imageUrl` trigger (`deploys create --image`, deploy hook `?imgURL=`, GraphQL/MCP) patched only the CR's projector-owned `spec.image`, so the projector restored the old image and the deploy closed `canceled`; `triggerFetched` now writes the row first, like `Rollback`.

> `018.md` filed and implemented 2026-09-15 from the live CLI QA sweep (`/qa-find-bugs-cli`, sweep 4) — moved to `done/`. Persistent state (`installation_id`, notice marker) lived in `~/.render/state` because `BEX_CLI_CONFIG_DIR` was mapped to `RENDER_CLI_CONFIG_PATH`. Now DIR maps to `RENDER_CLI_CONFIG_DIR` (default `$HOME/.bex`); new installs mint a bex-owned id rather than copying Render's.

> `017.md` filed and implemented 2026-09-15 from the live CLI QA sweep (`/qa-find-bugs-cli`, sweep 3) — moved to `done/`. `RENDER_CLI_CONFIG_DIR` was silently ignored because the launcher always pinned `RENDER_CLI_CONFIG_PATH` to `~/.bex/cli.yaml`. Option A: honor DIR (explicit `RENDER_*` wins); never rewrite the real credential file.

> `016.md` filed and implemented 2026-09-15 from the live CLI QA sweep (`/qa-find-bugs-cli`, sweep 2) — moved to `done/`. Four checklist rows named `ea sandbox <verb>`; the pinned CLI's group is plural (`sandboxes`, no alias). Ledger strings + pin-bump name re-diff + `ErrNoWorkspace` residual recorded; capability grades unchanged.

> `015.md` filed and implemented 2026-09-15 from the live CLI QA sweep (`/qa-find-bugs-cli`) — moved to `done/`. `bex logs` panics (exit 2, raw Go stack trace) with no usable credential in every non-interactive output mode: upstream `cmd/logs.go` calls `deps.LogLoader()` in `RunE` before auth, and `(*Dependencies).APIConfig` (`pkg/dependencies/dependencies.go:281`) does `panic(err)` on `DefaultAPIConfig`'s `ErrLogin`. Upstream defect, not bex (no request is ever sent); reproduced live against the v2.27.0 pin (`panic: run \`render login\` to authenticate`, exit 2; `services`control is a clean`Error:`, exit 1). Graded in `docs/cli-compatibility-checklist.md`: new Real-gaps bullet beside the `skills`panic,`logs`header row re-graded`[x]`→`[~]`, both bullets pin-conditional (re-check on every pin move). The note's item 3 — a launcher-owned `recover()`around`cmd.Execute()` — was explicitly **not** implemented (needs a user decision: it would mask genuine panics).

> `013.md`/`014.md` filed and implemented 2026-09-15 from the triage of the security scan at `~/.local/state/bex-security-integration/20260915T200437Z` — moved to `done/`. `013` collapsed the operator's two disagreeing answers about `BEX_REGISTRY` transport into one exported `registry.ClusterLocal` (the build plane's predicate, moved next to `NormalizeBase`): a bare host now defaults to `https://` unless it is cluster-local, and `registry.CredentialedBase` fails closed rather than putting an Authorization header on a cleartext off-cluster wire (`ResolveDigest`, `ListTags`, and the `app_controller` repo teardown). `014` added the Standard Webhooks timestamp tolerance (`VerifyTolerance`, 5m, both directions) that `Verify` advertised but never applied, with the clock injected so the window itself is testable.

> Scan finding 1 (platform Argo CD `AppProject` wildcard resource whitelists) was **triaged and not filed**: `deploy/gitops/base/appproject.yaml:1-7` documents the rationale — `sourceRepos` is the hard gate (the reviewed bex repo plus a digest-addressed OCI chart mirror), and the reviewed controller charts (cert-manager, CNPG, kpack, CAPI, OpenSandbox) genuinely need cluster-scoped CRDs/webhooks/RBAC. The stated precondition is write access to the platform GitOps source, which already owns the platform regardless of the whitelist; no tenant-reachable path authors Applications under this project. Enumerating kinds for a dozen upstream charts would also break on every chart upgrade. Accepted residual.

> `010.md`/`011.md`/`012.md` implemented 2026-09-08 (outbound-IPs UI; narrow plan aliases; datastore DeletionStalled) — moved to `done/`. `001.md` retired 2026-08-16 after its production gate found only 1–2 represented workspaces, zero build minutes, a 1.46 TiB sole complete egress sample, and remaining hourly gaps — no evidence-safe cap can be selected. `007.md` retired after its one useful artifact was captured: the production unmodified-CLI validation re-grade is now in `docs/cli-compatibility-checklist.md`; the note mixed that finished evidence chore with disposable deployment experiments and an unrelated dev-host suggestion, so it is not retained as roadmap work. `008.md` implemented and moved to `done/` (env groups in `resources[]` + action plans). `009.md` implemented and moved to `done/` (ADR018 deliberate divergences). `002.md` promoted to **m9** 2026-07-13; note moved to `done/`. `004.md` (KeyValue `maxmemoryPolicy` underscore-vs-hyphen, filed by `w9/m2`'s Render CLI compatibility walk) fixed 2026-07-15 (`dfff3034`), re-verified live end to end (create/list/get/update/suspend/resume/delete) — note moved to `done/`. `005.md` (Postgres owner/options wire-shape, filed by `w9/m2`) retired 2026-07-15 — a parallel session independently found the same gap across Postgres/Service/KeyValue and filed it as `w6/016`; `005.md` moved to `done/` pointing there rather than duplicating it.

- [x] **104** — [Blueprint numeric env values reject unsigned 64-bit integers](done/104.md) — **DONE 2026-10-15:** compiler809472bdc; sweep286 passes all16 two-client production validation targets/controls, completing the parked gate.

- [x] **105** — [CLI compatibility checklist advertises cross-repository image overrides](done/105.md) — **DONE 2026-10-14** (checklist `--image` row corrected to the same-repository rule)
