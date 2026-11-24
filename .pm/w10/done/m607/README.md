# w10 · m607 — The Render parity ledger is split into per-surface files under `docs/ADR018/`, with ADR018 as the index, and a soft size guard refuses any markdown file well before the 1 MiB blob limit

**Worker:** worker10 **Goal:** no Render parity ledger file is near the 1 MiB blob limit that `scripts/repo-artifact-guard.sh` enforces in ship-gates and CI, every link into `docs/ADR018-render-parity.md` (path and anchor) still resolves, and a growing markdown file is caught by a soft guard long before the hard refusal, so a parity amendment never again fails a ship or turns `repo artifact guard` red on main. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Headroom now: move Services & lifecycle (217 KB) into `docs/ADR018/services-lifecycle.md`, ADR018 keeping its heading as a pointer — **DONE** | 45m | — |
| [t002](done/t002.md) | Split every remaining top-level section (and the preamble's dated paragraphs) into files under `docs/ADR018/`; ADR018 becomes the index — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | Move the code and tooling that read ADR018 with it: the allowlist test, backend-test.yml paths, `ci_test_inputs.py`, `render-schema-drift.sh`, Go comments — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Every `ADR018-render-parity.md#…` anchor in use resolves, and the docs link gate checks anchors into the ledger files — **DONE** | 1h | t002 |
| [t005](done/t005.md) | Soft size guard: ship-gates and CI refuse a tracked markdown file over a soft limit, with a warning band below it — **DONE** | 1h | t001 |
| [t006](done/t006.md) | Pointers: root AGENTS.md, `docs/AGENTS.md`, and the skills that tell agents to amend ADR018 name the ledger files — **DONE** | 30m | t002 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t003, t004, t005, t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t007 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t008 |

Total: **9 tasks, about 6h20m**.

## Definition of done

1. Neither `docs/ADR018-render-parity.md` nor any file under `docs/ADR018/` is over the soft limit t005 sets (512 KB unless t005 records a better number). ADR018 is an index: title, status, legend, method, evidence sources, the headline and one pointer per section.
2. `docs/ADR018-render-parity.md` keeps its path, and every `ADR018-render-parity.md#…` anchor used in the repository on the day t004 runs resolves, checked by `bash scripts/docs-links-validate.sh` and not by hand.
3. Every moved section is byte-identical to the ADR018 text it came from apart from relative links rewritten for `docs/ADR018/` (verified by reversing the rewrites and diffing, as t001 did), and no concurrent amendment that landed during the split is lost.
4. `TestAllowlistCitesADR018Rows` (`lego/backend/internal/rendertest`) and the backend suite pass, reading the allowlisted-divergences table wherever it now lives; `python3 scripts/ci_test_inputs.py` passes.
5. `bash scripts/ship-gates.sh` and `.github/workflows/repo-artifact-guard.yml` refuse a markdown file over the soft limit (with the split as the remedy in the message), proved by the guard's self-test.

## Source + Goal linkage

- **Source:** [w10/054](../054.md), filed by the `/loopx` CI-fix worker after w10/053's `d2d63d5cda` pushed ADR018 at 1,048,634 bytes and turned `repo artifact guard` red on main (fixed by `4609555136`).
- **Goal linkage:** Render parity is the top product principle ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)), and ADR018 is where every workstream records it. A ledger that cannot be amended stops parity work in every workstream at once.
- **Expected outcome:** every workstream keeps amending the parity ledger daily (40–50 KB a day in November) without a ship failing on file size, and the next oversized markdown file is split on a warning, not after a red main.
- **Why now:** ADR018 was 279 bytes under the limit on 2026-11-23. t001 (`455f8f52c7`) bought 217 KB, about four to five days at November's growth, so the rest of the split must land within that window.
- **Render parity task omitted:** the milestone changes docs, repository tooling and a test's input path; no REST, GraphQL, MCP or dashboard surface changes.

## Notes for the worker

- **Concurrency.** Many sessions amend ADR018 every hour. Generate each move with a script from `origin/main` at ship time rather than by hand. On a rebase conflict in ADR018, take upstream's ADR018 and rerun the script, so every concurrent amendment lands in the file its section now lives in. t001 did exactly this with a one-off script outside the repository: find the `## <heading>` line, take everything up to the next `## `, prefix `../` to each relative link outside code spans and fences, point each `#anchor` that left with another section at `../ADR018-render-parity.md#anchor`, write the new file with a one-paragraph header, and leave the heading plus a one-line pointer in ADR018.
- **Directory name.** Files go under `docs/ADR018/`, never `docs/ADR018-<x>.md`: `scripts/adr-registry-validate.sh` refuses two `docs/ADR*.md` files claiming number 018.
- **Sizes on `origin/main` after t001 (2026-11-23), bytes:** ADR018 830,757 · `docs/ADR018/services-lifecycle.md` 218,427. Remaining sections: Deployment sources & IaC ~108 K · Logs ~92 K · bex ahead of Render ~86 K · Identity, workspaces & account ~78 K · Deploys ~59 K · Managed Postgres ~59 K · the matrix preamble ~56 K · Gap backlog ~50 K · Environment & config ~48 K · Render operation omissions ~46 K · Other datastores & storage ~43 K · Platform events & integrations ~27 K · Metrics ~25 K · Custom domains ~23 K · API contract ~20 K · Account deletion ~6 K · Resource-object metadata ~2 K · Evidence sources ~1 K.
- **Other markdown near the limit.** `.pm/w4/README.md` is 492,937 bytes and `.pm/w8/README.md` 329,759 on the same commit. t005's guard will reach `.pm/w4/README.md` first; file a note for it rather than exempting it.

## Closeout (2026-11-23)

Every definition-of-done item holds on the commits that close this milestone:

1. `docs/ADR018-render-parity.md` is an 11,250-byte index. The largest of the 18 files under `docs/ADR018/` is `services-lifecycle.md` at 218 KB, under the 512 KiB warning band.
2. ADR018 keeps its path and every `## `/`### ` heading as a pointer. Every `ADR018-render-parity.md#…` anchor used in the repository resolves, and `bash scripts/docs-links-validate.sh` now checks anchors into the ledger (t004).
3. The moved text was verified by script: masking link targets, each file equals its source, and each rewritten target resolves to the same heading. The move was redone from `origin/main` at each ship (`142e3bdbe4`, then API contract with t003).
4. `TestAllowlistCitesADR018Rows` reads `docs/ADR018/api-contract.md` and passed in the real-DB backend suite of the ship gate; `python3 scripts/ci_test_inputs.py` passes.
5. `scripts/repo-artifact-guard.sh` refuses markdown at 768 KiB and warns from 512 KiB, in ship-gates and in `repo-artifact-guard.yml`, proved by its self-test (t005). `.pm/w4/README.md`, the first file the warning will reach, is [w10/058](../../058.md).
