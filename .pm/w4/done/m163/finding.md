# Saved files never reach a native static build

Severity: **major**. Research against main `45fd1edd8`, live QA 2026-10-03, sweep 59. This files work only; no product fix is included.

## Reproduction and target

Use an owned Free static site, public repo `https://github.com/bex-co/bex`, branch main, root `examples/static-site`, publish directory `.`, auto-deploy off. Add harmless file `qa-r59.txt` with content `qa-r59-build-file-marker`; set Build Command to `cp /etc/secrets/qa-r59.txt index.html`.

Observed service `srv-db0bn6nfq78s73et1e1g`, name `qa-20261003-buildfile-r59`, resolved source `5c49673b2fda869c102aa02fb1bbf8d684ca6e60`. Initial deploy `dep-db0bn6nfq78s73et1e20` failed at 08:25:31Z with missing file. On a fresh signed-in page, REST, GraphQL reveal and MCP all returned the exact saved marker. Change Build Command through Settings to `printf 'qa-r59-control' > index.html`: deploy `dep-db0ons89ohsc73e5r8d0` reached Live; external curl and Playwright request both returned HTTP 200 and exactly `qa-r59-control`. Reload Settings and restore the original cp command: `dep-db0ooqffq78s73et1kp0` failed at 23:15:31Z with the same missing-file error. This isolates file delivery from repo checkout, shell, root directory, publish directory and static serving.

**Target:** the existing accepted secret-file configuration must be readable at `/etc/secrets/<name>` by a native build command, including static builds. The sample should deploy Live and return the marker. Do not normalize API reads to empty, automatically copy credentials into published output, or hide a build failure. The sample intentionally copies a non-secret marker to prove access.

The UI promises files mounted into this service at deploy time; static sites have no tenant runtime after extraction. Current storage success therefore does not fulfill the offered configuration. This is an uncovered input omission, not proof that a previously working build-file feature regressed.

## Root cause and consumer trace

- `lego/operator/internal/controller/app_controller.go:1348-1365`: implicit static build commands choose BuilderNative; six native language runtimes use the same path.
- `app_controller.go:889-898,918-930,1415-1469`: native dispatch only projects effective environment variables with projectNativeBuildEnv and passes RuntimeEnvSecret/NativeEnvRevision. It never passes effective file sources.
- `lego/operator/internal/build/build.go:310-351,865-871,938-944`: Options has no file input; only render-env enters the native buildctl secret transport and native runtime-env volume.
- `lego/operator/internal/build/native.go:115-139,163-181`: generated RUN mounts only render-env at /run/secrets/render-env; the preparer mounts only its output and environment Secret. The captured build log prints that exact RUN followed by cp's missing-file error. Empty env revision `none` is expected because this fixture has files but no env vars; it is not evidence the file store lost data.
- `app_controller.go:4466` implements runtime secretFileMounts. Exhaustive production call search finds **three** call sites: cronPodSpec at :4257, deployment projection at `deployment_projection.go:421`, and pre-deploy projection at :5237. None is a build RUN. The earlier sweep 56 native-Go runtime marker journey passed; its file access is explained by runtime projection, not build delivery.
- `lego/types/v1alpha1/app_types.go:582-589` can already represent ordered file references. `app_identity.go:81` currently classes FilesFromSecrets as release-only; `controller/release_identity.go:53-72,110-150` artifact input lacks the file list. Adding transport alone would leave direct-reference changes/cache reuse underspecified.
- `build/artifacts.go:125` and `app_controller.go:6106` enumerate current build artifact cleanup and known Secret names. Any added projection must participate in both and preserve App UID lifetime checks.

**Framework checked:** BuildKit image is pinned to v0.30.0 in build.go:69; generated Dockerfile selects docker/dockerfile:1.7. Opened [BuildKit v0.30.0 dispatchSecret](https://github.com/moby/buildkit/blob/v0.30.0/frontend/dockerfile/dockerfile2llb/convert_secrets.go) and [frontend 1.7.0 dispatchSecret](https://github.com/moby/buildkit/blob/dockerfile/1.7.0/frontend/dockerfile/dockerfile2llb/convert_secrets.go). Both require explicit secret ID/target wiring; there is no automatic inheritance of a Kubernetes Secret or runtime volume into a Dockerfile RUN. The 1.7 tag is not digest-pinned in generated syntax; exact production frontend patch digest was not captured. The existing nativeEnvRevision implementation documents and handles secret-content cache exclusion; extend that discipline to files instead of trusting changed bytes to invalidate a secret mount.

## Fix scope and adjacent behavior

Use the effective ordered file sources for the in-flight release, with service-owned files last, existing group precedence and release snapshot selection. Merge into an isolated App-owned projection next to the build job. Transport each file through BuildKit secrets at its exact validated /etc/secrets target, preserving bytes, without persisting values in Dockerfile/context/image metadata. Include membership and effective-content changes in native artifact/cache decisions; reuse opaque revision design. Preserve save-only/rebuild semantics and cleanup all projections on delete.

One production EnsureBuild options construction and one nativeBuildPreparer call feed this builder; nativeDockerfile has one production caller (the preparer). Scope is BuilderNative globally: web, static, cron, worker and private services. Static is the live failing case; native web/cron/worker/private build-file failures are source-inferred and require verification. Postgres/Key Value have no App native build. Dockerfile, CNB, prebuilt-image, and no-build static paths are controls, not silently expanded features.

Existing input aliases converge on App refs: REST service create and secret-file endpoints plus coherent /environment patch; GraphQL createService, setSecretFile/deleteSecretFile and patchServiceEnvironment; MCP service-create, set_secret_file/delete_secret_file and patch_service_environment; dashboard create and Environment editor; environment-group link/content paths and direct App refs. Preserve shared backend authorization and masking. Sensitive reads still require can_view_sensitive; manage writes require can_create. Unauthenticated/forbidden/not-found behavior must remain unchanged, with no cross-workspace existence disclosure. Protected or foreign build Secrets remain refused. Match current runtime optional-source behavior; infrastructure read failures/timeouts must fail explicitly rather than silently omit readable required inputs. Do not broaden access to clone, registry or signing credentials.

Pre-settle: existing live static output remains served while replacement builds and after failure; service Running plus latest deploy Build Failed was correct in the repeat screenshot. Pending builds must not become Live merely because a file projection was created.

## Render, precedent and deduplication

[Render's official Django migration example](https://render.com/blog/migrate-django-from-heroku#help-a-helper-file-for-derived-variables) reads its secret file from scripts used in both native build and start contexts. [Current environment docs](https://render.com/docs/configure-environment-variables#secret-files) explicitly describe runtime paths, not static build execution. No authenticated Render build was run, and static upstream behavior is **unverified**. The bex filing also rests on its own accepted static-file configuration and mount promise; do not claim the Render docs explicitly promise static build mounts. The separate Build Sources product is not evidence for this service path.

Searched all open, blocked and done PM files for secret-file/build/static combinations, FilesFromSecrets, /etc/secrets and native input terms; scanned open milestone titles across all workstreams, read DO_NOT_DO, last 40 dashboard/lego commits, native build history and `git log -S'FilesFromSecrets' -- lego/operator/internal/build` (no implementation). Current main retains omission. No anti-goal or explicit native-build-file exclusion found.

Precedent DoD dispositions:

- **w1/done/m16:** group create/vars/files/link remains valid runtime work; per-service /etc/secrets projection works at runtime (sweep56), but does not cover this native build. REST/GQL/MCP/UI exposure passes readback here; Render parity is only partially substantiated for builds. Original make test/dashboard gates were not rerun by this PM-only filing. Treat as residual scope gap.
- **w7/done/m41:** static create build field/payload, settings editor and command round-trip are demonstrated by successful control and failed cp runs; included/ignored build-filter creation and regression gates were not retested. This is not a field-wiring regression.
- **w4/done/m93:** group env at build/runtime, own env precedence, cleanup and native env transport concern env variables. All three DoD bullets remain outside the new file-input omission; no claim to reverify them in sweep59. Reuse its ownership and ordering pattern.
- **w7/done/m87:** changed MESSAGE cache behavior; literal/group/env precedence/removal/byte preservation; no secret disclosure; cache-disabled/source failure behavior; and pinned-image integration gates all concern existing env inputs. None proves file cache correctness. Preserve every class as regression coverage when extending inputs; not revalidated here.
- **w1/done/m152 / w5/done/m107:** release snapshots and retained saved settings must remain intact; do not restore superseded rollback-write behavior. Sweep56 showed runtime file save-only/restart/deploy/rollback controls pass.
- **w4/blocked/m155** addresses serving phase after failed publish, not missing build files; current repeat correctly retained Running. **w5/blocked/m105** command setting effectiveness passed the control. **w4/blocked/m145** pending-config notice is separate.
- w9/done/m89 and m92 are evidence/observable-DoD precedent, not duplicate hosting findings.

Unverified: native non-static build-file execution, group-file build precedence, filename edge cases, cache transitions, multi-namespace file projection, Docker/CNB build-file parity, and authenticated Render static behavior. Tasks cover verification without presenting those as observed failures.

## Evidence and cleanup

Verified local artifacts: `.playwright-mcp/qa-buildfile-r59-failed.png`, `qa-r59-api-captures.json`, `qa-r59-console.txt`, `qa-r59-network.txt` under the same directory. Screenshot shows exact failed cp and retained Running state. Full repeat accessibility/log text follows the wire evidence below. Initial create request is not retained; do not pretend the later readback is its response. Initial deploy REST result is retained.

Session expiry caused the initial settings redirect on continuation; renewed login resolved it. One dashboard-origin fetch to tenant origin failed CORS; curl and Playwright request were successful controls. Intentional post-delete GET404 and these own probes are not new bugs.

Deleted the site through UI (deleteService true), confirmed REST404; inspected 1,367 cluster objects across all namespaces, matching name, service ID and App UID `b3f7537d-5a65-4b5a-8a99-60a185634bad`: none remained. Revoked expired and renewed QA sessions, cleared browser cookies, removed local session handles. No foreign resources changed.

## Exact requests and complete responses

Authenticated browser requests use the session cookie without recording it. GraphQL POST requests below went to https://api.bex.co/graphql with application/json; MCP to https://api.bex.co/mcp with Accept application/json, text/event-stream. Test content is deliberately public and non-secret.

```json
[
  {
    "request": [
      {
        "operationName": "SetBuildCommand",
        "variables": {
          "id": "srv-db0bn6nfq78s73et1e1g",
          "command": "printf 'qa-r59-control' > index.html"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation SetBuildCommand($id: String!, $command: String!, $confirm: String) {\n  setBuildCommand(id: $id, command: $command, confirm: $confirm) {\n    id\n    buildCommand\n    phase\n    __typename\n  }\n}"
      }
    ],
    "status": 200,
    "body": "[{\"data\":{\"setBuildCommand\":{\"__typename\":\"Service\",\"buildCommand\":\"printf 'qa-r59-control' \\u003e index.html\",\"id\":\"srv-db0bn6nfq78s73et1e1g\",\"phase\":\"Failed\"}}}]\n"
  },
  {
    "request": [
      {
        "operationName": "SecretFileContent",
        "variables": {
          "id": "srv-db0bn6nfq78s73et1e1g",
          "name": "qa-r59.txt"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query SecretFileContent($id: String!, $name: String!) {\n  service(id: $id) {\n    id\n    secretFile(name: $name) {\n      id\n      name\n      content\n      __typename\n    }\n    __typename\n  }\n}"
      }
    ],
    "status": 200,
    "body": "[{\"data\":{\"service\":{\"__typename\":\"Service\",\"id\":\"srv-db0bn6nfq78s73et1e1g\",\"secretFile\":{\"__typename\":\"SecretFile\",\"content\":\"qa-r59-build-file-marker\",\"id\":\"qa-r59.txt\",\"name\":\"qa-r59.txt\"}}}}]\n"
  },
  {
    "request": [
      {
        "operationName": "SetBuildCommand",
        "variables": {
          "id": "srv-db0bn6nfq78s73et1e1g",
          "command": "cp /etc/secrets/qa-r59.txt index.html"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation SetBuildCommand($id: String!, $command: String!, $confirm: String) {\n  setBuildCommand(id: $id, command: $command, confirm: $confirm) {\n    id\n    buildCommand\n    phase\n    __typename\n  }\n}"
      }
    ],
    "status": 200,
    "body": "[{\"data\":{\"setBuildCommand\":{\"__typename\":\"Service\",\"buildCommand\":\"cp /etc/secrets/qa-r59.txt index.html\",\"id\":\"srv-db0bn6nfq78s73et1e1g\",\"phase\":\"Running\"}}}]\n"
  },
  {
    "request": {
      "method": "GET",
      "url": "https://api.bex.co/v1/services/srv-db0bn6nfq78s73et1e1g/secret-files/qa-r59.txt"
    },
    "status": 200,
    "body": "{\"name\":\"qa-r59.txt\",\"content\":\"qa-r59-build-file-marker\"}\n"
  },
  {
    "request": {
      "method": "GET",
      "url": "https://api.bex.co/v1/services/srv-db0bn6nfq78s73et1e1g/deploys/dep-db0ooqffq78s73et1kp0"
    },
    "status": 200,
    "body": "{\"id\":\"dep-db0ooqffq78s73et1kp0\",\"serviceId\":\"srv-db0bn6nfq78s73et1e1g\",\"status\":\"build_failed\",\"trigger\":\"config_change\",\"commit\":{\"id\":\"5c49673b2fda869c102aa02fb1bbf8d684ca6e60\",\"message\":\"fix(billing): close w3/m172 with live reclaim evidence\\n\\nThe live audit before rollout found 13 cardless bex Customers (2 opened\\nCheckout, 11 minted without a recorded checkout) beside 4 bound ones and\\n2 deleted-workspace tombstones. Two reclaimer passes after 8f6a69e9e\\nreached prod took the 13 to 0 and left the bound Customers and\\ntombstones untouched (128 -\\u003e 115 Customers account-wide).\\n\\nThe audit now buckets Customers without a bex tag as not_bex instead of\\ncounting them as bound: the Stripe account is shared, and 109 of its\\nCustomers belong to another product.\"},\"createdAt\":\"2026-10-03T23:14:49.583141Z\",\"updatedAt\":\"2026-10-03T23:15:31.970001Z\",\"startedAt\":\"2026-10-03T23:15:01.98983Z\",\"finishedAt\":\"2026-10-03T23:15:31.970001Z\",\"failureReason\":\"build failed in the docker build step:\\n0.089 cp: cannot stat '/etc/secrets/qa-r59.txt': No such file or directory\\n------\\nerror: failed to solve: process \\\"/bin/bash -c : bex-native-env-rev=none\\\\nwhile IFS= read -r record; do\\\\n  [ -n \\\\\\\"$record\\\\\\\" ] || continue\\\\n  key=${record%%=*}\\\\n  encoded=${record#*=}\\\\n  # The sentinel keeps command substitution from stripping value newlines.\\\\n  value=\\\\\\\"$(printf '%s' \\\\\\\"$encoded\\\\\\\" | base64 -d 2\\u003e/dev/null \\u0026\\u0026 printf '.')\\\\\\\" || exit 1\\\\n  export \\\\\\\"$key=${value%.}\\\\\\\"\\\\ndone \\u003c /run/secrets/render-env\\\\ncp /etc/secrets/qa-r59.txt index.html\\\" did not complete successfully: exit code: 1\"}\n"
  },
  {
    "request": {
      "jsonrpc": "2.0",
      "id": 59,
      "method": "tools/call",
      "params": {
        "name": "get_secret_file",
        "arguments": {
          "serviceId": "srv-db0bn6nfq78s73et1e1g",
          "name": "qa-r59.txt"
        }
      }
    },
    "status": 200,
    "body": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":59,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"content\\\":\\\"qa-r59-build-file-marker\\\",\\\"name\\\":\\\"qa-r59.txt\\\"}\"}],\"structuredContent\":{\"content\":\"qa-r59-build-file-marker\",\"name\":\"qa-r59.txt\"}}}\n\n"
  },
  {
    "request": {
      "method": "GET",
      "url": "https://api.bex.co/v1/services/srv-db0bn6nfq78s73et1e1g/deploys/dep-db0bn6nfq78s73et1e20"
    },
    "status": 200,
    "body": "{\"id\":\"dep-db0bn6nfq78s73et1e20\",\"serviceId\":\"srv-db0bn6nfq78s73et1e1g\",\"status\":\"build_failed\",\"trigger\":\"create\",\"commit\":{\"id\":\"5c49673b2fda869c102aa02fb1bbf8d684ca6e60\",\"message\":\"fix(billing): close w3/m172 with live reclaim evidence\\n\\nThe live audit before rollout found 13 cardless bex Customers (2 opened\\nCheckout, 11 minted without a recorded checkout) beside 4 bound ones and\\n2 deleted-workspace tombstones. Two reclaimer passes after 8f6a69e9e\\nreached prod took the 13 to 0 and left the bound Customers and\\ntombstones untouched (128 -\\u003e 115 Customers account-wide).\\n\\nThe audit now buckets Customers without a bex tag as not_bex instead of\\ncounting them as bound: the Stripe account is shared, and 109 of its\\nCustomers belong to another product.\"},\"createdAt\":\"2026-10-03T08:23:54.057098Z\",\"updatedAt\":\"2026-10-03T08:25:31.959937Z\",\"startedAt\":\"2026-10-03T08:25:01.980349Z\",\"finishedAt\":\"2026-10-03T08:25:31.959937Z\",\"failureReason\":\"build failed in the docker build step:\\n0.138 cp: cannot stat '/etc/secrets/qa-r59.txt': No such file or directory\\n------\\nerror: failed to solve: process \\\"/bin/bash -c : bex-native-env-rev=none\\\\nwhile IFS= read -r record; do\\\\n  [ -n \\\\\\\"$record\\\\\\\" ] || continue\\\\n  key=${record%%=*}\\\\n  encoded=${record#*=}\\\\n  # The sentinel keeps command substitution from stripping value newlines.\\\\n  value=\\\\\\\"$(printf '%s' \\\\\\\"$encoded\\\\\\\" | base64 -d 2\\u003e/dev/null \\u0026\\u0026 printf '.')\\\\\\\" || exit 1\\\\n  export \\\\\\\"$key=${value%.}\\\\\\\"\\\\ndone \\u003c /run/secrets/render-env\\\\ncp /etc/secrets/qa-r59.txt index.html\\\" did not complete successfully: exit code: 1\"}\n"
  },
  {
    "request": [
      {
        "operationName": "DeleteService",
        "variables": {
          "id": "srv-db0bn6nfq78s73et1e1g"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation DeleteService($id: String!, $confirm: String) {\n  deleteService(id: $id, confirm: $confirm)\n}"
      }
    ],
    "status": 200,
    "body": "[{\"data\":{\"deleteService\":true}}]\n"
  },
  {
    "request": {
      "method": "GET",
      "url": "https://api.bex.co/v1/services/srv-db0bn6nfq78s73et1e1g"
    },
    "status": 404,
    "body": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
  }
]
```

External public control: `curl -sS --max-time 20 -w '\nHTTP %{http_code}\n' https://qa-20261003-buildfile-r59.onbex.co` returned `qa-r59-control\nHTTP 200`. Independent Playwright GET:

```json
{
  "status": 200,
  "body": "qa-r59-control"
}
```

Repeat UI and complete captured log window:

```text
- main:
  - navigation "Breadcrumbs":
    - link "Projects":
      - /url: /
    - button "qa-20261003-buildfile-r59"
  - button "Search": Search ⌘ K
  - button "New"
  - button "Help and resources"
  - button "P"
  - text: Static Site
  - heading "qa-20261003-buildfile-r59" [level=1]
  - text: Service Running
  - 'link "Latest deploy: Build Failed"':
    - /url: /static/srv-db0bn6nfq78s73et1e1g/deploys/dep-db0ooqffq78s73et1kp0
    - text: Latest deploy Build Failed
  - button "Manual Deploy"
  - text: "Service ID: srv-db0bn6nfq78s73et1e1g"
  - button "Copy service ID"
  - link "github.com · bex-co / bex":
    - /url: https://github.com/bex-co/bex/tree/main
  - text: main
  - link "https://qa-20261003-buildfile-r59.onbex.co":
    - /url: https://qa-20261003-buildfile-r59.onbex.co
  - button "Copy service URL"
  - term: Slug
  - definition: qa-20261003-buildfile-r59
  - term: Revision
  - definition: rev-2
  - term: Created
  - definition:
    - time: 14h
  - text: Deploy status Build Failed Config Change dep-db0ooqffq78s73et1kp0
  - paragraph: "build failed in the docker build step: 0.089 cp: cannot stat '/etc/secrets/qa-r59.txt': No such file or directory ------ error: failed to solve: process \"/bin/bash -c : bex-native-env-rev=none\\nwhile IFS= read -r record; do\\n [ -n \\\"$record\\\" ] || continue\\n key=${record%%=*}\\n encoded=${record#*=}\\n # The sentinel keeps command substitution from stripping value newlines.\\n value=\\\"$(printf '%s' \\\"$encoded\\\" | base64 -d 2>/dev/null && printf '.')\\\" || exit 1\\n export \\\"$key=${value%.}\\\"\\ndone < /run/secrets/render-env\\ncp /etc/secrets/qa-r59.txt index.html\" did not complete successfully: exit code: 1"
  - paragraph: "5c49673 fix(billing): close w3/m172 with live reclaim evidence"
  - term: "Created:"
  - definition: October 3, 2026 at 4:14 PM
  - term: "Updated:"
  - definition: October 3, 2026 at 4:15 PM
  - term: "Started:"
  - definition: October 3, 2026 at 4:15 PM
  - term: "Finished:"
  - definition: October 3, 2026 at 4:15 PM
  - term: "Duration:"
  - definition: 29s
  - text: Status timeline
  - list:
    - listitem:
      - paragraph: Deploy created
      - time: October 3, 2026 at 4:14 PM
    - listitem:
      - paragraph: Deploy started
      - time: October 3, 2026 at 4:15 PM
    - listitem:
      - paragraph: Build Failed
      - time: October 3, 2026 at 4:15 PM
  - button "Log type": All logs
  - textbox "Search logs…"
  - button "Log options"
  - button "Maximize"
  - status:
    - text: Showing the newest 100 matching lines in this range — scroll up for older history.
    - button "Load older"
  - text: 04:15:21 PM
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: 04:15:21 PM
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "#7 [stage-0 1/4] FROM docker.io/library/node:24-bookworm@sha256:64af3819f9275802414d7cdc38c27e9d82bd564dec4d4da87d008255d36c63b4 04:15:21 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "#7 extracting sha256:496e07b192ff1c9265750d075a3387666375e1729f2a3a5c38e0b5cde05334c9 0.1s done 04:15:21 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "#7 extracting sha256:4cc81be23c065f7bd9f0e53015be3c099db194a9d930ac587fc53dcc5e0000a4 0.0s done 04:15:21 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "#7 DONE 16.3s 04:15:21 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: 04:15:21 PM
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "#8 [stage-0 2/4] WORKDIR /opt/render/project/src 04:15:21 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "#8 DONE 0.0s 04:15:21 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "#9 [stage-0 3/4] COPY . . 04:15:21 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "#9 DONE 0.0s 04:15:21 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "#10 [stage-0 4/4] RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env [\"/bin/bash\",\"-c\",\": bex-native-env-rev=none\\nwhile IFS= read -r record; do\\n [ -n \"$record\" ] || continue\\n key=${record%%=*}\\n encoded=${record#*=}\\n # The sentinel keeps command substitution from stripping value newlines.\\n value=\"$(printf '%s' \"$encoded\" | base64 -d 2>/dev/null && printf '.')\" || exit 1\\n export \"$key=${value%.}\"\\ndone < /run/secrets/render-env\\ncp /etc/secrets/qa-r59.txt index.html\"] 04:15:21 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "#10 0.089 cp: cannot stat '/etc/secrets/qa-r59.txt': No such file or directory 04:15:21 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "#10 ERROR: process \"/bin/bash -c : bex-native-env-rev=none\\nwhile IFS= read -r record; do\\n [ -n \\\"$record\\\" ] || continue\\n key=${record%%=*}\\n encoded=${record#*=}\\n # The sentinel keeps command substitution from stripping value newlines.\\n value=\\\"$(printf '%s' \\\"$encoded\\\" | base64 -d 2>/dev/null && printf '.')\\\" || exit 1\\n export \\\"$key=${value%.}\\\"\\ndone < /run/secrets/render-env\\ncp /etc/secrets/qa-r59.txt index.html\" did not complete successfully: exit code: 1 04:15:21 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "------ 04:15:21 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "> [stage-0 4/4] RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env [\"/bin/bash\",\"-c\",\": bex-native-env-rev=none\\nwhile IFS= read -r record; do\\n [ -n \"$record\" ] || continue\\n key=${record%%=*}\\n encoded=${record#*=}\\n # The sentinel keeps command substitution from stripping value newlines.\\n value=\"$(printf '%s' \"$encoded\" | base64 -d 2>/dev/null && printf '.')\" || exit 1\\n export \"$key=${value%.}\"\\ndone < /run/secrets/render-env\\ncp /etc/secrets/qa-r59.txt index.html\"]: 04:15:21 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "0.089 cp: cannot stat '/etc/secrets/qa-r59.txt': No such file or directory 04:15:21 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-8h3jtlppiu2ec0hqo6l2": "[8h3jt]"
  - text: "error: failed to solve: process \"/bin/bash -c : bex-native-env-rev=none\\nwhile IFS= read -r record; do\\n [ -n \\\"$record\\\" ] || continue\\n key=${record%%=*}\\n encoded=${record#*=}\\n # The sentinel keeps command substitution from stripping value newlines.\\n value=\\\"$(printf '%s' \\\"$encoded\\\" | base64 -d 2>/dev/null && printf '.')\\\" || exit 1\\n export \\\"$key=${value%.}\\\"\\ndone < /run/secrets/render-env\\ncp /etc/secrets/qa-r59.txt index.html\" did not complete successfully: exit code: 1 04:15:31 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-d9qdpse3ui10c5ihau0m": "[d9qdp]"
  - text: "==> Build failed: build failed in the docker build step: 04:15:31 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-d9qdpse3ui10c5ihau0m": "[d9qdp]"
  - text: "==> 0.089 cp: cannot stat '/etc/secrets/qa-r59.txt': No such file or directory 04:15:31 PM"
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-d9qdpse3ui10c5ihau0m": "[d9qdp]"
  - text: ==> ------ 04:15:31 PM
  - button "Filter logs by instance srv-db0bn6nfq78s73et1e1g-d9qdpse3ui10c5ihau0m": "[d9qdp]"
  - text: "==> error: failed to solve: process \"/bin/bash -c : bex-native-env-rev=none\\nwhile IFS= read -r record; do\\n [ -n \\\"$record\\\" ] || continue\\n key=${record%%=*}\\n encoded=${record#*=}\\n # The sentinel keeps command substitution from stripping value newlines.\\n value=\\\"$(printf '%s' \\\"$encoded\\\" | base64 -d 2>/dev/null && printf '.')\\\" || exit 1\\n export \\\"$key=${value%.}\\\"\\ndone < /run/secrets/render-env\\ncp /etc/secrets/qa-r59.txt index.html\" did not complete successfully: exit code: 1"
```
