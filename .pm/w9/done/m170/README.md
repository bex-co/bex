# w9 · m170 — ADR095 phase 3 — agent-shared output files and the Files tab

**Worker:** worker9 **Goal:** the agent can share files back (`share_file`) and users download them from the conversation, the Files tab or the API. **Status:** done — live acceptance passed 2026-11-05 (see Live acceptance)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | `share_file` in the in-sandbox bex MCP + gateway route — **DONE** | 75m | — |
| t002 | Dashboard: `share_file` cards and the Files tab — **DONE** | 75m | t001 |
| t003 | Expose outputs to API callers — **DONE** | 30m | t001 |
| t004 | Render parity (Devin reference) — **DONE** | 25m | t002, t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004, t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- `share_file` stores a file from /workspace, /tmp or /attachments as an output attachment through the gateway (session-pod identity, no credential in the sandbox); path escapes, directories and oversize files are refused.
- The conversation shows a file card for each shared file; the Files tab lists inputs and outputs with downloads and a matching skeleton.
- REST/GraphQL list outputs per turn; `outcome.files` added if w21's `outcome` exists, otherwise the coordination is recorded.
- Live: a session asked to "write a CSV report and share it" produces a downloadable file in the dashboard.

## Source + Goal linkage

- **Source:** [ADR095](../../../docs/ADR095-agent-attachments.md) (accepted for implementation by user 2026-11-03: "/pm ADR095 to w9"); Devin gap list in [ADR094](../../../docs/ADR094-coding-agent-mcp.md).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillar 5 (cloud coding agents); Devin parity for agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)).
- **Expected outcome:** parity with Devin's `source: devin` attachments — reports, CSVs and screenshots come back without being committed.
- **Why now:** ADR095 D12 phase 3; depends on m168 storage.
- **Dependency:** m168; dashboard pieces reuse m169 components.
- **Parity:** included: REST/GraphQL/dashboard surfaces; Devin is the reference.

## Outcome (2026-11-03)

t001–t006 are done.

**`share_file`** is in the in-sandbox `bex` MCP. The file is read through a single handle, and the allowed roots (/workspace, /tmp, /attachments) are agent ergonomics, not a boundary. It posts to a gateway route, `POST /agent-attachments/<ns>/<sid>` on :8082:
- the route uses the same pod admission as the PR verb (source pod, session label, budgets);
- bex-api accepts a share only from the session's current live sandbox (`CurrentSandboxCaller`);
- the declared SHA-256 is verified before the row becomes ready;
- the session quota is locked;
- the file is stored as an output of the current turn.

**Envelope.** The share envelope is MAC-domain-separated (`share-file\n`) and must carry `kind: "share-file"`. The original verbs refuse any body that carries `kind`, so a captured share cannot be replayed as a model-key mint. `TestAShareEnvelopeCannotMintTheModelKey` covers it.

**Dashboard.** Shared files render as cards after the activity group. A card's metadata comes from the stored output row, never from tool text. The session header has a Files sheet listing inputs and outputs, with a matching skeleton, load-more error handling and fresh-URL downloads.

**Outputs per turn** are available on REST, GraphQL and MCP. `outcome.files` is not added: ADR094's `outcome` block (w21) is not on main yet. Add it when that block ships (noted in ADR095).

**Parity (t004), Devin as the reference.** Devin's `source: devin` attachments map to `direction: output`, listed alongside inputs on every surface. Devin documents no explicit share verb; bex makes sharing an explicit agent action. That is deliberate, per ADR095 D6.

**Simplify (t005).** Two reviewers, a security pass and a reuse/efficiency pass; every finding except the optional shared VerbError was applied:
- shared admission;
- shared signing, nonce and URL helpers;
- one gateway client in the driver;
- the `useFreshDownload` hook;
- lock tightening.

**Verification**
- Real-deps backend suite: 74 packages.
- `make lint` passes.
- Driver: 132/132 tests.
- Dashboard: lint, and 5,797/5,797 tests.

## Blocked (2026-11-03): t007 live check

The live DoD (a session writes a CSV, shares it, and it downloads from the dashboard) needs the production attachments bucket, the same user action as [m168](../m168/README.md).

**To clear:** create the bucket and deploy, then `/loopx w9` runs the check.

## Unblock decision, 2026-11-04 (user decision D1, via `/unblock w9`)

- **Covers:** w9/m168, m169, m170, m171 (shared gate).
- **Decision.** The user authorizes the worker to create the production attachments bucket and its IAM user. The worker runs the existing idempotent script from the repo root:

  ```sh
  BEX_ENV_FILE=../bex9.env bash scripts/agent-attachments-s3-credentials.sh provision
  BEX_ENV_FILE=../bex9.env bash scripts/agent-attachments-s3-credentials.sh install
  ```

  - The root keys are `TF_STATE_ACCESS_KEY` / `TF_STATE_SECRET_KEY`, present by name in `../bex9.env`; never print their values.
  - The script uses the `hetzner-prod` context.
- **Reference.** Devin parity: upload, attach and returned files must work for real users.
- **Done signal.** `bex-system` has the attachments Secret, a deploy rolls bex-api with `BEX_AGENT_ATTACHMENTS_S3_*`, and `GET /v1/agent-sessions/capabilities` reports `attachments.enabled: true`.
- **Worker next step.** m168 runs the provisioning first. m169–m171 wait for that signal, then run their live checks as written in their definitions of done, signing in as the QA user (`scripts/qa-login.sh`).
- **If provisioning fails** (keys missing or rejected, Wasabi denies the call): stop, keep the error message without any secret, and return the item to `blocked/` with the exact failure.

## Live acceptance evidence (2026-11-04, production)

**Files sheet.** On a session it lists the two inputs as "You · turn 1 · 178 B" (screenshot screenshot (removed from git in the 2026-11-04 history rewrite; text above is the record)).

**Not run: `share_file`.** No agent sandbox can start on production right now (`containerd-shim-runsc-v1` missing on `bex-sandbox-cgqq6-vlvqv`; see m168's evidence). Re-run "write a CSV and share it" once sandboxes start.

Note: shared-image cards will also hit the dashboard CSP `img-src` gap recorded in m169.

## Live acceptance (2026-11-05, production at `dcdc774c1`)

- **PASS.** A repo-less session with no attachments asked the agent to write `/tmp/qa-report.csv` and share it.
  - The transcript shows the `mcp__bex__share_file` call returning `{attachmentId, name: qa-report.csv, mediaType: text/csv, size: 26}`.
  - `GET /v1/agent-sessions/{id}/attachments?direction=output` lists it as turn 1, `output`, 26 B. The download is byte-identical to the expected 3-line CSV.
  - In the dashboard at 1280 px, the "qa-report.csv 26 B" card renders after the activity group (it follows the tool steps in DOM order). The Files sheet lists "qa-report.csv · Agent · 1 · 26 B".
- **Bonus.** The session's sandbox started and completed in about 45 s on the replaced gVisor node, which confirms the 074 fix end to end.
