# CLI sweep 1068 — Key Value allow-list drives the public flag (live, 2026-10-20)

One disposable free Key Value instance created and deleted (204). No new bug filed.

- Fresh instance: `ipAllowList: []`, `public: false`. PATCH one `{cidrBlock, description}` entry → 200 and the read shows the entry with `public: true` (publicly reachable, restricted to the listed CIDRs). Clearing the list → `public: false` again. The flag is derived from the list, never set independently — same derivation observed in sweep 947, re-verified on the new pin.
