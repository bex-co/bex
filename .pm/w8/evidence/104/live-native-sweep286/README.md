# w8/104 — live two-client acceptance, sweep286

Published Bex0.3.2 and unchanged Render2.27/a764810a7682 now pass **all16 actual native production validation runs** with exit0/valid:true. [Exact manifests, argv, full stdout/stderr and timestamps](native-results.json).

The matrix covers both unquoted9223372036854775808 and18446744073709551615 in root service, project-nested service and env-group values through each client:12 target cases. Signed64 maximum and quoted unsigned64 maximum root controls add four successes. The same first target still failed both clients in [sweep283](../live-native-sweep283/README.md). The original compiler/typed-parser digits and bounded-field controls separately pass in [sweep282](../../current-source-sweep282/README.md).

Source refresh moved to `667943d859ff71ab3f6a085985dcd5b4e272e0c5`, whose API manifest now pins source5c7ee1e5aae6 containing fix809472bdc. Actual deployed artifact identity remains unobserved; the successful behavior is the live acceptance, not an inference from that manifest. Existing implementation records cover the relevant Apps suite. No resource/deploy was created, and all copied Render configs were removed. Native mutation wire, hosted env runtime, live GraphQL/MCP/browser and independent Render production remain untested; these were not the remaining validation gate.

This completes the explicitly parked two-client three-shape gate for [104](../../../done/104.md). The note is moved to done and its board status/links synchronized. QA implemented no product fix and performed no deployment, release, commit or push.
