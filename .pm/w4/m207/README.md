# w4 · m207 — Make card-radio pickers work with the keyboard

**Worker:** worker4 **Goal:** the four card-radio producers support focus/selection navigation, one group Tab stop, and meaningful group names across their eight consumers. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Give card radios one complete controlled keyboard contract | 45m | — |
| t002 | Integrate the four producers and audit all eight consumers | 40m | t001 |
| t003 | Render parity | 15m | t002 |
| t004 | Simplify | 10m | t003 |
| t005 | Test coverage | 35m | t003 |
| t006 | Closeout | 10m | t004, t005 |

## Definition of done

- Fresh Key Value Create and Web Create at 1440×1000 and 390×844: focus Free, press Right/Down, and watch focus and the single checked tier move to the next option. Tab leaves the group instead of visiting every tier. Space still selects the focused tier; restore Free without submitting. Each group has its translated visible plan label as its accessible name.
- Fresh Service Create at both widths: the same journey moves from Web to Private in the five-type group; restoring Web preserves its existing form/type behavior. The group has the translated Service Type name. Choosing a type never submits the form.
- On an owned Free Web service's Instance Type page at both widths, Right/Down move focus and the draft tier; Save Changes follows that draft. Restore Free: Save is disabled, the API still says Free, and no plan mutation occurred without confirmation. Preserve the named group and Free/Paid regions.
- Fresh Workspace Create at both widths: Right/Down move from Hobby to Pro in the draft; Tab leaves the group; Space remains functional; restore Hobby without submitting. Preserve the existing Plan name, usage-description relationship, catalog text and creation/payment flow.
- Complete t002's eight-consumer audit and t005's unprobed-case verification with explicit live/local/source dispositions. These five observed surfaces do not certify Postgres/Key Value detail, Workspace Change Plan, other service types, disabled/unknown states or catalog transitions. Complete affected dashboard checks before closeout.

## Source + Goal linkage

- **Source:** continuous functional `$qa-find-bugs w4`, a449/a450, 2026-10-14, muse.env consumed only by the login helper. Inbox w4/300 is promoted here: [original source](source-300.md), [expanded finding](finding.md), [a449 receipts](../evidence/300-plan-radio-keyboard.json), [a450 receipts](../evidence/m207-card-radio-family.json). Severity minor; Tab/Space remains a workaround.
- **Goal linkage:** usable hosting setup/configuration under [ADR008](../../../docs/ADR008-vision.md) and advertised radio semantics, with existing UI shapes under [ADR018](../../../docs/ADR018-render-parity.md).
- **Expected outcome:** keyboard users can select a service type, resource tier or workspace plan using normal card-radio interaction, and can identify each group by name.
- **Why now:** independent live probes verified the missing interaction in every explicit card-radio producer; correcting one grid leaves three producers broken. Four of eight consumers also lack a group name. Existing callbacks update drafts, so the correction belongs in UI interaction.
- **Scope/size:** 2h35 across common interaction, integration/caller audit, parity and meaningful verification. Filing only. The standalone `300.md` is moved, with no duplicate active inbox item or stub.
- **Render parity:** included for the visible correction. Preserve captured layouts and API/save flows; use the existing [W3C radio contract](https://www.w3.org/WAI/ARIA/apg/patterns/radio/). No signed-in Render keyboard replay is claimed.
- **Cleanup:** a450's only runtime fixture stayed Free and was deleted to API/public 404; seven baseline ID families restored. Both sessions revoked, cookies cleared. No workspace, paid tier or billing attempt was submitted.
