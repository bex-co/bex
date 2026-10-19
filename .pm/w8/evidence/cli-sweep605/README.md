# CLI sweep 605 — `deploys create --commit` with ref names: branch, `HEAD`, slashed branch

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `1a49cb806`, 2026-10-19. Owned fixture `qa605-git` (`srv-dbb9vbqbpdss73ci0n90`, free express-hello-world), deleted at close: REST 404, URL 404, zero `qa605` names. Completes sweep 604's Unrun item.

- **`--commit master`** (a branch name, not the service's configured branch) resolved to that branch's HEAD (`e7a5c566…`), built, and went `live` — a commit deploy can target another branch's tip without changing the service's configured branch.
- **`--commit HEAD`** resolved to the default branch's HEAD (`039c3477…`) and queued normally.
- **`--commit ag/test-branch`** (slashes in the ref) resolved to that branch's HEAD (`a78686b1…`) and queued — the git-ref validation from sweep 604 admits slashes while still refusing whitespace/metacharacters.
- The two later creates queued behind the live `master` deploy under the usual newest-wins semantics; the fixture was deleted with them pending, which also exercised deletion with queued deploys (service, deploys, and URL all gone).

## Unrun

- Tag refs (the example repo has no tags to resolve).
- Commit deploy on a service whose repo became unreachable after creation.
