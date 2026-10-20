# CLI sweep 637 — notification PATCH dialects: one writable enum, one read-only projection

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixture `qa637-web` (`srv-dbbb4gavfpec73dm0ieg`, free hello-app), deleted at close: REST 404, zero `qa637` names. REST boundary (no CLI notification verb at this pin). Extends sweep 563.

- **Writable enum:** `notifyOnFail` accepts exactly `default|notify|ignore` — boolean-ish spellings (`"true"`, `"false"`, a JSON `true`) are refused, strings with the enum list in the message, the boolean as an invalid field value. `notify` and `ignore` both stored and read back.
- **The second enum is read-only here:** `notificationsToSend` in a PATCH body → `400 request body contains unknown field "notificationsToSend"` for every value. Yet it moves on its own: after `notifyOnFail: "ignore"`, reads showed `notificationsToSend: "none"` — the two read fields are projections of one stored policy (sweep 563's finding, now shown to be driven by the writable side).

## Unrun

- Whether a failing deploy under `ignore` actually suppresses the notification email (no mail observation available to this loop).
- Workspace-level notification defaults that `default` defers to.
