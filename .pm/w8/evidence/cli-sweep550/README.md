# CLI sweep 550 — valid image-to-Git source switch

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at local `0d1ad2fcf`, 2026-10-19T11:19–11:24Z. One owned free web service was created, switched from an image source to a Git source, observed through the replacement build, deleted by exact ID, and verified absent (public URL `404`, deploy read `404`, list omission).

## Journey

1. Created `qa-20261019-5b6da0-src550` from `docker.io/ealen/echo-server:0.9.2`; it reached `live` and served the echo server response.
2. In one update, set `--repo https://github.com/bex-co/bex --branch main --root-directory examples/hello-go`. The response showed the saved Git source, `rootDir: examples/hello-go`, and Docker context `examples/hello-go`; `imagePath` was no longer the saved source.
3. Including `rootDir` dispatched a `service_updated` deploy. This matches the backend contract: `SetRootDir` bumps `spec.restartedAt` specifically so a root-directory change takes effect as a fresh build, while source switching itself is explicitly tested as `TestMCPUpdateServiceSwitchesSourceWithoutDeploying`.
4. During the build, the former echo image remained the serving body. Build logs proved the new declared subtree was selected (`RUN CGO_ENABLED=0 go build -trimpath -o /app .` from `examples/hello-go`). Once the deploy reached `live`, the public body changed to hello-go's default `OK`, the service read `Running` / `rev-2`, and the saved fields remained repo `bex-co/bex`, branch `main`, root `examples/hello-go`.

This also explains sweep 549 without a contradiction: sequential branch/repo savings can remain undeployed until a deploy-causing field or explicit deploy, whereas a root-directory change itself is a build-triggering input. `undeployedChanges` was absent for the source identity; that field is documented as saved runtime configuration/secret-source drift, and the operator's release-record comparison deliberately excludes source identity except for the no-repo image comparison.

## Cleanup

Deleted exact-owned `srv-dbavpggqabuc73f7t06g`; former URL and deploy reads returned `404`, and the post-delete service list did not contain it.

## Unrun

- Git-to-image valid switch in one update.
- Source switch with a private GitHub repository or webhook push after the switch.
- Production remained pinned to `f4e38371055f`; w8/129, w8/130, and w8/131 fixes are on local/origin `main` but were not deployed during this sweep.
