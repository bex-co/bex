# CLI sweep 1016 — private/worker types fail closed on free (live, 2026-10-20)

Refusal-side probes only; the follow-up name list proves nothing was created. No new bug filed.

- `private_service` and `background_worker` creates with `plan: free` → coded 400 `a {type} requires a paid plan; plan must be one of starter|standard|pro|pro_plus|pro_max|pro_ultra`. Same fail-closed pattern as cron (sweep 982): the gate refuses, names the allowed set, writes nothing.
