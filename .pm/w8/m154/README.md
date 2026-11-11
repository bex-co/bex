# w8 · m154 — A native-runtime service finds its secret files in its root directory, as on Render

**Worker:** worker8 **Goal:** a Node, Python, Ruby, Go, Rust or Elixir service (web, private, worker, cron, and its pre-deploy command) reads each secret file both at `/etc/secrets/<name>` and at `<service root>/<name>`, as Render documents; Docker and image services keep `/etc/secrets` only. **Status:** todo

## Tasks (in order)

| id   | title                                                               | est | depends_on |
| ---- | ------------------------------------------------------------------- | --- | ---------- |
| t001 | Decide the mechanism and the collision rule, record them in ADR013  | 30m | —          |
| t002 | Native services see each secret file in the service root at runtime | 60m | t001       |
| t003 | Blast radius: cron, worker, pre-deploy, `rootDir`, env-group files  | 40m | t002       |
| t004 | Render parity                                                       | 20m | t003       |
| t005 | Simplify                                                            | 15m | t004       |
| t006 | Test coverage                                                       | 40m | t004       |
| t007 | Closeout (live replay on production)                                | 20m | t006       |

## Definition of done

1. On production, with the released `bex` CLI and a human login:

   ```text
   printf 'TOKEN=abc\n' > .env.qa ; printf '{"k":"v"}\n' > cfg.json
   bex services create --name <n> --type web_service --repo https://github.com/render-examples/express-hello-world \
     --runtime node --plan free --build-command "yarn install" \
     --start-command "sh -c 'pwd; sha256sum cfg.json .env.qa /etc/secrets/cfg.json /etc/secrets/.env.qa; node app.js'" \
     --secret-file cfg.json:./cfg.json --secret-file .env.qa:./.env.qa -o json --confirm
   bex logs -r <srv> --type app --limit 20 -o text
   ```

   The log shows the working directory, and four checksum lines: the root copies match the `/etc/secrets` copies and the local files.

2. The same service with `rootDir` set reads the files from that directory (its service root), not from the repository root.
3. A secret file from a linked environment group appears in the service root too.
4. Changing a secret file's content and deploying changes both copies; deleting it removes both.
5. An image-backed service and a Dockerfile service still have the files only under `/etc/secrets` (control).
6. Fixtures deleted afterwards.

## Finding (2026-11-11 UTC, `/qa-find-bugs-cli` w8 loop sweep `45ef`)

- **Severity:** major — an app that reads an uploaded `.env`, credentials JSON or key file from its working directory (the usual reason to upload one) starts without it on bex and with it on Render. Nothing reports the difference.
- **Versions:** released `bex v0.5.0` (Render CLI v2.35.0, `24ac2d60fdb2`); repo HEAD `60260d64e`; production platform pin `2f1c62c93f49`.
- **Context:** `https://api.bex.co/v1/`, workspace `bex-canary` (Scale), QA human device login, non-TTY, `-o json`.
- **Repro:**

  ```text
  $ bex services create --name qa-20261110-45ef-web --type web_service --repo https://github.com/render-examples/express-hello-world --branch main \
      --runtime node --plan free --build-command "yarn install" \
      --start-command "sh -c '…; for f in /etc/secrets/*; do echo \"SECRET-45ef \$f \$(sha256sum < \$f | cut -c1-64) \$(wc -c < \$f)\"; done; ls app.conf cfg.json 2>&1 | sed \"s/^/ROOT-45ef /\"; node app.js'" \
      --secret-file app.conf:sf/app.conf --secret-file cfg.json:sf/cfg.json --secret-file empty.txt:sf/empty.txt --secret-file big.txt:sf/big.txt -o json --confirm
  → srv-dbpvqf2p92ls73cjiqsg, deploy dep-dbpvqf2p92ls73cjiqt0 live

  $ bex logs -r srv-dbpvqf2p92ls73cjiqsg --type app --limit 40 -o text
  05:38:17  SECRET-45ef /etc/secrets/app.conf ee16e993…184123a 107          (all four files: sha256 and size equal the local files)
  05:38:17  SECRET-45ef /etc/secrets/cfg.json 97e4abee…ba28c4 22
  05:38:17  ROOT-45ef ls: cannot access 'app.conf': No such file or directory
  05:38:17  ROOT-45ef ls: cannot access 'cfg.json': No such file or directory
  05:38:19  Example app listening on port 10000!                            (same shell, same directory: `node app.js` found app.js)
  ```

- **Expected (Render):** "At runtime, the secret file is available at `/etc/secrets/<filename>`. For non-Docker services, the file is _also_ available in your service's root directory." (render.com/docs/configure-environment-variables § Secret files, read 2026-11-11). For Docker services Render documents `/etc/secrets/<filename>` at runtime, and at build time "Render also copies your secret files into the root of your Docker build context" (render.com/docs/docker-secrets).
- **Actual:** `/etc/secrets/<name>` only, for every runtime.
- **Attribution:** infrastructure/runtime (operator pod projection). The CLI request is upstream's and the files are stored and mounted byte for byte.
- **Root cause:** `lego/operator/internal/controller/app_controller.go:5787-5808` (`secretFileMounts`) builds one projected volume and one mount at `secretFilesMountPath = "/etc/secrets"` (`:5725`). Its three callers (`deployment_projection.go:645`, `app_controller.go:5523`, `app_controller.go:6920`) add nothing else. A native image's working directory is `/opt/render/project/src` (`lego/operator/internal/build/native.go:334`), so the service root is that path plus `rootDir`.
- **Fix options (t001 chooses):**
  - **A — per-file mounts:** for native runtimes, add one read-only `subPath` mount per file of the same projected volume at `<service root>/<name>`. No copy of secret bytes into the image or a writable layer. A subPath mount does not follow later Secret updates, which is fine because a secret-file change already needs a deploy.
  - **B — link at start:** the native start wrapper symlinks `/etc/secrets/*` into the working directory before running the start command.
  - Either way decide what happens when the repository already has a file of that name (Render does not document it; the secret file winning is the safe reading), and keep names that are not plain file names out of the root.
- **Blast radius:** web, private service, background worker, cron job, the pre-deploy Job (`internal/predeploy/predeploy.go:130` carries the same projection), linked environment-group files, and services with a `rootDir`. Native **builds** already read the files at `/etc/secrets/<name>` (w4/m163); Render documents no root copy for native builds, so builds stay as they are unless t004 finds otherwise. Docker builds: Render copies the files into the build context root; whether bex does was not tested here (follow-up if not).
- **Unverified:** Render's behavior was read from its documentation, not run on a Render account. Worker, cron and pre-deploy were not exercised (paid plans); `rootDir` and environment-group files were not exercised.
- **Also observed, not part of this milestone:** `ls -la /etc/secrets` shows the kubelet's `..data` and `..<timestamp>` entries beside the files; a loader that walks the directory sees them.
- **Dedupe:** `rg -i "service's root directory|also available in your service|secret files.*root"` over `.pm` and `docs` finds no item. ADR018's secret-files row cites the same Render page for "runtime paths only" and does not mention the root-directory sentence. w4/m163 (done) covers build-time `/etc/secrets`.
- **Cleanup:** the fixture service is deleted with the sweep.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` w8 loop, sweep `45ef`, 2026-11-11 UTC (finding above).
- **Goal linkage:** Render parity for a service's runtime environment (ADR008 § Render parity first, ADR013): a Render app moved to bex unchanged must find its files where it left them.
- **Expected outcome:** apps that load `./.env` or `./credentials.json` uploaded as a secret file work unchanged.
- **Why now:** the CLI's `--secret-file` and the dashboard upload both present the feature as Render's; the missing copy fails silently at app start.
- **Render parity included:** the change is observable runtime behavior behind REST/GraphQL/MCP/dashboard secret files.
