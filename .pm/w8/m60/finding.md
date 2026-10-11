# Copy command changes log searches and generates invalid time arguments

Why: a command copied from a working log view must reproduce that view's accepted search and time filters.

**Severity:** minor. **Attribution:** upstream Copy serializer. **Status:** todo. User-requested functional CLI QA sweep135, 2026-10-11 UTC; researched filing only. Seven tasks, 2h50m including upstream preparation, shared callers, conditional adoption and verification.

## Native reproduction

Production `https://api.bex.co/v1/`, workspace `bex-canary` / `tea-daif693dqjvc73e7as3g`, isolated human device OAuth. Released Bex v0.3.2 and unmodified Render v2.27.0 use pin `a764810a768202704e7206eb7b87a47211fcd98e`. Inspected HEAD `7911ab99f9977f837fe87ee29ee901fc6afc5fc4`; deployed API/operator revision unobserved.

Fresh Free Python web fixture `srv-db5r0gc9vsac73cmeu3g`, `qa-20261011-8b460d-logcopy`, emitted six known lines prefixed `QA135_8b460d`: `LEFT RIGHT`, `LEFT only`, `RIGHT only`, `LEFT,RIGHT`, `plain`, `unrelated`. The corrected deployment reached Live and HTTP200 returned its exact nonce. [Creation, runtime, emitter and setup qualification](evidence/fixture.json), [program](evidence/program.py.txt), [fixture driver](evidence/fixture-driver.py.txt).

Run each original search interactively, wait for its marker row, then press Ctrl+S. No filter form is opened or applied:

```sh
bex logs --resources "$OWN_ID" --type app --text 'LEFT RIGHT' -o interactive
bex logs --resources "$OWN_ID" --type app --text '"LEFT,RIGHT"' -o interactive
bex logs --resources "$OWN_ID" --type app --text plain --start 30m -o interactive
```

Repeat with the actual same-pin Render executable against Bex, using an equivalent isolated config. [Ten full native captures](evidence/native-copy.json) retain original argv, terminal text, Ctrl+S events and exact clipboard stdin. All ten exit0, without timeout/panic, after normal Ctrl+C dismissal. A PATH-local `pbcopy` recorder intercepts the native clipboard child; the user's clipboard is untouched. [Driver](evidence/copy-driver.py.txt), [recorder](evidence/clipboard-stub.py.txt).

For Bex, Copy emits these arguments verbatim (Render emits the same payload with executable `render`):

```text
--text=LEFT RIGHT
--text=LEFT,RIGHT
--start={2026-10-11 08:30:05.415654 -0700 PDT m=-1799.973905249 0x719618c71250}
```

Copied text was parsed as data with `shlex.split`, then passed to the explicit corresponding native binary with unchanged payload arguments and only `--confirm -o json` added. No shell execution, payload repair or executable substitution occurred. [Journey driver](evidence/journey-driver.py.txt), [complete original controls](evidence/controls.json), [complete copied replays](evidence/replays.json).

| Search | Original Bex / Render | Copied Bex / Render |
| --- | --- | --- |
| One phrase `LEFT RIGHT` | exit0, one exact row / one | exit0, three rows / three |
| One literal `LEFT,RIGHT` | exit0, one exact row / one | exit0, four rows / four |
| `plain`, start `30m` | exit0, one row / one | exit1 before request / exit1 |
| Plain control | exit0, one / one | exit0, one / one |
| Explicit two OR terms `LEFT,RIGHT` | exit0, four / four | exit0, four / four |

Phrase replay durations: Bex2.228s / Render2.186s. Comma:2.282s /2.444s. Time:0.037s /0.031s; stderr begins `invalid argument "{2026-10-11" for "--start" flag: invalid timestamp`. Original phrase:2.571s /2.783s; comma:2.568s /2.678s; time:0.306s /0.325s. Other captured durations are retained in the artifacts.

Expected: shell argument boundaries, CSV element boundaries and accepted time representations survive Copy. Actual: a phrase becomes `LEFT` plus an ignored positional `RIGHT`; a literal comma becomes two OR terms; pointer time becomes a Go struct and is rejected.

The first immediate Bex phrase read after emission returned empty; its exact reread after ingestion returns the expected row. Both observations are retained, and ingestion latency is excluded from this finding. The fixture initially listened on10000 while serviceDetails.port was3000; the recorded own correction uses PORT/fallback3000. Native update created a correction deployment; an additional explicit deploy canceled queued setup children. These are harness setup observations, excluded from the finding.

## Producer and consumer

At the exact pin, `pkg/command/inputs.go:373–442`, `InputToString` dereferences pointers at395–401 before formatting; joins slice elements with bare commas at406–415; emits raw scalar/positional/flag text at420–435; then joins arguments with spaces at439–442. It has no shell quoting, CSV encoder or command flag-type context. Dereferencing `*TimeOrRelative` bypasses its pointer-receiver String method (`pkg/command/time.go:31–41`). Merely calling that method would inherit the separately filed082 fractional precision loss.

`pkg/command/formatter.go:40–45` feeds this output into CommandName. `pkg/command/wrapper.go:129–145` stores it in ModelWithCmd.Cmd, and `pkg/tui/stack.go:159–163` passes it unchanged to clipboard.WriteAll. Logs reach this path through `pkg/tui/flows/log.go:63–73` and87; TailLogsFlow at54–59 also creates a pointer StartTime.

`cmd/logs.go:42–56` parses LogInput and consumes it without rejecting spare positional arguments. `--text` is StringSlice at77; `--start` is the actual time flag at75. The real `NewLogsCmd(nil)` flag parser plus `command.ParseCommand` reproduces these changed values and invalid time errors without executing a command or making network calls. [Nine formatter cases](evidence/serialize-results.json), [real parser results](evidence/parse-results.json), [formatter source](evidence/serialize-probe.go.txt), [parser source](evidence/parse-probe.go.txt), [parser input](evidence/parse-args.json), [module](evidence/go.mod.txt), [checksums](evidence/go.sum.txt). To rerun either probe, copy its source to main.go and module/checksums to go.mod/go.sum; parser probe reads parse-args.json on stdin. Unicode commas, embedded quotes and CRLF here are local model evidence, not additional live journeys.

[Mechanical stack-entry census](evidence/stack-callers.json):42 AddToStack/AddToStackFunc sites in22 production files, including11 workflow flow entries and a workflow create entry outside default supported hosting coverage. [View-field census](evidence/view-fields.json):55 tagged fields, including all11 string lists and both time pointers in LogInput. These counts are review scope, not a claim every entry is visible, affected or live tested. Hidden commands suppress CommandName. Inspect input types, aliases and reachability before changing shared serialization.

Actual native HTTP request bytes were not intercepted. CLI outputs preserve complete owned log objects; the real parser proves the upstream failure before the backend. No REST/GraphQL/MCP/dashboard compensation is proposed.

## Proposed correction and verification

Prepare an upstream command-aware Copy formatter reached through CommandName, without breaking the exported InputToString API. Build discrete argv tokens, preserving positional order, nil/default semantics and accepted pointer values; emit shell-quoted tokens for the supported platform shells. For StringSlice flags use the actual CSV grammar; retain element boundaries and accepted bytes. For StringArray flags emit repeated flags, and retain enum list behavior rather than treating all slices alike. Consult the actual effective flag metadata. Time flags must retain relative text or use RFC3339Nano for absolute values; coordinate with082 without duplicating its shared String fix. Verify the formatter against real pinned flag parsing, including positional spaces, quotes, Unicode, literal commas, ordinary lists, repeated arrays, nil/default and explicit empty neighbors. CRLF must be assessed after the existing parser boundary; do not claim recovery of bytes already changed by077.

Repeat all three native failures plus both passing controls on fresh owned Free fixtures with an adopted upstream pin. Verify automatic TailLogsFlow time values locally; live managed-datastore tails remain deliberately unverified under083. Check counted callers and command aliases, preserving current endpoint/configuration and executable identity. Submission, acceptance, pin adoption and release are explicit later dependencies; a local patch alone cannot satisfy Done. No fork or vendored correction, under DO_NOT_DO's CLI rule.

## Dedupe and cleanup

Searched open, blocked and done PM records for InputToString, copied command, clipboard and quoting. m57 targets form.go Apply producer/decoder;082 targets TimeOrRelative.String fractional loss; blocked077 targets StringArray getter CRLF/empty parsing. None fixes Copy's serializer. Documented branding residuals are excluded; native Bex Copy actually emitted `bex`, and no branding regression is claimed. Targeted CLI history has no Copy correction.

[Upstream status](evidence/upstream-status.json): current main `c4d106cfdaf8eec6d23475155e43f4baeeed2013` has byte-identical InputToString-to-EOF source. Three read-only repository issue searches (clipboard+quote, InputToString, copy command) returned zero. This is not exhaustive; dedupe again before separately authorized submission. No upstream issue, PR or message was sent.

[Cleanup](evidence/cleanup.json): all three creation/correction deploy children enumerated, exact native delete exit0/1.153s; detail/public runtime404; own ID absent; six baseline IDs present; quota services6 used/0 terminating, datastores0. Direct Kubernetes descendants were not observed. PTYs and copied configs removed; ongoing hunt's isolated grant remains active. Twenty sanitized durable artifacts. No product implementation, commit or push.
