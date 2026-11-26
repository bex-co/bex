# w1 · m645 — Every operation in Render's dashboard has a ledger row, and a weekly job finds new ones and lifted flags

**Worker:** worker1 **Goal:** every GraphQL operation and every feature-flag gate in Render's public dashboard bundle (703 operations and 8 flags on 2026-11-25) has a disposition row in `docs/render-artifacts/dashboard-operations.md`; a weekly job fails when Render adds or removes one, so a dashboard-only feature reaches the board within a week and a flagged experiment is never mistaken for a GA feature. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Extract operations and flag gates from Render's bundle](done/t001.md) — **DONE** | 1h30m | — |
| t002 | [Write the ledger: one row per operation and per flag](done/t002.md) — **DONE** | 2h30m | t001 |
| t003 | [Record the `postgres-parameter-overrides` finding in ADR018](done/t003.md) — **DONE** | 20m | t001 |
| t004 | [Weekly `render-dashboard-ops` job with fixture self-tests](done/t004.md) — **DONE** | 1h | t002 |
| t005 | [Record the method](done/t005.md) — **DONE** | 15m | t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 15m | t003, t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t003, t005 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t007 |

## Definition of done

1. `docs/render-artifacts/dashboard-operations.md` has one row for every operation and every flag the job reads from Render's live bundle. Each row has a disposition (matched, partial, gap, non-goal, n/a or flagged) and a pointer: an ADR018 row, a board item or a `.pm/DO_NOT_DO.md` line. Every gap row names an open board item.
2. The weekly job passes against the live bundle. Each of these fixture mutations makes it fail with a message naming the operation or flag:
   - a new operation;
   - a removed operation;
   - a new flag gate;
   - a removed flag gate, with the message naming the item to reopen;
   - an entry chunk that cannot be found.
3. ADR018's Postgres section records that Render's parameter-override editor sits behind the per-workspace experiment flag `postgres-parameter-overrides`, so w2/066's D2=A stands while the gate exists.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one more aspect of render parity for w4` (2026-11-25), item 2. On the user's request, the evidence was re-checked in a headed browser the same day and filed to w1. Item 1 of the brainstorm is folded in here; it is not filed on its own.
  - **The crawl:** Render's dashboard bundle is public and readable without signing in. It starts from `https://dashboard.render.com/` → `assets/index-*.js`, and its import graph has 459 chunks, holding 703 GraphQL operations.
  - **Chunk names change daily:** the entry was `index-BPMVVQol.js` in the morning and `index-D5V4ZgbE.js` by evening, so rows cite operation names and strings, never chunk hashes.
  - **Flag gates:** gated features call the hooks the `featureflags-*.js` chunk exports. On 2026-11-25 there were 8 flags:
    - `postgres-parameter-overrides`
    - `share-postgres`
    - `grandfather-owners-into-sharing`
    - `expose-auto-deploy-trigger`
    - `experiment-zero-state`
    - `experiment-post-deploy-upgrade`
    - `okta-app-available`
    - `skip-check-suspendable-when-showing-payment-failed-banner`
- **Why item 1 was folded in:** the brainstorm's item 1 proposed bringing back Postgres `postgresql.conf` overrides, because Render's bundle has a full editor: `postgresConfigParameters`, `updateDatabaseParameterOverrides` and the "PostgreSQL Configuration Overrides" card on the database Info page. The browser check showed the card renders only when `owner.featureFlags` holds `postgres-parameter-overrides` or `owner-all`.
  - The hook records "Experiment Exposed" with `experiment_group: treatment/control`.
  - Render's Workspace Settings Early Access list, "Opt in to new features before General Availability (GA)", is empty (`[]`), so no workspace can turn the flag on itself.
  - So an ordinary Render workspace has no editor, and the user's D2=A on [w2/066](../../../w2/done/066.md) ("与 Render 一致") still matches Render. The ledger's flag row and t004's removed-gate check are what reopen it.
- **Prior use of the technique:** w3 has read the bundle many times, one feature at a time (w3/m641, w3/115–118, w3/122, w3/m528), as have w2/m87, w3/m531 and w22/m603. None of these read it as a complete inventory, and nothing notices when Render adds an operation.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first). This is the dashboard counterpart of the changelog ledger (w4/m553, `scripts/render-changelog-drift.sh`), which by its own comment cannot see "a feature Render ships only in its dashboard" unless the changelog announces it.
- **Expected outcome:**
  - Render's dashboard-only behaviors become a closed list with owners. The first pass already found the three features filed beside this milestone: [m646](../../m646/README.md) (rollback comparison), [m647](../../m647/README.md) (2FA reminders) and [m648](../../m648/README.md) (avatars).
  - A Render experiment that goes GA, meaning its gate is removed, is on the board within a week.
- **Why now:** the first brainstorm on this bundle nearly reversed a user decision because of a flagged experiment. The ledger separates GA from flagged before anyone files work on it.
- **Render parity omitted:** a script, a CI job and docs; no REST, GraphQL, MCP or dashboard change.

## Closeout (2026-11-26)

- **DoD 1 holds.** [dashboard-operations.md](../../../../docs/render-artifacts/dashboard-operations.md) has one row for each of the 703 operations and 8 flags `scripts/render-dashboard-ops.sh` read from the live bundle on 2026-11-26 (entry `index-D5V4ZgbE.js`, 459 chunks; the counts match this README's 2026-11-25 read exactly). 474 matched, 137 non-goal, 62 gap, 22 n/a, 7 flagged, 1 partial. Every gap names an open or parked item: `w1/m646`, `w1/m647`, `w1/m648`, `w7/071`, and the eight notes this pass filed, [148](../../148.md) (per-service Authentication), [149](../../149.md) (Cursor Origin Git provider), [150](../../150.md) (Stripe integrations), [151](../../151.md) (org audit log keys, member login status), [152](../../152.md) (sandbox pages), [153](../../153.md) (image pre-check), [154](../../154.md) (Render's env var link picker against DO_NOT_DO's Link-database line) and [155](../../155.md) (deploy step timeline).
- **DoD 2 holds.** The check passes against the live bundle with the committed ledger. `scripts/render-dashboard-ops.test.sh` fails, with the name in the message, on a new operation, a removed operation, a new flag gate, a removed flag gate (quoting its row's pointer, "reopen `w2/066` D2=A" for `postgres-parameter-overrides`), and an entry chunk that is missing or cannot be fetched. The job runs every Monday in `render-schema-drift.yml`; its first scheduled run is 2026-11-30.
- **DoD 3 holds.** ADR018 § Managed Postgres (the Postgres observability row, beside the retirement) and ADR009 record the `postgres-parameter-overrides` gate, the editor's shape and the trigger.
- **Deviation:** t004's "a flag gate that moved to a different chunk family" is a notice, not a failure (see t004's outcome).
