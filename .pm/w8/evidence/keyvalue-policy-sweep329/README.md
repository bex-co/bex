# Native Key Value memory policies and name-only continuity — sweep329

[Complete native commands, PTY output, API receipts and cleanup](native-runtime-cleanup.json) · [finite guarded journey](journey-driver.py.txt) · [bounded PTY helper](bounded-pty-helper.py.txt) · [native help](native-help.json) · [results](results.json).

One fresh explicit Free Key Value is created in bex-canary through published Bex0.3.2/a6350da0, with --memory-policy cache, --persistence-mode off and Frankfurt. Native create exits0 and ID/name/workspace/free/options readback proves ownership, with no baseline Key Value. Source is 5977e4f98c1355cd9dfa5917d3f38d4a49772ea2; serving SHA unobserved. The oracle remains unmodified Render2.27.0/a764810a7682. Local external client is /opt/homebrew/bin/redis-cli8.10.1. Each native kv-cli invocation uses its documented interactive output under a real bounded PTY and -- passthrough; no JSON-mode connection is invented.

## Results

- Bex by ID and unchanged Render by the uniquely resolved owned name both execute PING and return PONG/native exit0.
- Native Bex cache create reaches the actual allkeys-lru policy. Native Bex --memory-policy queue reaches noeviction. Native Render --memory-policy volatile_ttl by name reaches volatile-ttl. API metadata uses the pinned Render underscore enum while real INFO memory reports the engine's hyphenated spelling.
- A scratch SET through Bex returns OK; GET through Render returns the exact nonce value. A valid native Bex rename changes only the display name. Exact GET reads before, immediately after and later through the new unique name all pass. The last GET is 84.7seconds after rename intent. All4 real INFO server reads across this rename window retain one run_id. Persistence stays off and memory policy stays volatile_ttl. This is observed metadata-only continuity, not a general persistence guarantee.
- Bex restores cache/allkeys-lru; the actual running engine and a final Render PING by the new name pass again. Policy changes explicitly enter config_restart before converging; intermediate readiness receipts are retained. No data-retention claim is made across these intentional restarts with persistence off.
- An unsupported memory-policy argument returns nonzero and leaves name/policy/persistence unchanged. An initially generated rename label exceeded the30-character Key Value limit; the server correctly returned400 RESOURCE_NAME_INVALID and fresh readback proved no mutation. Its original command/refusal is retained as a harness exclusion/validation control; a shorter29-character rename is a separate accepted request.

All17 valid external native connection/process/data/config comparisons pass. There is no new confirmed defect. This adds normal current-pin execution evidence to the older [w2/m57 connection acceptance](../../../w2/done/m57/README.md) and the [w8/m58 field-preservation boundary](../../done/m58/README.md), without claiming their full original live DoDs were repeated.

## Setup qualifications

Native create defaults this fixture to private. Two premature native kv-cli attempts actually launch redis-cli against the internal .svc hostname, which is unavailable from the laptop. The UI then remains on its error view until the24-second harness bound. Native completed exit is unknown for these interrupted parents; no native exit124/success is claimed. These are setup exclusions, not an external connection defect. The blank interactive error detail is already [w8/085](../../blocked/085.md), not a new finding.

API-only PATCH public:true configures this exact disposable fixture's external endpoint. The first connection-info read returns409 while publication converges; a later200 proves rediss/external-host/SNI readiness. That200 response's credential-bearing URIs are not saved; only scheme/host and a URI-redacted CLI command receipt are retained. All successful actual Redis operations then run through native kv-cli. Publication is setup coverage, not a native public-toggle flag. This is a regular external connection journey; no other workspace/resource is probed.

The first readiness helper incorrectly expected raw REST persistenceMode at the top level. Native get succeeded and the raw API returned200 with the documented options object; the assertion was corrected in the harness. Raw API responses are unchanged. Native HTTP request bodies were not independently captured in this sweep; command arguments, responses and actual child-client output are retained rather than labeled wire captures. Copies of the isolated Render config are removed after every Render call, and no cache connection credential file is created.

## Source and scope

The pinned pkg/types/keyvalue/maxmemory.go NormalizeMemoryPolicy maps cache/queue to allkeys_lru/noeviction; create/update builders supply the existing options/PATCH fields. Backend keyvalue/service.go renderToCRD/crdToRender normalizes the persisted underscore/hyphen forms and preserves omitted patch fields. The native rediscli view obtains cliCommand and execs the real local binary with the caller's passthrough arguments. These actual runtime results verify that end-to-end path. No product implementation or Go test change is made, and no full module suite is claimed.

Unverified: eviction under load, durability with persistence off, data survival across intentional policy restarts, persistence-mode changes, paid plans, IPv6 separately, browser/GraphQL/MCP writes, valkey-cli fallback and serving SHA. The one-key rename control is deliberately separated from the policy restarts.

## Cleanup

Before deletion, exact owned detail remains Free/off and the run ledger has no dependent fixture. Native Bex delete exits0. Later detail404, empty native Key Value inventory, original six service IDs intact and Key Value quota0 used/0 terminating prove cleanup. Physical pod/socket teardown is not directly measured. The isolated QA session remains for the ongoing loop. This evidence is uncommitted pending another ship instruction.
