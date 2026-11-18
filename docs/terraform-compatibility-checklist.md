# Render Terraform provider compatibility checklist

Render's official Terraform provider ([render.com/docs/terraform-provider](https://render.com/docs/terraform-provider), registry `render-oss/render`, source `render-oss/terraform-provider-render`) graded against bex with **its own acceptance suite**, unmodified, re-recorded against a bex API. It is a client bex must serve, like the [Render CLI](cli-compatibility-checklist.md); [ADR018](ADR018-render-parity.md) points here from its "Terraform provider" row.

Legend: `[x]` passes against bex · `[~]` passes in part · `[ ]` fails, with the board item that owns each bex-side cause · `[-]` a recorded omission or non-goal ([ADR018 § Render operation omissions](ADR018-render-parity.md#render-operation-omissions), `.pm/DO_NOT_DO.md`).

## Pins and harness

- **Provider:** `render-oss/terraform-provider-render` at `06af456d9bdb1b839f29923ac7e435c4a1f01f3c` (tip on 2026-11-16, after release v1.9.1).
- **Terraform:** 1.17.2. The harness checks the release zip against a sha256 pinned per platform.
- **Harness:** `scripts/terraform-provider-verify.sh <package>... | all` with `BEX_API_URL`, `RENDER_API_KEY` (a bex API key) and `RENDER_OWNER_ID` (the workspace `tea-` id). Optional: `TF_VERIFY_RUN` (a test-name filter), `TF_VERIFY_WORKDIR` and `TF_VERIFY_TIMEOUT`.
  - **Clone.** It clones the pinned commit into a work directory outside the repository; it refuses one inside. Nothing from the clone runs except `go test` on the named package.
  - **Record mode.** It runs one test per `go test`, with `TF_ACC=1` and `UPDATE_RECORDINGS=true`. The provider's go-vcr helper then talks to `RENDER_HOST` and writes a fresh cassette over Render's own.
  - **Results.** It prints one row per test: target `bex` (the test uses the recording provider) or `offline` (an `httptest` fake or a unit test), the result, and the first error.
  - **Sweep.** After each test, it deletes every resource that appeared in the workspace while the test ran, then confirms none is still listed. A test that fails after bex created something leaves it out of Terraform's state, so this is what keeps the next test's names free. Use a workspace no one else is creating resources in.
  - **Run output.** Re-recorded cassettes and logs stay in the work directory. It prints a recordings tarball's sha256 and the `git -C <work>/terraform-provider-render diff` that compares bex's answers with Render's.
- **Workspace:** the tests assume Render's test workspace, which is on a plan with webhooks, autoscaling, network isolation and inbound IP rules, so the graded workspace was upgraded to Scale (`changeWorkspacePlan`).
- **Tests:** `scripts/terraform-provider-verify.test.sh` (offline: argument and work-directory refusals, the result table, the sweep against a fake API), run in `scripts.yml`.

## Runs (2026-11-18)

Production's pin (`054f29128f64`) did not yet carry w9/m472's request-body fixes, so every grade comes from **dev-9**, w9's own stack: `scripts/dev-env.sh 9 up` plus `agent-up` for OpenBao, which holds env vars and secret files. bex-api was built from main at `35d7bbdf2`, so it carries w9/m472 and w9/m473. A throwaway identity on a Scale workspace held a freshly minted API key.

- **Run A (production-shaped).** `BEX_REGION=fsn1`, so resources read back `frankfurt` as on production. The API key is used as minted, which bex binds as a Developer. Recordings sha256 `ac31fc8f…` (all packages) and `efd4a688…` (the three tests rerun after the harness learned the clone's directory name).
- **Run B (diagnostic, not a production configuration).** The same stack with `BEX_REGION=oregon` and the API key also granted Admin in dev-9's OpenFGA. That lifts the two failures that stop most tests at their first step (rows 1 and 3 below), so the failures behind them show. Recordings sha256 `7f17e253…`.
- **Registry credentials** have no upstream test at the pin. They were graded by a supplementary `apply`, second `plan` and `destroy` of a `render_registry_credential` and its data source, using the provider binary built from the pinned commit (`dev_overrides`).
- **Cleanup.** Every resource the runs created was destroyed by Terraform or the sweep, and the workspace read empty on every collection at the end. Afterwards the API keys were revoked, dev-9 was torn down and its empty tenant namespaces were deleted. Recordings were not uploaded: they are run output that the harness re-creates.

## Grades

| Package | Resources · data sources | Grade | Run A (production-shaped) | Run B (diagnostic) | Owning items |
| --- | --- | :-: | --- | --- | --- |
| backgroundworker | `render_background_worker` · data source untested upstream | `[ ]` | 0/7. Region ×5, preview notifications `true`, protected environment 403 | 0/7. `imagePath` ×3 (+ autoscaling % ×1), env-vars `null` ×2, preview notifications `true`, `previews.generation: automatic` | w9/105, w9/106, w9/109, w9/110, w9/111 |
| cronjob | `render_cron_job` · data source untested upstream | `[ ]` | 0/6. Region ×4, preview notifications `true`, protected environment 403 | 0/6. env-vars `null` ×3, preview notifications `true` ×2, `imagePath` | w9/105, w9/106, w9/109, w9/110 |
| dedicatedip | `render_dedicated_ip` · data source | `[-]` | 0/3, `404` (dedicated outbound IPs) | same | omission |
| envgroup | `render_env_group`, `render_env_group_link` · both data sources | `[~]` | 2/2 env-group tests pass: create, update, import, destroy. The link resource and both data sources are tested only against a fake server upstream (9/9 offline pass) | 2/2 | — |
| keyvalue | `render_keyvalue` · data source | `[ ]` | 0/2. Connection info `409` right after create, protected environment 403 | 0/2. Connection info `409` | w9/108, w9/106 (w9/105 next: the tests name `oregon`) |
| logstreams | `render_log_stream` · data source | `[-]` | 0/1, `404` | same | omission (external telemetry streams) |
| metricstream | `render_metrics_stream` · data source | `[-]` | 0/1, `404` | same | omission (external telemetry streams) |
| notifications | `render_notification_setting` · data source | `[~]` | Data source passes. The resource's owner `PATCH` is 403 for an API key | Resource fails on `previewNotificationsEnabled: true` | w9/107; `true` is the recorded previews divergence |
| postgres | `render_postgres` · data source | `[ ]` | 0/4. Region (IP allow-list test), protected environment 403, `log_stream_override` `404`, `parameterOverrides` refused | 1/4. The IP allow-list test passes: create, update, import, destroy. The others: `log_stream_override` ×2, `parameterOverrides` | w9/105, w9/106; log streams are an omission; `parameterOverrides` is Render-retired (w2/066) |
| privateservice | `render_private_service` · data source untested upstream | `[ ]` | 0/8. Region ×6 (+ autoscaling %, `imagePath`), preview notifications `true`, protected environment 403 | 0/8. env-vars `null` ×3, `imagePath` ×3 (+ autoscaling % ×1), preview notifications `true`, `previews.generation: automatic` | w9/105, w9/106, w9/109, w9/110, w9/111 |
| project | `render_project` · data source untested upstream | `[ ]` | 0/1. Every create with an environment is 403 for an API key | 1/1: create, rename, add and remove environments, import, destroy | w9/106, w9/107 |
| redis | `render_redis` · data source | `[ ]` | 0/2. Connection info `409` right after create, protected environment 403 | 0/2. Connection info `409` | w9/108, w9/106 (w9/105 next) |
| registrycredential | `render_registry_credential` · data source | `[ ]` | Supplementary: create is 403 for an API key | Create, read through the data source, an empty second plan and destroy all pass | w9/107 |
| staticsite | `render_static_site` · data source untested upstream | `[-]` | 0/1. Its one test sets `previews.generation: automatic` | same | the PR-previews non-goal; nothing else of the resource is exercised |
| webhook | `render_webhook` · data source untested upstream | `[ ]` | 0/1. Create is 403 for an API key | same (an Admin key is still refused: it is not a human caller) | w9/107 |
| webservice | `render_web_service` · data source | `[ ]` | 0/6. Region ×4 (+ autoscaling %, `imagePath`), preview notifications `false`, data source preview notifications `true` | 0/6. `imagePath`, autoscaling %, `rootDir: ""` on an image service, env-vars `null`, preview notifications `false`, data source `true` | w9/105, w9/109, w9/110, w9/111, w9/112, w9/113 |

Totals against bex: run A passed 3 of the 47 tests that use the recording provider, and run B passed 5. Each failing test stops at its first refused request, so a fixed row can still uncover a later failure. Re-run the harness after each fix. The "Region" failures in run A's column are fixed (class 1, w9/105): with production's mapping, the tests behind them now stop where run B did.

## Failure classes

Each class names the request, Render's recorded answer and bex's answer. "Render" is the provider's own cassette at the pin; "bex" is the re-recording.

1. **The requested region was read back as bex's own** (w9/105, fixed 2026-11-18). For example, `POST /v1/services` with `serviceDetails.region: "oregon"` got Render's 201 with `oregon`, and bex's 201 with `frankfurt`. The provider stores the response's `region`, a required `RequiresReplace` attribute, so Terraform aborted the apply with _Provider produced inconsistent result after apply: .region: was "oregon", but now "frankfurt"_. This hit every service, Postgres and Key Value configuration that names a region other than `frankfurt`; Render's suite names `oregon` in 50 configurations and `ohio` in 5. A resource now reads back the Render region it was created with and still runs in `BEX_REGION` ([ADR049](ADR049-render-yaml-parity.md) D7, amended). **Rechecked on dev-9 with `BEX_REGION=fsn1`** (main `1ec5007be` plus the fix): `TestAccPostgresIPAllowListResource` passes (create, update, import, destroy; every recorded answer reads `oregon`), and `TestWebServiceResource_Autoscaling` gets past the region to class 7's memory percentage (w9/111).
2. **An environment that names the defaults needs Admin** (w9/106). The provider always sends `protectedStatus` (`"unprotected"` unless configured) and `networkIsolationEnabled: false` for each environment. Render answers 201. bex requires `can_manage` whenever either field is present (`lego/backend/internal/projects/service.go`), so a Developer gets 403 even for `unprotected`. An API key is always a Developer, so every `render_project` with environments fails. That stops 7 tests, most of them at a shared project fixture.
3. **An API key acts as a Developer, never as its creator's role** (w9/107). Render's key permissions go up to "every permission your member role allows" ([render.com/docs/api-keys](https://render.com/docs/api-keys)), and classic keys "have the same permissions as your member role". bex binds every key as a Developer (`BindKey`), so its Admin-only writes are 403 for any key:

   - `PATCH /v1/notification-settings/owners/{id}`;
   - `POST /v1/registrycredentials`;
   - a protected environment.

   `POST /v1/webhooks` is refused even for an Admin key, because only a human caller may mint a signing secret (`AuthorizeMintClass`, w4/079).

4. **Key Value connection info `409` right after create** (w9/108). Render answers `GET /v1/key-value/{id}/connection-info` 200 with its strings while the store is still `creating`. bex answers `409 "key value credentials are still converging"` until the operator records the credential revision. Each Key Value and Redis test reads it right after create, and so does the provider's create.
5. **`PUT …/env-vars` and `…/secret-files` with a `null` body** (w9/109). The provider sends `null` to clear a service's variables and files, and Render answers 200 `[]`. Render's cassettes hold 66 such requests (32 env-vars, 34 secret-files), all answered 200. bex's Render request gate refuses with `400 request body does not match the Render API schema`. w9/m472 made the lists nullable inside a create body only.
6. **`imagePath` is read back as written** (w9/110). A create with `image.imagePath: "nginx"` gets Render's `imagePath: "docker.io/library/nginx:latest"` and bex's `"nginx"`. The provider takes `image_url` and the tag from `imagePath` only when it has a `:` or `@`, so `image_url` reads null. Terraform then aborts every image-service create that names a bare image.
7. **A disabled autoscaling criterion loses its percentage** (w9/111). A create with `criteria.memory: {enabled: false, percentage: 70}` gets 70 back from Render and 0 from bex.
8. **A service notification override with `previewNotificationsEnabled: "false"`** (w9/112). Render answers 200 and reads `"false"` back. bex answers `400 … use default`. `"true"` stays refused: preview notifications belong to PR previews, a non-goal.
9. **A `PATCH` of an image service carrying `rootDir: ""`** (w9/113). Render answers 200. bex answers `400 has no repo to build (root directory only applies to build-from-git)`, though the value is the service's own empty root directory.

**Recorded omissions and non-goals** (no item):

- dedicated IPs (`404`);
- log streams, metrics streams and per-resource `log_stream_override` (`404`);
- `previews.generation: automatic`/`manual` (`services previews is not supported`);
- `previewNotificationsEnabled: true`, on the owner settings and on overrides;
- Postgres `parameterOverrides`. Render dropped it from its own API on 2026-11-02, and bex refuses it by name (w2/066), so those provider tests predate Render's API.

**Test assumptions:** none was the cause of a failure.

- The suite's repositories (`render-examples/*`) and images (`nginx`) are public.
- Owner ids and emails are scrubbed by the recorder.
- The plan-gated features match a Scale workspace.
- A destroy that failed after an earlier error (a project whose environment still held a resource the provider had lost) is a consequence of the first failure, not a separate class.
