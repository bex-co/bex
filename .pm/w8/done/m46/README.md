# w8 · m46 — Near-simultaneous deploy triggers race: the newest can lose, both can be canceled, and a deploy row can record the wrong image

**Worker:** worker8 **Goal:** concurrent deploy triggers on one service resolve deterministically, Render-style: the most recently accepted trigger wins, every earlier open deploy closes `canceled` (superseded), and each deploy row records exactly the image or commit its caller requested. **Status:** done

## Tasks (in order)

| id   | title                                                                                                             | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Reproduce in a backend test: two concurrent `Trigger(imageUrl)` calls against a fake store + CR, and pin the bad outcomes — **DONE** | 45m | —          |
| t002 | Serialize triggers per service (store-level per-app lock/CAS around row write + deploy row + release stamp) — **DONE**         | 60m | t001       |
| t003 | CR patch with optimistic concurrency and a fresh generation (no stale `previousGeneration+1`) — **DONE**                       | 40m | t002       |
| t004 | Newest-wins: an accepted trigger explicitly supersedes earlier open deploys; deploy hook + restart + rollback share it — **DONE** | 40m | t002       |
| t005 | Render parity — **DONE**                                                                                                     | 20m | t003, t004 |
| t006 | Simplify — **DONE**                                                                                                          | 15m | t005       |
| t007 | Test coverage — **DONE**                                                                                                     | 30m | t006       |
| t008 | Closeout — **DONE**                                                                                                          | 15m | t007       |

## Definition of done

- Two `bex deploys create <srv> --image …:A` / `…:B` launched within the same second (B second): the later accepted request ends `live` serving its image, the earlier ends `canceled`, and **each deploy row's `image.ref` equals what that call requested**. Saved source settings follow m44's separately verified policy. Repeated 10 times in a loop, the outcome is the same every time.
- The same holds for two deploy-hook `?imgURL=` calls and for an image trigger racing a parameter-free restart.
- Sequential triggers keep today's behavior (already correct with a gap of 3 s or more).

## Evidence (2026-09-26, `/qa-find-bugs-cli` sweep 18, production `726042a28`, which includes the w8/022 fix)

Released `bex v0.2.1` (pin v2.27.0), human device login, workspace `bex-canary`. Fixture: free image web service `srv-darq49gd0qnc73d79tsg` (`mendhak/http-https-echo`, since deleted).

| run | triggers (server `createdAt`)                                              | outcome                                                                                                             |
| --- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| 1   | `:34` `dep-darq4jgd0qnc73d79tug` and `:35` `dep-darq4jhsmc7s73cq5icg`, same instant | the **older** `:34` went `live`, the newer `:35` was `canceled`                                                     |
| 2   | `:35` `dep-darq4tgd0qnc73d79tvg` (10:46:14.337) then `:34` `dep-darq4thsmc7s73cq5idg` (10:46:14.351) | **both** `canceled`, nothing new rolled out, and the second row's `image.ref` is **`:35`** although that call requested `:34` |
| 3   | `:35` (10:47:01.242) then `:33` (10:47:04.484), 3 s apart                  | correct: `:33` `live`, `:35` `canceled`                                                                             |

The CLI exits 1 for each canceled trigger (`--wait`), so a CI job would report a failed deploy for the run that should have won.

## Mechanism (the exact interleaving is inferred; t001 must pin it)

- `lego/backend/internal/deploys/service.go` `triggerFetched` (`:574` on) has no per-service serialization. Each call writes the row image (`SetAppImage`, `:618-622`, last writer wins, added by w8/022's row-first fix), computes `previousGeneration := a.Generation` / `releaseGeneration := previousGeneration + 1` from its own earlier read (`:616-617`), and patches the CR with `patchApp` (`:238-242`), which uses `client.MergeFrom`, a merge patch **without** optimistic locking. It then opens the deploy row (`CreateDeploy`, `:657`).
- The projector then re-applies the row's (last-written) image onto `spec.image`, which bumps the generation again. The reconciler closes any open deploy whose release generation was superseded, so either or both racing rows can close `canceled`. Row, CR and deploy-row images can each come from a different caller.
- Elsewhere in the same package, the deploy-hook maintenance path already uses `client.MergeFromWithOptions(…, client.MergeFromWithOptimisticLock{})` (`deployhook.go:196,240`), and so does `keyvalue/service.go:592-652`.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs-cli` sweep 18 (w8), 2026-09-26: a concurrency probe after w8/022 made image deploys work.
- **Goal linkage:** correct deploy semantics (ADR006 Render-compatible deploys; "newest deploy supersedes" is Render's model) and trustworthy deploy history (the row must say what ran).
- **Expected outcome:** CI pipelines that fire several deploy triggers in quick succession (a burst of pushes, a hook plus a manual deploy) end with the newest artifact live and honest rows.
- **Why now:** w8/022 just enabled image deploys by hook and CLI, so bursty triggers are now a realistic path, and today the newest can silently lose.
- **Render parity included:** the behavior is visible on REST/GraphQL/MCP deploy rows and the dashboard.

## Unverified

- The exact step that mislabels run 2's second row (the projector vs. the `CreateDeploy` input).
- Repo-backed commit triggers racing each other (image path only was exercised).
- Render's exact tie-break for triggers within milliseconds. Assumed newest-accepted wins, per Render's deploy docs; t005 should confirm.

## Historical blocker (2026-09-27; resolved 2026-10-02)

t001–t007 are done. Deploy triggers on one service are serialized (per-app advisory lock, fresh read, optimistic-lock patch, deploy row inside the same section), so the newest accepted trigger wins and each row records its own image. Only **t008** remains: the live 10-iteration same-second A/B loop on a production fixture. It needs the deploy to land (`blocked/m42`) and a logged-in `bex` CLI.

## Live re-verification (2026-09-27, `/qa-find-bugs-cli` sweep 52)

**DoD holds (2/2 rounds)** on production `4a0422577`, `bex v0.2.1`, human device login, workspace `bex-canary`, free `mendhak/http-https-echo` fixtures (deleted). Two `bex deploys create <srv> --image …:34|:35` launched together: round 1 was accepted at 13:03:36.882 (:35) and 13:03:37.394 (:34), and round 2 at 13:04:24.963 (:34) and 13:04:25.006 (:35), 43 ms apart. Both rounds had the same outcome: the later-accepted trigger went `live`, the earlier one closed `canceled`, each row's `image.ref` equals what its call requested, and the service's `imagePath` matches the live row.

## Live closeout (2026-10-02)

Isolated `bex v0.2.1` QA login, workspace `bex-canary`, owned free image service `srv-davmgjmde41s73canr2g`. **30 qualifying rounds passed: 10 image/image, 10 deploy-hook/deploy-hook, 10 image/restart.** Each pair used different requested image tags (`mendhak/http-https-echo:34` and `:35`); restart selected the prior live artifact. Every earlier accepted row ended `canceled`, every latest row ended `live`, and each row's `image.ref` matched its own request. The public endpoint returned HTTP 200 after every qualifying round. The final running pod independently showed `docker.io/mendhak/http-https-echo:34`, matching the last live row.

Maximum launch gap: **44.4 ms**; maximum server acceptance gap: **321.5 ms**. “Newest” refers to server acceptance, not which shell thread was scheduled first.

An earlier restart round aged into free-tier hibernation: `dep-davmqik5o9vs73dt7pv0` canceled and `dep-davmqimde41s73cansa0` remained queued at the 180-second probe limit. The CR reported `AutoHibernated`; a wake request initially returned 503, then the service woke and resumed deployment. That timed-out probe and the initial same-image restart probes are excluded. The final ten distinct-image restart rounds were rerun consecutively with keep-awake requests. These results certify concurrency on an awake fixture, not free-tier sleep/wake timing.

Saved source settings are a separate m44 contract: current production image triggers still update saved `imagePath`, while restart can select a different live image. The old DoD's saved-image-equals-runtime statement is superseded by the user's accepted per-deploy override decision. This closeout establishes ordering, runtime selection and row attribution; m44 retains its own post-ship saved-settings verification.

| Scenario / round | Earlier (canceled) | Latest (live) | Live tag |
| --- | --- | --- | --- |
| image/image / 1 | `dep-davmjpk5o9vs73dt7p20` | `dep-davmjpmde41s73canreg` | `34` |
| image/image / 2 | `dep-davmjvs5o9vs73dt7p30` | `dep-davmjvude41s73canrfg` | `34` |
| image/image / 3 | `dep-davmk7c5o9vs73dt7p5g` | `dep-davmk7c5o9vs73dt7p60` | `35` |
| image/image / 4 | `dep-davml5k5o9vs73dt7p70` | `dep-davml5mde41s73canrig` | `34` |
| image/image / 5 | `dep-davmld6de41s73canrk0` | `dep-davmldede41s73canrkg` | `35` |
| image/image / 6 | `dep-davmlks5o9vs73dt7p9g` | `dep-davmlks5o9vs73dt7pa0` | `35` |
| image/image / 7 | `dep-davmlsk5o9vs73dt7pb0` | `dep-davmlsmde41s73canrn0` | `34` |
| image/image / 8 | `dep-davmm445o9vs73dt7pc0` | `dep-davmm46de41s73canro0` | `34` |
| image/image / 9 | `dep-davmmc45o9vs73dt7pd0` | `dep-davmmc6de41s73canrp0` | `34` |
| image/image / 10 | `dep-davmmkede41s73canrr0` | `dep-davmmkk5o9vs73dt7pe0` | `35` |
| hook/hook / 1 | `dep-davmmuede41s73canrrg` | `dep-davmmuc5o9vs73dt7peg` | `35` |
| hook/hook / 2 | `dep-davmn5c5o9vs73dt7pfg` | `dep-davmn5c5o9vs73dt7pg0` | `35` |
| hook/hook / 3 | `dep-davmncmde41s73canrs0` | `dep-davmncmde41s73canrsg` | `34` |
| hook/hook / 4 | `dep-davmngs5o9vs73dt7pig` | `dep-davmngude41s73canrt0` | `35` |
| hook/hook / 5 | `dep-davmnomde41s73canrtg` | `dep-davmnok5o9vs73dt7pj0` | `34` |
| hook/hook / 6 | `dep-davmo0c5o9vs73dt7pjg` | `dep-davmo0ede41s73canru0` | `34` |
| hook/hook / 7 | `dep-davmo845o9vs73dt7pk0` | `dep-davmo86de41s73canrug` | `35` |
| hook/hook / 8 | `dep-davmoeede41s73canrv0` | `dep-davmoec5o9vs73dt7pkg` | `35` |
| hook/hook / 9 | `dep-davmom6de41s73canrvg` | `dep-davmom45o9vs73dt7plg` | `35` |
| hook/hook / 10 | `dep-davmotude41s73cans0g` | `dep-davmots5o9vs73dt7pm0` | `34` |
| image/restart / 1 | `dep-davmshk5o9vs73dt7q20` | `dep-davmshmde41s73cansd0` | `35` |
| image/restart / 2 | `dep-davmsp6de41s73canse0` | `dep-davmsp45o9vs73dt7q30` | `35` |
| image/restart / 3 | `dep-davmstude41s73cansf0` | `dep-davmsts5o9vs73dt7q40` | `35` |
| image/restart / 4 | `dep-davmt5mde41s73cansgg` | `dep-davmt5mde41s73cansh0` | `34` |
| image/restart / 5 | `dep-davmtcude41s73cansig` | `dep-davmtcude41s73cansj0` | `34` |
| image/restart / 6 | `dep-davmtkk5o9vs73dt7q50` | `dep-davmtkmde41s73cansl0` | `34` |
| image/restart / 7 | `dep-davmtrude41s73cansm0` | `dep-davmtrs5o9vs73dt7q60` | `35` |
| image/restart / 8 | `dep-davmu3mde41s73cansng` | `dep-davmu3k5o9vs73dt7q70` | `35` |
| image/restart / 9 | `dep-davmubc5o9vs73dt7q80` | `dep-davmubede41s73cansog` | `35` |
| image/restart / 10 | `dep-davmuiude41s73cansq0` | `dep-davmuiude41s73cansqg` | `34` |


**Cleanup verified:** both owned web-service fixtures and the free Postgres fixture were deleted; all four baseline services remained. The isolated CLI credential was revoked, and the temporary local test containers were removed.
