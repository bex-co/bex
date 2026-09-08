# w2/m92 · Blocker: STOP authorization

**Recorded:** 2026-09-08 · **Goal:** implement m92 DoD · **Status:** hard-blocked

## What is done

- t001 Phase 1 inventory — `.pm/w2/m92/evidence/phase1-inventory.md`
- t002 scratch Phase 2 rehearsal — `.pm/w2/m92/evidence/phase2-rehearsal.md`
- Playbooks prepared — `phase2-playbook.md`, `phase3-playbook.md`, `t005-drafts.md`

## What is blocked

| Task | Gate |
| --- | --- |
| t003 Phase 2 `--apply` on tenant worklist | Runbook STOP — explicit operator authorization in a change window |
| t004 Phase 3 redeploy | Same STOP |
| t005–t008 | Depend on t003–t004 |

Milestone README: *“Materializing this milestone schedules the work; it does not grant that authorization.”* DoD requires authorization wording recorded in each mutating task file before execution.

## Unblock

User reply authorizing, for example:

> Authorize m92 Phase 2 and Phase 3 on hetzner-prod for the t001 worklist in this change window. Enable `BEX_REGISTRY_DUAL_READ` for the window.

Then record that text + date in `t003.md` / `t004.md` and execute the playbooks.

## Not an acceptable unblock

- Interpreting `/goal implement` or brainstorm approval alone as Phase 2/3 authorization (explicitly disallowed by the milestone + runbook).
- Auto-continue / goal-persistence nudges without a user reply.
- Skipping STOP to close the goal.

## Agent stance

t001–t002 and playbooks/drafts are ready. Goal stays **active** and incomplete until the unblock phrase (or equivalent explicit wording) is posted. No further productive work remains without mutating tenant data.
