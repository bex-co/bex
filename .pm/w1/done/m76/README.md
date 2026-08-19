# w1 · m76 — Refactor-review hazard fixes

**Worker:** worker1 **Goal:** close the nine small correctness/observability gaps the 2026-08-19 architecture review surfaced — none is a refactor; each is a targeted fix that stops waiting on refactor decisions. **Status:** done

## Tasks (in order)

| id   | title                                                       | est | depends_on                               |
| ---- | ----------------------------------------------------------- | --- | ---------------------------------------- |
| t001 | KeyValue connect panel: add the canViewSensitive reveal gate — **DONE** | 30m | —                                        |
| t002 | KeyValue detail hook: eager poll before refetch — **DONE** | 20m | —                                        |
| t003 | modelproxy: no-follow CheckRedirect on the upstream client — **DONE** | 20m | —                                        |
| t004 | admissionListener: emit shed metrics on aux gateway listeners — **DONE** | 30m | —                                        |
| t005 | Gateway session gauge undercount + sandboxsse double-count — **DONE** | 45m | —                                        |
| t006 | kv-sni-proxy: SetHealthy on the deletion path — **DONE** | 15m | —                                        |
| t007 | Operator doc-comment repairs (reconcileStaticSite + orphans) — **DONE** | 20m | —                                        |
| t008 | Render parity — **DONE** | 30m | t001, t002, t003, t004, t005, t006, t007 |
| t009 | Simplify — **DONE** | 30m | t008                                     |
| t010 | Test coverage — **DONE** | 45m | t008                                     |
| t011 | Closeout — **DONE** | 15m | t010                                     |

## Definition of done

The keyvalue connect panel refuses to reveal the `redis://` URI without `canViewSensitive` (matching the databases twin); `useKeyValue`'s refresh starts a 3s poll before refetching; the modelproxy upstream client cannot follow redirects; shed connections on the web-shell/sandbox-exec/agent-attach/git/model listeners appear in `bex_ssh_limit_rejected_total`; `bex_ssh_gateway_active_sessions` counts every SessionLimiter slot-holder and `authentications_total` counts one exchange once; deleting the last invalid KeyValue CR updates `bex_kv_proxy_healthy`; `reconcileStaticSite` carries its own doc comment. Each fix has a regression test.

## Source + Goal linkage

- **Source:** 2026-08-19 architectural refactor review §1 (five parallel subsystem deep-dives; ledger artifact: https://claude.ai/code/artifact/fe4af1ce-211f-4109-a541-f0aabd273c73)
- **Goal linkage:** security posture + operability of shipped surfaces (ADR035/ADR062 gateway custody, ADR021 KeyValue parity with the databases UX); keeps the ADR028→ADR073 security lineage's invariants observable.
- **Expected outcome:** the two dashboard drift bugs, the two gateway metric blind spots, the credential-redirect latent gap, and the stuck proxy gauge are all closed with tests, independent of any refactor landing.
- **Why now:** each gap is live in production behavior today; all were found incidentally by the review and none needs the refactors to fix. Render parity is included because t001/t002 change tenant-facing dashboard behavior.
