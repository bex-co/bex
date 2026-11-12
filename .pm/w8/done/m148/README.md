# w8 · m148 — Static-site builds keep working when GitHub release downloads fail

**Worker:** worker8 **Goal:** a native static-site build no longer fetches Hugo from `github.com/gohugoio/hugo/releases` on each build. When a platform toolchain fetch does fail, the deploy says which download failed and how, not `digest mismatch sha256:…`. **Status:** done (2026-11-12: t001–t006 shipped 2026-11-10; t007's production replay built, published and served a free static site)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Serve the pinned Hugo tarballs from bex-controlled supply — **DONE** | 90m | — |
| [t002](done/t002.md) | A failed platform toolchain fetch names the download and its HTTP status — **DONE** | 45m | — |
| [t003](done/t003.md) | Blast radius: every third-party fetch a native build makes — **DONE** | 30m | t001 |
| [t004](done/t004.md) | Render parity — **DONE** | 20m | t002, t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 40m | t004 |
| t007 | Closeout — **DONE** (live static deploy DoD) | 20m | t006 |

## Definition of done

1. With egress to `github.com` release downloads blocked for a builder (or the Hugo stage's GitHub URL pointed at a 504 stub in a test build), `bex services create --type static_site … --publish-directory out` followed by `bex deploys create <id> --wait --confirm -o json` reaches `live`, and the site answers 200 with its `index.html`.
2. The built image's `hugo version` still reports `0.111.3 … extended` and its tarball sha256s are unchanged (`b382aacb…` amd64, `69517641…` arm64).
3. A build whose platform toolchain fetch fails (forced in a test) ends `build_failed` with a `deploy_ended.failureReason` such as `bex could not download its Hugo toolchain (HTTP 504 from github.com); this is a platform problem — retry the deploy`, and never with a bare `digest mismatch`.

## Finding

### Every native static-site build fails with `digest mismatch` while GitHub's release downloads answer 504

- **Severity:** major (a blocker while it lasted, from at least 09:45Z to 11:24Z on 2026-11-08, ended by about 11:58Z). During such a window no static site can be created or redeployed on bex. Sites that do not use Hugo fail too, because the Hugo stage is part of every native static build. It was not only GitHub's public outage: after GitHub recovered for the public internet, bex's builders kept failing (see the post-recovery evidence below). Recovery: sweep `4d90`'s fresh static site `srv-dbo678fkf7pc73a353vg` went `live` at 11:59:48Z and served its `index.html` with 200.
- **Versions:** checkout build `bex vqa-460c283a6` (HEAD `460c283a6`), Render CLI v2.35.0 `24ac2d60fdb2`. Built commits `1aad339` and later on `main`. Deployed revision unknown.
- **Context:** 2026-11-08 09:45–09:54Z, `https://api.bex.co/v1/`, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`), human OAuth (workspace owner's CLI login), non-TTY. `/qa-find-bugs-cli` sweep `a6b6`.
- **Repro:**

  ```sh
  bex services create --name qa-<nonce>-static --type static_site --repo https://github.com/bex-co/bex --branch main \
    --root-directory examples/hello-go \
    --build-command "mkdir -p out/sub && echo static-<nonce> > out/index.html && echo sub > out/sub/index.html" \
    --publish-directory out -o json
  # deploy dep-dbo49hs0r0ts73a72te0 → build_failed at 09:47:07Z; every path on the site answers 503
  bex deploys create <id> --wait --confirm -o json   # dep-dbo4cu340q1c73bmudrg → build_failed, exit 1, 79 s
  bex logs -r <id> --type build -o text --limit 100
  ```

  Build log (tail) and event, identical on both builds:

  ```text
  #15 [bex-hugo-amd64 1/1] ADD --checksum=sha256:b382aacb522a470455ab771d0e8296e42488d3ea4e61fe49c11c32ec7fb6ee8b https://github.com/gohugoio/hugo/releases/download/v0.111.3/hugo_extended_0.111.3_linux-amd64.tar.gz /hugo.tar.gz
  #15 ERROR: digest mismatch sha256:bef140a1a96994029153dca8c00b1750b9a5a764fb9db2dc68d7bb40e8a29e8a: sha256:b382aacb522a470455ab771d0e8296e42488d3ea4e61fe49c11c32ec7fb6ee8b
  ==> Build failed: build failed in the docker build step:
  ==> error: failed to solve: digest mismatch sha256:bef140a1…: sha256:b382aacb…

  deploy_ended {"deployStatus":"failed","fullDeployStatus":"build_failed","failureReason":"build failed in the docker build step:\nerror: failed to solve: digest mismatch sha256:bef140a1…: sha256:b382aacb…"}
  ```

- **External evidence (same window, from the QA host):** `GET https://github.com/gohugoio/hugo/releases/download/v0.111.3/hugo_extended_0.111.3_linux-amd64.tar.gz` answered `504` with a 92-byte HTML body four times between 09:48 and 09:52Z (`x-github-request-id: 8E72:E86D4:E7FA9D:114B541:6AF04615`). The sha256 of that body is `bef140a1…`, exactly the digest BuildKit received. The arm64 tarball and `cli/cli`'s v2.60.0 tarball also answered 504. Small assets (`hugo_0.111.3_checksums.txt`) and `github.com/` answered 200. githubstatus.com reported "All Systems Operational". The same URL was still `504` at 10:03, 10:17:05 and 10:17:18Z, so the static-build outage lasted at least 90 minutes. The release API lists both tarballs with their usual sizes (18278686 / 16778397 B), so the pinned checksums are correct and the outage is GitHub's.
- **Post-recovery evidence (sweep `8c72`, 11:19–11:24Z):** from the QA host the same URL answered `200` with the genuine tarball at 11:19:09, 11:23:53 and 11:23:54Z (18278686 B, sha256 `b382aacb…`). A fresh static site `srv-dbo5lenkf7pc73a353mg` (`qa-20261108-8c72-static`, created 11:19:22Z) still ended `build_failed` at 11:23:47Z. Its log shows the `bex-hugo-amd64` `ADD` attempted several times (11:21:37–11:21:38) and then `#15 ERROR: digest mismatch sha256:839488ebc08446a096a893996ed23eac321ac166724cd8c5d9092057834d2d79: sha256:b382aacb…`. A different wrong body (`839488eb…`, not the 504 page's `bef140a1…`) reached the builder while the public internet got the file. That suggests GitHub treats bex's builder egress differently, for example rate-limiting a shared datacenter address. Unverified: the builder never logs the HTTP status, which t002 must surface. The bex static-build outage therefore spans at least 09:45Z→11:24Z.
- **Attribution:** infrastructure/runtime (bex build supply chain). The trigger is external, but the hard dependency is bex's choice.
- **Root cause:** `lego/operator/internal/build/nativehugo.go:44-57` `nativeStaticTools` emits `ADD --checksum=<sha> https://github.com/gohugoio/hugo/releases/download/…` for every native static build (`if !o.StaticSite` is the only gate). The comment says it is "fetched once", but two consecutive builds both refetched, so a cached copy did not save them (whether BuildKit's cache key avoids the request is unverified). The BuildKit 504 body becomes a checksum error, and `failureReason` passes it through verbatim.
- **Fix:** (t001) publish the two pinned tarballs, or a `FROM scratch` image holding the unpacked `hugo` binary, to bex-controlled storage that builders already depend on (the platform registry). Build it in CI from the same checksums, and switch the stage to `COPY --from=<registry>/…@sha256:<digest>`, keeping the per-arch checksums inventoried in `toolchain-freshness.json`. (t002) map BuildKit errors from platform-owned stages (`bex-hugo-*`, `bex-pnpm-*`, etc.) to a named platform-download failure in `failureReason`, keeping the raw line in the build log.
- **Blast radius:** `nativehugo.go` (2 GitHub URLs). `nativepnpm.go:56,97-102` fetches from `registry.npmjs.org` (Node builds depend on npm anyway, but legacy pnpm runs on every Node build). `nativecommon_erlang.go:151` asserts an OTP GitHub URL in the base image Dockerfile. Media/busybox helper images come from Docker Hub. t003 records each with its need-at-build-time verdict.
- **Regression checks:** DoD 1–3; a Hugo quickstart site (`hugo --gc --minify`) still builds; compute builds get no Hugo stage.
- **Unverified:** whether BuildKit's `ADD --checksum` cache can satisfy a repeat build without a network request on bex's builders; how long the GitHub incident lasted; Render's own build behavior during a GitHub CDN outage (Render does not document fetching Hugo per build).
- **Dedupe:** `rg -i "digest mismatch|hugo"` over `.pm` open and done: only w8/m106 (added Hugo, done 2026-10-17) and unrelated registry-migration digest checks. No item covers the GitHub runtime dependency.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop for w8, sweep `a6b6`, 2026-11-08.
- **Goal linkage:** dependable hosting (ADR008), static sites at Render parity (ADR029). The w8/m106 Hugo supply must not make static hosting depend on a third-party CDN per build.
- **Expected outcome:** static deploys keep working through GitHub release-asset incidents, and any remaining platform fetch failure is self-explanatory.
- **Why now:** it was observed live as a total static-build outage. The fix is local to the build image definition and error classification.
- **Render parity included:** `failureReason` is served on REST, GraphQL, MCP and the dashboard deploy page.

## Blocked — 2026-11-10

Everything but t007 is done and shipped. t007 replays the Definition of done on production, which needs (1) the scheduled 19:00 UTC deploy (production deploys run at 19:00/00:00/05:00 UTC since `4630a48a7`) and (2) a signed-in run. `scripts/qa-login.sh` is rejected by Kratos (rc 1 on 2026-11-10, as on 2026-11-08), so the QA password must be fixed or the user must re-authorize their own CLI login with run-owned `bex-canary` fixtures, as they did for the 2026-11-08 sweep.
