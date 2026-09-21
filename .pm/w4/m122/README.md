# w4 · m122 — The events feed reports failed operations as accomplished facts: five "Custom domain verified" rows for a domain that never verified

**Worker:** worker4 **Goal:** a row in the Activity feed means the thing it names actually happened — a verb that was authorized but then failed produces no event, and the domain lifecycle's real transitions (claim created, claim promoted to verified, claim removed) are what the feed reports. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on   |
| ---- | ----------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Decide the mechanism: gate the audit→event projection on operation outcome, or excuse the intent verbs and record facts | 40m | —            |
| t002 | Stop `apps.VerifyDomain` from minting `custom_domain_verified`, and emit the fact on actual promotion | 45m | w4/m122/t001 |
| t003 | Do the same for `apps.AddDomain` / `apps.DeleteDomain`, which fire on refused calls today   | 35m | w4/m122/t002 |
| t004 | Audit the remaining 45 `eventTypes` verbs for routine post-authorization failure and place each | 45m | w4/m122/t001 |
| t005 | Render parity — the feed's event vocabulary and the domain rows across REST/GraphQL/MCP/UI  | 30m | w4/m122/t004 |
| t006 | Simplify — `/simplify` over the code this milestone changed                                 | 25m | w4/m122/t005 |
| t007 | Test coverage — a failed verb produces no event; a real transition produces exactly one     | 40m | w4/m122/t005 |
| t008 | Closeout — close the milestone once the definition of done actually holds                   | 15m | w4/m122/t007 |

## Definition of done

- **A failed verification leaves no trace in the feed.** On a service with a pending, unverifiable custom domain, clicking **Re-check** five times (or calling `POST /v1/services/<id>/custom-domains/<name>/verify` five times, each answering `409 DOMAIN_OWNERSHIP_PENDING`) adds **zero** rows to `GET /v1/services/<id>/events`. Today it adds five, every one of them typed `custom_domain_verified`, rendered by the dashboard as "Custom domain verified".
- **A real verification produces exactly one row.** When the TXT record is in place and `VerifyDomain` promotes the claim, the feed gains exactly one `custom_domain_verified` row, and a second Re-check on the now-verified domain (which returns the claim without re-promoting) adds none.
- **A refused add leaves no trace either.** `addCustomDomain(name: "<another service's platform host>")` returns `400 … is a reserved platform hostname` and produces **no** `custom_domain_added` event. Today it produces one — verified live on 2026-09-21, where one refused add plus one accepted add yielded two identical `custom_domain_added` rows at the same second.
- **Delete matches.** A refused or no-op `deleteCustomDomain` produces no `custom_domain_removed`; a real removal produces one per removed claim (see the pairing asymmetry recorded in `.pm/w7/done/050.md:259` — that asymmetry is a **separate** known behavior and this milestone must not silently change it; t003 states explicitly whether it does).
- **The rest of the vocabulary is placed, not assumed.** t004's output is a written table of all **48** `eventTypes` entries, each classified as _cannot routinely fail after authorization_ / _can fail and is now gated_ / _is an intent verb and is now excused_, with the reason. No entry is left unclassified.
- **The doctrine is recorded where the next person will read it.** The comment block above `eventTypes` (`lego/backend/internal/events/service.go:279-307`), which already explains the deliberately-absent verbs and `w4/m118`'s intent-verb rule, gains the outcome rule too, so the next verb added to the map is placed correctly by default.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` 2026-09-21 pass 107 (w4-targeted, `muse.env` credentials), browser-driven against `https://dashboard.bex.co`, journey 5 (custom domain add + verification). Fixture `qa-20260921-dom` (`srv-daoe0ah2dbts73fi2k80`), since deleted.
- **Goal linkage:** ADR038 (events) and the Activity feed's contract as a truthful record of what happened to a service. Direct continuation of **`w4/m118`** (2026-09-21), which established the repo's rule that _intent verbs are not events_ and moved three cron verbs into `excusedVerbs` — this is the same rule applied to a case it did not reach. The mechanism was already observed and explicitly **recorded, not filed** in `.pm/w7/done/050.md:259-260` ("Events are minted per **authorized verb**, not per row"); that note covered a pairing asymmetry, not a failed operation reported as a success.
- **Expected outcome:** a user debugging why their custom domain will not serve stops being told, five times over, that it was verified. More generally, the feed stops being a log of _attempts that passed authorization_ and becomes a log of _things that happened_ — which is what every surface that reads it (dashboard Activity, `GET /services/{id}/events` on REST/GraphQL/MCP) presents it as.
- **Why now:** verification is the one domain operation whose failure is the **normal** case — a Re-check before the DNS record propagates is the expected first outcome, so the wrong event is emitted on the common path, not an edge case. And the feed is the only place a user can look: the domain row itself just says "Pending", so the feed is what they read next, and it contradicts the row.
- **Render parity task included** — the event vocabulary is exposed on REST, GraphQL and MCP (`GET /services/{id}/events`) and rendered by the dashboard, so the change touches all four.

## Evidence (live, 2026-09-21)

**1. Five verifications, five failures, five "verified" events.** `qa-20260921.example.com` added to `srv-daoe0ah2dbts73fi2k80`. Every verify call answered 409:

```
POST /v1/services/srv-daoe0ah2dbts73fi2k80/custom-domains/qa-20260921.example.com/verify
→ 409 {"code":"DOMAIN_OWNERSHIP_PENDING","error":"domain ownership TXT record is not verified",
       "id":"conflict","message":"domain ownership TXT record is not verified",
       "params":{"recordHost":"_bex-challenge","recordName":"_bex-challenge.example.com","recordZone":"example.com"}}
```

GraphQL (what the dashboard's Re-check button calls) answers the same, with the code in `extensions`:

```json
{
  "data": { "verifyCustomDomain": null },
  "errors": [
    {
      "message": "domain ownership TXT record is not verified",
      "path": ["verifyCustomDomain"],
      "extensions": {
        "code": "DOMAIN_OWNERSHIP_PENDING",
        "recordHost": "_bex-challenge",
        "recordName": "_bex-challenge.example.com",
        "recordZone": "example.com"
      }
    }
  ]
}
```

The domain's own row stayed **Pending / Pending** throughout. The Activity tab, read immediately after, showed:

```
Custom domain verified   5m
Custom domain verified   5m
Custom domain verified   6m
Custom domain verified   6m
Custom domain verified   6m
Custom domain added      8m
```

Five rows, five failures, zero verifications. `GET /v1/services/<id>/events` confirms the type is `custom_domain_verified` with `details: {}`.

**2. The same mechanism on add — a refused call gets an event.** Two calls, one refused and one accepted:

```
addCustomDomain(name:"block-eden-mono.onbex.co")  → errors: ["bad request: \"block-eden-mono.onbex.co\" is a reserved platform hostname"]
addCustomDomain(name:"qa-20260921.example.com")   → {"name":"qa-20260921.example.com","ownershipStatus":"pending"}
```

`GET …/events?limit=6` two seconds later:

```json
[
  { "t": "custom_domain_added", "at": "2026-09-21T07:53:52Z" },
  { "t": "custom_domain_added", "at": "2026-09-21T07:53:52Z" },
  { "t": "service_hibernated", "at": "2026-09-21T07:52:35Z" }
]
```

Two `custom_domain_added` rows for one add. `block-eden-mono.onbex.co` belongs to a **different** service and was never attached to this one.

## Root cause

Two files, and the second is where the decision belongs.

- `lego/backend/internal/core/audit.go:918-933` — `Base.emit` is called from the **authorize** step and builds the row as `Outcome: AuditAllowed` unless `authzErr != nil`. `AuditOutcome` is documented (`:30-35`) as "the result of an authorized **write attempt**" with values `allowed` / `denied` — an _authorization_ verdict, not an operation result. The audit log is therefore correct on its own terms: the caller really was allowed to attempt `apps.VerifyDomain`. Nothing here needs to change for the audit log's own purpose, and t001 must decide whether to add an operation outcome here or to stop projecting these verbs at all.
- `lego/backend/internal/events/service.go:308-359` — `eventTypes` maps that intent verb onto a past-tense fact: `"apps.VerifyDomain": TypeCustomDomainVerified` (`:341`), `"apps.AddDomain": TypeCustomDomainAdded` (`:339`), `"apps.DeleteDomain": TypeCustomDomainRemoved` (`:340`). The projection is unconditional — `ev.Type = eventTypes[r.Verb]` at `:806`, with `:433` the filter's mirror. The map has **48** entries (`awk '/^var eventTypes/,/^\}/' … | grep -c ':.*Type'`).
- `lego/backend/internal/apps/domains.go:683-750` — `VerifyDomain` itself is correct: it returns the coded 409 and, on failure, records only `RecordDomainVerificationAttempt`. It never promotes. So the event is minted upstream of the verb's own result and cannot be suppressed from inside it.
- There are **no** domain facts to fall back on: `allFactTypes` (`events/service.go:370-396`) lists 19 types and none of them is a domain type, so excusing the three verbs without adding facts would make a real verification invisible — which is exactly the regression `w4/m118` was fixing in the other direction. t002 must add the fact in the same change that removes the verb.

## Blast radius

- **Webhooks and notifications are not affected.** `grep -rn --include='*.go' "TypeCustomDomain" lego/ | grep -v _test | grep -v events/service.go` → **0** hits, and `eventvocab` carries no domain types. The three domain event types reach exactly one place: the events feed, read by the dashboard Activity tab and by `GET /services/{id}/events` on REST, GraphQL and MCP.
- **The other 45 verbs are the real scope question, and t004 owns it.** Every verb in `eventTypes` is projected the same way, so any of them that can fail after passing authorization has the same defect. The three domain verbs are the ones proven live; the rest are a claim until t004's table exists. Verbs to look at first are the ones with a validation step after authorization — `apps.SetIPAllowList`, `apps.SetPlan`, `apps.Scale`, `apps.SetAutoscaling`, `secrets.SetEnvVar` (which `w4/m120` just taught to refuse a blueprint-owned key) — but that list is a starting point for the audit, not its conclusion.
- **Verbs that already behave correctly need regression tests, not just the broken ones.** A verb that cannot fail after authorization is correct today _because_ of the unconditional projection; if t001 picks the outcome-gating mechanism, every one of them changes code path.

## Adjacent classes

t001's decision must state where each of these lands, because they are all "the verb did not do what its name says":

- **authorization denied** — already `AuditDenied`, already not projected (the store filters to allowed rows); stays out.
- **operation refused (4xx after authz)** — the bug; must produce no event.
- **operation failed (5xx / infrastructure)** — must also produce no event, and must not swallow the audit row.
- **operation was a no-op** (re-verifying an already-verified domain, re-adding an existing claim — `addCustomDomain` returned 200 for the already-pending claim in the evidence above) — no state changed, so no event; this is the case most likely to be missed by an outcome flag that only distinguishes error from success.

## Unverified this run

- Only the three domain verbs were exercised live. The other 45 entries in `eventTypes` were read, not probed — t004 is the work of placing them, and nothing in this filing should be read as a claim about their current behavior.
- Whether the dashboard's Activity row for `custom_domain_verified` names the hostname was not established: the REST payload carried `details: {}`, and the rendered row read "Custom domain verified" with no domain name. If the row never names the domain, the harm is smaller than it looks on a single-domain service and larger on a multi-domain one; t002 should check and say.
- `w6/m122` (done) fixed the dashboard **hiding** `custom_domain_verified`; this milestone is about the event being **emitted** when nothing was verified. The two do not overlap, but m122's fixture-sharing note (`.pm/w6/README.md:165`) is worth reading before touching the events catalog.

## Also checked this run, and found correct

Recorded so the next pass does not re-walk them:

- **The Re-check button does give feedback.** A first reading called it silent; re-probed at 300 ms–2.5 s, it reliably shows "qa-20260921.example.com isn't verified yet — DNS may still be propagating." The earlier reading sampled ~9 s after the click, past the toast's ~4 s lifetime. `use-custom-domains.ts:245-270` handles `DOMAIN_OWNERSHIP_PENDING` explicitly.
- **The TXT instructions are right.** For `qa-20260921.example.com` the panel says Host `_bex-challenge` in the `example.com` zone, which matches `ownershipDNSRecordName` (`apps/domains.go:86-97`) resolving `_bex-challenge.<registrable-domain>` — the relative-vs-FQDN trap `w4/092` already fixed.
- **Free-tier sleep and wake work.** With `idleTTLSeconds: 60`, `service_hibernated` fired, three requests over 7 s returned 503 and the fourth returned 200 — an ~11 s wake. The 503s are the activator's designed wake page with a JS auto-reload (`lego/operator/cmd/activator/main.go:18`), not a failure.
- **`dashboardUrl`'s short routes resolve.** `/r/<red-…>` and `/web/<srv-…>` both redirect to the real routes.
- **Key Value end to end.** Create → Available, connection info matches REST exactly, and the published `redis-cli --sni …` command answers `PONG` against the public TLS endpoint; 201 keys and 487 commands drove Disk/Memory/Connections charts and the Logs tab.
