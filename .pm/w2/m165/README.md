# w2 · m165 — Environment IDs must round-trip through the pinned Render CLI

**Worker:** worker2 **Goal:** IDs returned by environment discovery work as `--environment` selectors for Postgres and Key Value, while existing environment identities, placement, and access controls survive. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Repair the environment ID producer/consumer contract | 50m | — |
| t002 | Verify shared callers, legacy identities, and authorization boundaries | 45m | t001 |
| t003 | Render parity across REST, GraphQL, MCP, and dashboard | 25m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage and live CLI acceptance | 45m | t003, t004 |
| t006 | Closeout | 10m | t005 |

## Definition of done

- Environment discovery returns a canonical ID accepted by the pinned Render CLI's `IsEnvironmentID` (`^evm-[a-z0-9]{20}$`); feeding that exact value into supported Postgres and Key Value selectors succeeds.
- Installed Bex and an unmodified same-pin Render CLI against Bex both pass ID/name controls on fresh disposable fixtures. Exercise Key Value create/get/list and Postgres create/get/list with the discovered ID; verify the remaining shared callers and aliases through meaningful coverage.
- Legacy `env-*` API links still resolve the same identity. Existing SQL membership, App/Database labels, environment-group metadata, Blueprint placement, ACLs, protected confirmation, and authorization remain coherent. No environment recreation or resource move is required.
- REST, GraphQL, MCP, dashboard links, project `environmentIds`, and resource `environmentId` fields agree on the canonical outward ID. Legacy input normalization happens at an explicit common boundary without resource-existence leaks or ambiguous alias collisions.
- Valid name + project selection, no-environment selection, and raw `services --environment-ids` controls continue to work. Name filters keep their ordinary name semantics.
- Relevant tests and lint pass; live acceptance and owned-fixture cleanup evidence are recorded before closeout.

## Source + Goal linkage

- **Source:** User-requested infinite `$qa-find-bugs-cli` hunt filed in `w2`, 2026-10-02. [Finding and complete sanitized evidence](finding.md). Severity **major**; implementation has not begun.
- **Goal linkage:** ADR008's Render-alternative hosting goal, ADR006/ADR018's compatible API, and ADR020's stable identifiers. The imported CLI is the compatibility client, not a fork to modify.
- **Expected outcome:** A customer can copy an environment ID from Bex discovery and use it to scope datastore automation successfully, including environments created before this fix.
- **Why now:** Live production returns `env-*`, but the pinned client recognizes only `evm-*`. This breaks multiple datastore operations before their target request is sent. Changing only future mints would strand all existing environments, so identity compatibility needs explicit treatment.
- **Scope:** One confirmed bug, one repair task, and a separate shared-code/legacy verification task. This is roughly three hours including the standing closing tasks. Render parity is included because IDs cross every tenant-facing surface. No product fix, commit, or push was authorized by the QA request.
