# Finding — a Key Value allowlist save that flips external access restarts the store, and can delete its data

Live functional `/qa-find-bugs w4`, pass 8be45a, 2026-10-19 UTC, `muse.env`, workspace `bex-canary`. Production was pinned to `94a76a923136`, and research is on main `fe734f586`. The probes are in [evidence.json](evidence.json). The owned fixture is deleted.

- **Severity:** major. An ordinary networking save causes an unannounced outage, and on a Persistence Off store it deletes every key. The card says "Internal access is unchanged."
- **Repro (as performed):**
  1. Create a Free Key Value from the dashboard with Public access on, then set Persistence mode to **Off**. That change is guarded by the dialog "Delete all data and change persistence?" and a sudo phrase.
  2. Networking: allow your IP (`PATCH /v1/key-value/<id> {"ipAllowList":[{"cidrBlock":"<ip>/32"}]}`, the same verb Save allowlist uses). Status reads `config_restart` and then `available` about 12 s later. `SET qa:8be45a survived` → OK.
  3. **Control:** add a second rule while external access stays on. Status stays `available`, and `GET` → `survived`.
  4. Save an empty list. "Public access" becomes **No**, and status reads `config_restart` for about 42 s. During it, `GET …/connection-info` → `409 conflict: key value credentials are still converging` and Reveal shows no copy buttons.
  5. Allow your IP again. Status reads `config_restart` for about 20 s, then `GET qa:8be45a` → **nil** and `DBSIZE` → **0**.
- **Expected:** a networking save does not restart the store, or it says before saving that it will, as the Maxmemory card ("Saving restarts the store, so connections drop for up to a minute") and the Persistence card do. A store with Persistence Off must never lose its data to an allowlist save without the same typed confirmation that Persistence Off itself requires.
- **Root cause:**
  - `lego/backend/internal/keyvalue/service.go:716-721` `SetIPAllowList` routes through `UpdateKeyValue`. An empty list turns `spec.public` off and a non-empty one turns it on (the card copy: "Saving CIDR rules enables external access… Saving an empty list blocks external connections").
  - `lego/operator/internal/controller/keyvalue_controller.go:711` derives `intent.public` from `spec.public`, and `:866-879` adds `--tls-port 6380`, the TLS cert/key args, the `valkey-tls` container port and the `public-tls` Secret volume to the **pod template** only while `public` is true. Flipping it changes the template, so the StatefulSet rolls the pod.
  - `reconcileKeyValueTLS` (the `!public` branch) deletes the cert-manager Certificate and its Secret. That is why the template must drop the mount, and why turning access back on needs a new certificate.
  - With Persistence Off (`--dir /tmp`, `:861-863`), a restart is an empty store.
  - The card copy, `dashboard/src/features/keyvalue/locales/en.ts:617-619` `keyvalue.networkingHint`, says "Internal access is unchanged."
- **Fix (target behavior):**
  1. After a store has had external access once, turning it off or on again must not roll the Valkey pod. Keep the certificate and the TLS listener, and gate external access where it is already gated, at the SNI proxy's allowlist check and the Service port. t001 confirms this does not weaken the IP gate.
  2. The first enable on a running store still has to add the listener. Before saving, the Networking card says it restarts the store and interrupts connections for up to a minute. With Persistence Off it requires the same typed "deletes all data" confirmation the persistence change uses. The API answers that save with a coded refusal unless `confirm` is supplied, using the `confirm` precedent of protected environments.
  3. "Internal access is unchanged" is shown only where it is true.
- **Blast radius:**
  - Writers of `spec.public` on KeyValue: `UpdateKeyValue` with `IPAllowList`, which serves REST `PATCH /v1/key-value/{id}`, the dedicated allowlist route, GraphQL `setKeyValueIpAllowList` **and `setKeyValuePublic`** (a direct toggle, `keyvalue/graphql.go:258`) and MCP `update_key_value`. Also create with `public`, and the environment IP fan-out `SetEnvironmentIPAllowList` (`service.go:646`). Whether an environment allow-list change can flip a member's `public`, and so restart every member store at once, is unverified; t001 traces it.
  - The same card copy appears for Postgres (`dashboard/src/features/databases/locales/en.ts:793`). Postgres's public toggle only changes CNPG `serverAltDNSNames` (`database_controller.go:881-893`). Whether CNPG restarts instances for that is **unverified**, so it is a separate claim and t001 checks it.
- **Adjacent classes:**
  - Maxmemory and Persistence changes keep their existing warnings.
  - A plan resize ("a rolling update that preserves data") is unchanged.
  - IP allowlist enforcement while external access is on stays as observed: per-address, effective within about 7 s, and IPv6 refused when only IPv4 is listed.
- **Unverified:**
  - The internal client interruption itself (no in-cluster client). It is inferred from the pod roll, `config_restart` and the 409 window.
  - The Postgres sibling.
  - Render's behavior on allowlist changes.
- **Dedupe:**
  - `rg -i 'Internal access is unchanged|allowlist.{0,60}restart|public.{0,40}restart' .pm docs` found no item.
  - `w1/done/m166` made `config_restart` truthful, and `w4/m116` routed the allowlist through the shared patch. Neither covers the restart itself.
- **Not bugs, recorded for the next hunter:**
  - The copied External URL fails with plain `redis-cli -u` because `redis-cli` sends no SNI. ADR021:93 documents this, and the dashboard's CLI command adds `--sni`.
  - The copied values equal the API's.
