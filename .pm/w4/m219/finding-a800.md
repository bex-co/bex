# a800 — Confirm the service family and the Postgres dialog boundary

Functional follow-up on 2026-10-16 UTC, researched at `9b68b35765693cd0f3de5278a6f2ef4d9891b4db`. This extends m219's same shared-form cause; no new milestone or product fix.

## Four more observed failures

The exact a797 V2 probe held the same real HTTP200 startup entry response, released it unchanged, and typed through the browser keyboard before handlers attached. No API response was changed, and no resource was created.

| Receipt | UTC | Type | Width | Accepted Name | After startup |
| --- | --- | --- | --- | --- | --- |
| a800PrivateHeld | 23:40:04 | private_service | 1440 | qa-a800-private | empty |
| a800WorkerHeld | 23:40:10 | background_worker | 390 | qa-a800-worker | empty |
| a800CronHeld | 23:40:21 | cron_job | 1440 | qa-a800-cron | empty |
| a800StaticHeld | 23:40:27 | static_site | 390 | qa-a800-static | empty |

All four have a trusted native input event while `reactProps:false`, one intentionally held fetched 200 response, and the same DOM node through every sample. Each after-ready control retains its corresponding Name plus `-ready`. Together with a797's web cases, Name loss is now live-observed in all five service types through the same `services.new.tsx:180–199` / `use-service-name-draft.ts:33` implementation. This is a five-type census at the listed widths, not a claim of both widths for every type. Source tabs other than the default GitHub tab remain untested here.

## Postgres dialog control

Fresh-load `https://dashboard.bex.co/?new=database` at390px under the identical held startup response. After waiting1.2s with the real response already fetched, there are zero accessible Name textboxes and zero dialogs. Release the response unchanged: New Postgres appears, its Name input is enabled with React props, and native typing `qa-a800-dialog-ready` remains unchanged after1.5s. No create clicked.

The reason is source-traced: `routes/index.tsx:150–166` derives the open state from the URL; `common/components/ui/dialog.tsx` uses Radix DialogPortal. The installed lockfile-pinned `@radix-ui/react-dialog@1.1.15` depends on `@radix-ui/react-portal@1.1.9`. Its actual `dist/index.mjs:12–15` starts mounted false, sets true in a layout effect, chooses `document.body` only after mount, and returns null without a container. That matches the live absence-before/ready-after control. No generic claim that other dialogs pass is made.

## Handoff and cleanup

[Durable evidence](../evidence/m219-a800-siblings.json) holds the four complete failure receipts, exact Postgres comparison code and receipt, dependency mechanism, inventory check and session-clear receipt. Reuse the exact a797 V2 factory and listed parameters for service replays. Primary bootstrap and React-controlled input research remain in [the original finding](finding.md).

Add these observed type/width cases to m219's live acceptance and regression matrix, preserve the Postgres control, and leave dialog behavior unchanged. The local readiness gate still belongs to the same three SSR form roots; estimates and closing task order remain unchanged. Existing source-route alias coverage remains work to verify, not a new live claim.

No resources or paid actions. At23:41:12 all seven inventory families match baseline, with no console errors or HTTP failures in the cleanup capture. At23:41:16 the run's own session is revoked (whoami401), cookies0, sole tab about:blank, exact handles removed. No historical request census claim.
