# Workspace slug accepts startup edits and then clears them

Why: a customer can type a valid workspace slug into a usable-looking field and silently lose it as the page initializes.

**Severity:** major (creation draft loss). Ordinary functional QA a846, 2026-10-17 UTC; source main `e99af00efe64185376f19513f07ad81c2a8879d2`. Extend the existing m219 owner through t014, its ninth observed root; this filing changes no product code.

## Reproduction and evidence

Fresh-load `/new/workspace` with Hobby selected and no resume attempt. At1440×844 and390×844, fetch and hold the current real entry response, type into **Workspace slug** using the browser keyboard, then release it unchanged. Repeat mobile from a fresh load. [Durable artifact](../../evidence/m219-a846-workspace-startup.json) contains the exact scripts and complete receipts.

All three runs expose one effectively enabled, writable slug input without React props. Trusted input records the exact marker. The marker survives the first samples with React props, then becomes empty on the same node; all20 node-identity samples agree. The entry is HTTP200,1,954,259bytes, `index-pJFig2Dm.js`. No response was rewritten. Ready typing is retained; an additional native `QA-A846-READY` input settles to the intended lowercase `qa-a846-ready` after1.2s. No console or HTTP failures occurred during this pass.

The inspected screenshots `.playwright-mcp/qa-a846-slug-{before,after}.png` capture only the slug field's parent, excluding account billing-email and payment content. They supplement the committed receipts.

## Located cause and bounded fix

`dashboard/src/routes/new.workspace.tsx:46–54` initializes `draftName` empty and selects `attempt?.name ?? draftName`. Its first Input at :129–140 binds that name and lowercases `onChange`, but is disabled only when a prepared attempt exists. A native event before handlers cannot update the draft. Subsequent controlled rendering restores the empty state.

The official pinned React19.3.1 production source was reopened: `updateInput` at :1269–1293 writes the controlled value when different; `initInput` at :1315–1343 preserves the current DOM value while hydrating. This agrees with text surviving the initial handler attachment before clearing. Neither the precise later render nor the deployed React version was isolated; do not attribute the loss to a particular query response.

Reuse `useIsHydrated` locally on the slug: `disabled={!hydrated || creation.attempt != null}`. Keep it visibly in place with unchanged geometry; once ready, native typing must persist. Preserve the lowercase conversion,1–30 character resource-name validation, focus/labels, prepared-attempt lock, resume behavior and all existing submit/payment predicates. `name` is consumed by `isValidDnsLabel`, `handleSubmit` and `handleAddPaymentMethod`; changing only readiness leaves those contracts intact.

There are two Input sites in this route; only the slug changes. Billing Email is read-only for the observed Hobby default and disabled for an existing attempt. Other plans/resume cases need regression checks, not a claim of observed email loss. One production route registration and one production `useWorkspaceCreationBilling` call were found. Canonical `/new/workspace` is linked by the switcher and empty-membership/delete redirects; no additional route producer was found. PlanPicker has two production mounts (this page and Change Plan); neither changes. The hydration hook currently has10 production calls in9files; preserve them and global Input behavior.

## Controls, dedupe and cleanup

The qualified a845 control used Project Settings: its displayed name is disabled outside editing, and ready Escape/Cancel restores the saved value at390/1440. This was not a held-script replay and does not establish a broader Project startup guarantee. Earlier a839/a840 absence controls retain their existing qualifications.

Distinctive open/blocked/done searches, all86 open/blocked milestone titles, DO_NOT_DO, recent40 product commits and targeted `new.workspace.tsx` history found no slug-readiness fix. m207 owns PlanPicker keyboard behavior; workspace lifecycle/billing work and m219's earlier eight roots do not cover this input's observed loss. Preserve those existing contracts. ADR018, ADR030 and ADR075 govern the surrounding journey; this adds no billing or plan behavior.

[Render's workspace documentation](https://render.com/docs/organizations) describes entering a name before creation. It does not establish authenticated startup retention, and no signed-in Render comparison was run.

**Unverified:** nonempty resume attempts, other plans/locales, policy error/loading variations and completion of workspace creation. These remain scoped regression work. No workspace or paid action was submitted: zero GraphQL mutations were observed. All seven hosting inventory ID sets match baseline; workspace-ID equality was not measured. Exact-session logout succeeded, whoami401, cookies0, one blank page and own credential handles removed. No fixture exists to delete.
