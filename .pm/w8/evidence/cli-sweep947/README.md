# CLI sweep 947 — Key Value IP allow-list cycle (live, 2026-10-20)

Two disposable free Key Value instances created, cycled, deleted (204s; GETs 404). No new bug filed.

- The PUT body is `{"cidrs": ["203.0.113.9/32"]}` (string list; an object-array guess refused with the Go shape named in the error). Setting one entry → 200 with `ipAllowList` read back and **`public: true`**; bogus CIDR → named 400 `"999.9.9.9/32" is not a valid CIDR`; resetting to `{"cidrs":[]}` → `ipAllowList: []`, `public: false`. The `public` flag is derived from list emptiness — an empty list means no external exposure, which matches the connection-info split (internal vs external strings, sweep 934).
