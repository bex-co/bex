# w8 · m46 — Near-simultaneous deploy triggers race: the newest can lose, both can be canceled, and a deploy row can record the wrong image

**Worker:** worker8 **Goal:** concurrent deploy triggers on one service resolve deterministically, Render-style: the most recently accepted trigger wins, every earlier open deploy closes `canceled` (superseded), and each deploy row records exactly the image or commit its caller requested. **Status:** blocked

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
| t008 | Closeout                                                                                                          | 15m | t007       |

## Definition of done

- Two `bex deploys create <srv> --image …:A` / `…:B` launched within the same second (B second): B ends `live` serving B, A ends `canceled`, the service reads `imagePath: …:B`, and **each deploy row's `image.ref` equals what that call requested**. Repeated 10 times in a loop, the outcome is the same every time.
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

## Blocked (2026-09-27)

t001–t007 are done. Deploy triggers on one service are serialized (per-app advisory lock, fresh read, optimistic-lock patch, deploy row inside the same section), so the newest accepted trigger wins and each row records its own image. Only **t008** remains: the live 10-iteration same-second A/B loop on a production fixture. It needs the deploy to land (`blocked/m42`) and a logged-in `bex` CLI.
