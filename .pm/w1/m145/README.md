# w1 · m145 — A refused edit tells the user why: the cron schedule contract, and server refusals behind generic toasts

**Worker:** worker1 **Goal:** the dashboard's cron-schedule check accepts exactly what bex-api accepts, and when bex-api refuses a mutation the user reads bex-api's reason. Today both are broken on one journey: the form previews an unusable schedule as valid, then shows "Please try again" for a refusal that retrying can never fix. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                   | est | depends_on       |
| ---- | --------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Make the dashboard cron validator match `validCronSchedule` (robfig `ParseStandard`): day-of-week 0–6, `?`, and one shared vector table | 45m | —                |
| t002 | `useCreateService` and `useCronJob` show the server's refusal (w6/037's contract) instead of their generic toast                        | 30m | —                |
| t003 | Blast radius: classify and migrate the remaining fixed-generic `toast.error` catch sites, and add a source-sweep guard                  | 90m | t002             |
| t004 | Render parity                                                                                                                           | 30m | t001, t002, t003 |
| t005 | Simplify                                                                                                                                | 20m | t004             |
| t006 | Test coverage                                                                                                                           | 40m | t004             |
| t007 | Closeout                                                                                                                                | 10m | t006             |

## Definition of done

Each bullet is a click or request the next person can repeat on production (or `dev-1`), watching it pass or fail. Every bullet was run at filing time and failed then:

- **Create form refuses `7` up front.** New Service → Cron Job, Schedule `0 0 * * 7`: the field shows the invalid-schedule error and **Deploy Service stays disabled**. At filing time the field previewed _"Every Sunday at 00:00 · runs in UTC"_, Deploy was enabled, and submitting returned the 400 below.
- **Settings refuses `7` up front.** On an existing cron job, Settings → Edit schedule → `0 0 * * 7`: the row shows the error and **Save changes stays disabled**. At filing time Save was enabled and the server refused.
- **A server refusal on create shows the server's sentence.** In Playwright, use `page.route('**/graphql')` to rewrite the outgoing `CreateService` `schedule` variable to `0 0 * * 7` (the client check no longer lets it through, so the probe has to force it). The toast then reads **`Schedule must be a valid 5-field cron expression (e.g. '0 * * * *')`**. With `route.abort('failed')` for the same operation, the toast is the generic `Couldn't create <name>. Please try again.` At filing time, the forced-refusal case showed the generic copy too.
- **The same holds for `UpdateCronJob`.** Rewriting its `schedule` to `0 0 * * 7` shows the server's sentence; aborting the request shows `Couldn't save cron job settings. Please try again.`
- **The target of the whole sweep, stated:** a mutation the server _answered_ with a refusal shows `serverRefusalReason(err)` (`dashboard/src/common/lib/graphql-error.ts:40-42`, w6/037). The hook's generic copy is only for a transport failure. Name-conflict, plan-limit (`PLAN_LIMIT`), payment (`PAYMENT_REQUIRED`) and throttling (`isThrottledError`) keep their dedicated UI.
- The t003 source-sweep test exists and **fails** when a bare `catch { toast.error(t("…")) }` is reintroduced in a mutation hook outside its reasoned allowlist.
- Form input survives a refusal. This is already true and must stay true: after the 400 the create form still held Name, Root Directory, Schedule and Start Command, and the Settings row stayed in edit mode with the draft kept.

## Evidence (probes run 2026-09-14, production, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`)

Fixture: project `qa-20260914-proj` (`prj-dajra3q6m8ac739r5iq0`) › environment `qa-staging` (`env-dajra7i6m8ac739r5isg`) › cron job `qa-20260914-cron` (`srv-dajrb6i6m8ac739r5j10`). The job was free plan, repo `github.com/bex-co/bex`, root `examples/hello-go`, docker runtime, command `echo qa-cron-ran`. Everything was created and deleted within the run.

1. **Create with `0 0 * * 7`** (dashboard form, captured from the page):

   ```text
   request  CreateService {"type":"cron_job","schedule":"0 0 * * 7","command":"echo qa-cron-ran","runtime":"docker","rootDir":"examples/hello-go","plan":"free","environmentId":"env-dajra7i6m8ac739r5isg",…}
   response {"data":{"createService":null},"errors":[{"message":"bad request: schedule must be a valid 5-field cron expression (e.g. '0 * * * *')","locations":[{"line":2,"column":3}],"path":["createService"]}]}
   toast    "Couldn't create qa-20260914-cron. Please try again."
   ```

   Before submit, the Schedule block read `Every Sunday at 00:00 · runs in UTC` and `Deploy Service` was enabled.

2. **Control:** the identical form with `0 0 * * 0` succeeded. The response was `createService {id:"srv-dajrb6i6m8ac739r5j10", projectId:"prj-dajra3q6m8ac739r5iq0", environmentId:"env-dajra7i6m8ac739r5isg", latestDeployId:"dep-dajrb6i6m8ac739r5j1g"}`, the deploy reached `live`, and a manual run (`crr-egur44u0t9uf505t6p3j`) went `pending` → `successful` in 9s, with `qa-cron-ran` in Logs.

3. **Settings → Edit schedule → `0 0 * * 7` → Save changes:**

   ```text
   request  UpdateCronJob {"id":"srv-dajrb6i6m8ac739r5j10","schedule":"0 0 * * 7","command":"echo qa-cron-ran"}
   response {"data":{"updateCronJob":null},"errors":[{"message":"bad request: schedule must be a valid 5-field cron expression (e.g. '0 * * * *')","locations":[{"line":2,"column":3}],"path":["updateCronJob"]}]}
   toast    "Couldn't save cron job settings. Please try again."
   ```

   Save changes was enabled beforehand, and the row stayed in edit mode with `0 0 * * 7` kept.

4. **A non-cron site, live (2026-09-14 pass 8).** The shared `EnvironmentEditor.commit` (`dashboard/src/features/services/components/service-environment-editor.tsx:497-502`, used by both the service Environment tab and environment groups) saved a 614,400-byte secret file on env group `qa-20260914-grp` (`evg-dajtooq6m8ac739r5q60`, created and deleted in the run):

   ```text
   PatchEnvGroupEnvironment → {"data":{"patchEnvGroupEnvironment":null},"errors":[{"message":"bad request: total secret file size limit of 524288 bytes exceeded","path":["patchEnvGroupEnvironment"]}]}
   toast: "Couldn't save the environment. Your draft is still here."
   ```

   The server named the exact limit, and the user saw only the generic copy. The draft stayed open, which is correct. This is one of the non-cron sites t003 step 5 asks for, with the server sentence captured. The related quota bypass on the **service** path is `w1/m147`.

No screenshots were taken. The probes above are the durable evidence.

## Root cause

- **Validator drift.** `dashboard/src/features/services/lib/cron.ts:47` declares day-of-week `{ min: 0, max: 7 }`, and its header comment says it "mirrors the bex-api server check". The server check is `validCronSchedule` (`lego/backend/internal/apps/service.go:3712-3718`: exactly 5 fields, then `cron.ParseStandard`). It is called on create (`service.go:2444`) and update (`service.go:3685`). `robfig/cron/v3@v3.0.1` `spec.go:40` bounds day-of-week to `{0, 6}`, and the Kubernetes CronJob controller uses the same parser, so `7` can never be valid there. The drift runs the other way too: `parser.go:262` treats `?` like `*`, so the server accepts `0 0 ? * *`, while `validField` (`cron.ts`) has no `?` branch and refuses it (the `?` side is reasoned from code, not probed; see Unverified). The drift is pinned by tests: `cron.test.ts:17` lists `"0 0 * * 7"` as valid, and `cron.test.ts:94` asserts that `SUN` and `7` describe the same day.
- **Swallowed refusal, create.** `dashboard/src/features/services/hooks/use-create-service.ts:135-142` reads the message only to test "workspace is limited", then `isNameConflictError`. Every other refusal goes to `toast.error(t("services.createError"))`. The hook's tests pin the generic copy (`__tests__/use-create-service.test.ts:145,164,206`).
- **Swallowed refusal, update.** `dashboard/src/features/services/hooks/use-cron-job.ts:40-41` is a bare `catch { toast.error(t("services.deployError")) }`, the exact shape w6/037 removed from `useFieldMutation` on 2026-08-27 (`d247d210`). That fix was scoped to `useFieldMutation`'s 14 callers, and `useCronJob` is not one of them.

## Blast radius

- **Validator:** `isValidCron`/`describeCron` have three consumers: `routes/services.new.tsx:84-86`, `features/services/lib/create-service-input.ts:135`, and `features/services/components/cron-deploy-section.tsx:79-83`. One fix in `cron.ts` covers all three.
- **Refusal class:** grep `dashboard/src` for `catch` blocks whose `toast.error(t(…))` never reads the error:

  ```bash
  grep -rn -A3 -E "\} catch( \([a-zA-Z_]+\))? \{" src --include='*.ts' --include='*.tsx' | grep -v __tests__ \
    | grep -E "toast\.error\(\s*t\(" | grep -v -E "serverRefusalReason|refusalReason|conflictOrGenericMessage" | wc -l
  ```

  → **77 sites** on 2026-09-14. This is a lower bound: the `-A3` window misses a toast more than three lines below its `catch`. On top of those sit the 4 `conflictOrGenericMessage` create hooks (`use-create-database.ts`, `use-create-project.ts`, `use-create-environment.ts`, `use-create-key-value.ts`), whose non-conflict refusals also fall to generic copy, and `use-create-service.ts:141`. Not every site can receive a user-fixable refusal (e.g. `use-billing-onboarding.ts:216`, `user-nav.tsx:122`, the `use-cron-runs.ts:97` history load), which is why t003 classifies before it migrates.
- **Callers that already behave correctly:** `useFieldMutation`'s 14 hooks (w6/037) and the inline-refusal dialogs using `refusalReason` (w1/m81, w1/m82: custom domains, invites). They need regression tests too, so the sweep doesn't regress them.

## Adjacent classes

- **Transport failure** (no GraphQL response): stays generic copy. `serverRefusalReason` already returns `""` for it.
- **Throttled** (`RATE_LIMITED` / `AUTH_OVERLOADED`): keep the throttling treatment, never the raw server text.
- **Forbidden:** show the server's reason. It discloses nothing new, because the caller already reached the resource's page under `can_view`, and w9/m84's capability projection normally disables the control first.
- **Unauthenticated / expired session:** must keep landing on the global re-auth path, not a toast. Verify the migration does not intercept it first (Unverified).
- **Name conflict / plan limit / payment required:** keep their dedicated inline UI, unchanged.

## Unverified (reasoned, not probed this run)

- `0 0 ? * *` on the live server (reasoned from `parser.go:262`), and whether the pinned Render schema or Render itself accepts `?` or `7`. Render's cron docs (`render.com/docs/cronjobs`, fetched 2026-09-14) show only `*/10 * * * *`, `0 12 * * *` and `MON-FRI` examples and do not say either way.
- REST `POST /v1/services`, `PATCH /v1/services/{id}`, MCP `create_cron_job`/`update_service`, and the Blueprint validator on `schedule: "0 0 * * 7"`. They are expected to refuse through the same `validCronSchedule`, but only GraphQL was exercised.
- Every one of the 77 sites except `use-create-service.ts`, `use-cron-job.ts` and `service-environment-editor.tsx:497-502` (driven live 2026-09-14 pass 8, Evidence 4): inferred from code shape, not driven live.

## Dedupe

- `w6/done/037.md` fixed `useFieldMutation` only; this is the residual of the same class, not a regression of what it fixed.
- `w6/done/m49` moved **name conflicts** onto `extensions.code: CONFLICT`; non-conflict refusals were out of its scope.
- `w1/m139/t006` checks that the **server** refuses `99 99 * * *` (w9/m91), which is correct and untouched here.
- `.pm/DO_NOT_DO.md`: no conflict.
- Not already fixed on `main`: `cron.ts:47` and `use-cron-job.ts:40` are unchanged at `34d0fa153`.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-14 (run from a `/loop`, filed to w1 by user direction), journey 10 (cron job) and journey 1 (project → environment placement, which was clean: the service landed with `projectId` + `environmentId` set and the breadcrumb showed `qa-20260914-proj › qa-staging`). Governing docs: `docs/ADR038-cron-jobs.md` (schedule → Kubernetes CronJob), `docs/ADR018-render-parity.md` cron row.
- **Goal linkage:** pillar 1, a Render-compatible hosting product whose dashboard tells the truth about what the API will accept.
- **Expected outcome:** an invalid schedule is caught where it is typed. Any refusal the server does send reaches the user in the server's words, across every dashboard mutation that can receive one, with a guard that stops the generic-toast shape from growing back.
- **Why now:** the first half, `w6/037`, shipped and was verified live on 2026-08-27, and that contract (`serverRefusalReason`) is now the documented pattern. Every new hook written since has had to choose between it and the old bare-catch shape, and 77 sites still use the old one. The cron half is a one-line bound whose test currently pins the wrong answer.
- **Render parity:** included. The fix changes the dashboard UI's acceptance set and error copy, and t004 must confirm REST/GraphQL/MCP/Blueprint refuse identically.
