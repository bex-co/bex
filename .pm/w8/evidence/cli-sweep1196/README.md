# CLI sweep 1196 — KV allow-list writes carry a data-loss confirmation gate (live, 2026-10-20)

Two disposable free Key Value instances created and deleted (204, verified 404). No new bug filed. No real keys existed in either instance, and the destructive confirmation was never sent.

- The object-entry body form (`[{cidrBlock, description}]`) → 400: this route's documented body is a string list, `{cidrs: [...]}` (the entry-object form belongs to the core CIDR surface, per [keyvalue/rest.go](/Volumes/nvme4tbfish/projects/bex8/lego/backend/internal/keyvalue/rest.go:313)).
- With the correct body, adding a CIDR to a persistence-off instance → 400 with code `KEY_VALUE_RESTART_CONFIRMATION_REQUIRED`: enabling external access restarts the instance, which with persistence off deletes every key, so the API demands a typed `confirm` phrase naming the instance before it will proceed. The allow list stays empty after the refusal.
- A bogus CIDR → 400; an empty `{cidrs: []}` write → 200 (no-op).
