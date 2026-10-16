# Committed Ruby version files are ignored — sweep354

[Results](results.json) · [full native commands/responses, build logs, version snapshots, API readbacks and cleanup](native-wire-version-runtime-API-cleanup.json) · [primary public-source reference](primary-source-reference.json) · [actual resolver probe](real-resolver-probe.go.txt) / [events](real-resolver-events.jsonl) · [maintained checks](maintained-resolver-events.jsonl) · [fresh main](fresh-main-comparison.json) · [final inventory](final-inventory.json) · [dedupe](dedupe.json).

Research source779c825e2b6b99c8e3b697b20355781880da7fc8; serving revision unobserved. Published Bex0.3.2/a6350da0 and unchanged Render2.27.0/a764810a7682 target api.bex.co with the continuing isolated human identity in bex-canary. Source refresh/pin/help and explicit Free capacity were checked; the installed customer binary remains unchanged.

## Actual version selection

A fresh anonymous Discourse/main reference is commit802125b979b5bb848a7cfc52409ef3176ffd4d82. Its root Gemfile.lock records Ruby3.4.7p58, while its Gemfile allows>=3.4 and<4.1. There is no .ruby-version/.tool-versions/gems.locked. Exact source hashes: Gemfile8485bytes/bf35ad5fddd287ba5844788629ee153bbbef7058770b21bf276738a6829b2cb4; lock55990bytes/5931d9aac02cd610eeb499e9ff520784924e86f896ae6da7ce3ab35f7044579d. The main ref is rechecked immediately before live creation; both diagnostic deployment commits match it.

[Render's specialized Ruby version page](https://render.com/docs/ruby-version) puts the root Ruby-version lock ahead of .ruby-version, then .tool-versions, then the Gemfile directive. Under Bex's deliberately reviewed patch-to-line mapping, this pin selects the supported3.4 line, not the exact vendor patch. The deliberate patch/default/retired-line policies are not findings.

Each client creates one new explicit Free Ruby web service, Frankfurt, autoDeploy off, rootDir omitted/root and NODE_VERSION24 to keep the secondary JavaScript choice stable. No Ruby override, PATH edit or dependency installer is supplied. Native creates exit0 with exact returned workspace/name/runtime/Free/ID and pre-inventory absence. The Bex POST is captured unchanged and returns201; the successful Render create is direct, so no captured status/body claim is made for that transport. [Bex argv](bex-initial-command.sh.txt), [Render argv](render-initial-command.sh.txt).

The repository supplies real version-selection files; a tiny stdlib service supplies the workload. It does not install gems or start Discourse. The authored Ruby probe first asserts both file hashes and absence of alternate Ruby sources, then prints its standalone nonce_SOURCE_LOCK_CONFIRMED output. Its --require-lock flag asserts selected Ruby is3.4. Both first deployments instead run Ruby4.0 and the probe aborts with nonce_LOCK_EXPECTED_3_4_ACTUAL=4.0.x; native historical build logs show both executed markers and build exit1. The harness assertion causes that exit; no Bundler/dependency/app failure is claimed. The underlying product mismatch is the selected major line.

## Diagnostic recoveries

Both native services update commands remove only the --require-lock flag. Exact equality after deleting that one argument, unchanged source/env/start, and one new child each are checked. Both updates exit0, become Live/HTTP200 and retain exact committed files and marker in build/start snapshots. Both phases report Ruby4.0 despite the lock's3.4.7p58; Ruby engine is MRI, other Ruby source files remain absent, Node24 works, and original source commits match the independent reference.

These are healthy wrong-version controls, not a fix or acceptance of the lock pin. No version override, source change, Ruby installation or runtime repair is applied. Six API-only REST/GraphQL/MCP reads agree on the exact Ruby/Free/source/main/build/start settings; MCP explicitly names the QA workspace. [Probe](harmless-Ruby-probe.rb.txt), [native journey](finite-native-journey.py.txt), [shared reads](shared-API-readbacks.py.txt).

## Real resolver and source-policy limits

The unchanged ResolveNativeVersion probe uses the actual public Gemfile/lock and returns4.0 from Gemfile where the lock selects3.4. Its equivalent gems.locked case also returns4.0; an authored .tool-versions Ruby3.3.6 case returns default3.4. These3 input cases fail, plus their parent. Literal Gemfile, .ruby-version and no-signal controls pass (3); no skips/setup failures. Per-case exact choices/images/source labels are retained. Only the root public lock is hosted; alternative lock/tool-version/nested cases are implementation verification.

Four maintained selector/refusal/rewrite/JS-companion tests pass,29 named events and0 skips. Those existing checks do not cover the newly failing inputs. The old Ruby priority test and ADR060 table explicitly prefer Gemfile; this repair must revise that recorded subset/priority against the current specialized provider contract, while preserving the existing reviewed-line policy. m51's accepted literal/file/version-catalog work and m77's companion work remain within their earlier completed scopes. No repository Go file or product implementation is added; the probe uses a virtual overlay.

## Capture qualification and cleanup

An earlier Render call through the owned loopback relay exits1/EOF with no captured POST/ID. No create is blindly replayed. After324seconds, two native inventory processes plus a complete API inventory agree with service usage7/terminating0 and show its unique name absent. The relay/config are closed, that attempt is conclusively unallocated, and only then a fresh nonce is created directly with native0. [Original error and full reconciliation](excluded-local-capture.json), [finite reconciliation logic](excluded-create-reconciliation.py.txt). This local capture failure is excluded as a product finding; it does not replace the successful direct native create evidence.

Both actual services are deleted with native0. Parent details/former URLs and all4 first/diagnostic children are404/absent. Original6 services/original1 Blueprint remain, services used6/terminating0, PostgreSQL/KeyValue0, groups/projects empty. No paid/cron fixture or remote source edit. Per-call configs/request files are removed; the isolated login remains for the authorized continuing loop. Credentials are masked and baseline configuration projected before filing. No browser, full Discourse build, other live Ruby signal class, paid resource or local generated-image execution is claimed. Uncommitted.
