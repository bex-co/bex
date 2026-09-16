# w11 — project workstream (worker11)

**Worker:** worker11. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Milestones

> **Every open milestone is blocked and has been moved to `blocked/` (2026-09-15).** All four trace to one gate: release-signed builds on physical iOS and Android hardware, which no dev environment can produce. Nothing in this workstream is implementable until that gate clears. See [docs/runbooks/mobile-push.md](../../docs/runbooks/mobile-push.md) for the credential inventory and the exact steps you need to run.

- [x] **m1** — Seed and sanitize the Expo mobile foundation (8 tasks) — **DONE 2026-08-02** ← from user request + source-mobile agent audit
- [x] **m2** — Secure native shell: auth, API, workspace, navigation (9 tasks) — **DONE 2026-08-02** ← from ADR048 D5 reconciliation
- [x] **m3** — Read-only mobile supervision (9 tasks) — **DONE 2026-08-02** ← from ADR048 D2 / M1
- [x] **m4** — Safe one-tap operations (8 tasks) — **DONE 2026-08-02** ← from ADR048 D2 / M1
- [ ] **m5** — **BLOCKED (needs your hardware + Play Console: signed physical iOS/Android devices, the Play App Signing fingerprint, and the two association repo vars)** — [Push channel + notification hygiene](blocked/m5/README.md) (11 tasks) ← from ADR048 D2 + gap 1/3. t001–t006 and t008–t010 are done; **every implementable task is complete**. `t007` needs release-signed builds on real hardware, which a dev environment cannot produce, and `t011` closeout is gated on t007's evidence.
- [ ] **m6** — **BLOCKED (same device gate as m5/t007)** — [Phase-1 agent mission control](blocked/m6/README.md) (9 tasks) ← from ADR048 M2 + ADR047 phase 1 — **t001–t008 implemented + verified + shipped/deployed to prod 2026-08-08 (`3bbe956d`)**; t009 closeout hard-blocked on the live phone→draft-PR proof + physical-device push verification (also the w3/m41 + w11/m5 gates), unavailable in a dev environment.
- [ ] **m7** — **BLOCKED (transitively, on `m6/t009`)** — [Live agent attach and needs-decision steering](blocked/m7/README.md) (9 tasks) ← from ADR048 M3 + ADR047 phase 2. Its originally stated blocker — the ADR047 phase-2 gateway — **cleared**: the attach listener, transcript store and conversation API shipped in `w3/m43`. All 9 tasks remain unimplemented, but its hard gate `w11/m6/t009` is device-blocked, so it cannot start.
- [ ] **m8** — **BLOCKED (same device gate as m5/t007)** — [Tier-2 mobile quick actions](blocked/m8/README.md) (9 tasks) ← from ADR048 D3. t001–t007 and t009 done; only `t008` remains and it waits on m5/t007's signed-device gate.
- [x] **m9** — Global side drawer: workspace switcher, personal status, and logout (9 tasks) — **DONE 2026-08-02** ← from user request 2026-08-02 after studying `../beancount-io/mobile/src/components/ledger-drawer/`
