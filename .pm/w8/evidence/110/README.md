# Missing-host reference error locations — sweep314

[27 native results and inventory](native-results.json) · [four exact native wire captures](native-wire.json) · [nine authored documents and expected coordinates](authored-cases.json) · [source checks/observations](source-results.json) · [failing source regression](source-regression.go.txt).

Installed Bex0.2.1, published Bex0.3.2/a6350da0 and unchanged Render2.27/a764810a7682 validate all nine documents against Bex production in bex-canary / tea-daif693dqjvc73e7as3g. Source7f596cbb3 contains c79cb52fe. Serving revision is unobserved. All27 validity/exit decisions are correct, and the clients return identical decoded responses per case. Six documents have incorrect error origins in all three clients:18 mislocated results. Three controls per client pass their location expectations:9 controls, including the valid self-only reference.

| Case | Correct location(s) | Live location(s), each client |
| --- | --- | --- |
| self-only | valid | valid |
| missing-only | services[0].envVars[0]@9:5 | services[0].envVars[0]@9:5 |
| self-before-missing | services[0].envVars[1]@14:5 | services[0].envVars[0]@9:5 |
| missing-before-self | services[0].envVars[0]@9:5 | services[0].envVars[0]@9:5 |
| two-missing-same-target | services[0].envVars[0]@9:5, services[0].envVars[1]@14:5 | services[0].envVars[0]@9:5, services[0].envVars[0]@9:5 |
| project-self-before-missing | projects[0].environments[0].services[0].envVars[1]@18:9 | projects[0].environments[0].services[0].envVars[0]@13:9 |
| project-two-missing-same-target | projects[0].environments[0].services[0].envVars[0]@13:9, projects[0].environments[0].services[0].envVars[1]@18:9 | projects[0].environments[0].services[0].envVars[0]@13:9, projects[0].environments[0].services[0].envVars[0]@13:9 |
| cross-consumer-valid-reference-before-missing | services[1].envVars[0]@21:5 | services[0].envVars[0]@9:5 |
| cross-consumer-two-missing-same-target | services[0].envVars[0]@9:5, services[1].envVars[0]@21:5 | services[0].envVars[0]@9:5, services[0].envVars[0]@9:5 |

The four additional native forwarding captures include Bex self-before-missing, Render project-two-missing, Bex valid self-only and Render correctly located missing-only. All multipart ownerId/file bytes are unchanged and forwarded only to the fixed production validation endpoint. All return HTTP200; invalid outputs exit1 and valid output exits0. Native error arrays equal server errors, proving server ownership. They add two mislocations and two passing controls:31 total native commands,20 mislocated results,11 matching location/valid controls. Successful pinned decoders omit server-only pricing/plan fields; full successful-plan identity is not claimed. Authorization is excluded.

Current-source diagnostic calls the real ValidateBlueprint with fakeWorkspace/ownershipCtx selecting the normal workspace. It fails six exact authored-location assertions, while the self-only, missing-only and reordered controls pass. The parent also fails. These are intentional regression failures exposing unfixed product behavior, not a passing implementation. [Regression events](source-regression-events.jsonl). The24 named existing checks across13 top-level tests pass, including aggregate051, typed namespace126, zero-write206 and prior100 deferred/current-state location controls. [Existing events](existing-source-events.jsonl), [compatibility events](compatibility-source-events.jsonl). No full backend suite or fixed candidate is claimed.

An initial private overlay target accidentally included the backend path twice; go test matched no test. Its [excluded events](excluded-unmatched-overlay-events.jsonl) are retained and count as zero checks. Correcting only the absolute virtual filename made the actual regression execute; expectations and product source were unchanged.

[Initial native driver](native-driver.py.txt) · [four added layout cases driver](additional-native-driver.py.txt) · [wire capture driver](native-capture-driver.py.txt). Drivers refer to isolated temporary fixture/config locations represented as <owned-temp>. Expected coordinates come from the actual YAML mapping nodes, not message matching.

Validation only: no Blueprint apply, hosting allocation, paid resource, deployment or baseline mutation. Every authored input/copied config and forwarding server/process is removed/reaped. Post native inventory has none of this nonce's authored resource names and retains all six original service IDs. Quotas remain services6 used/0 terminating, PostgreSQL0/0, Key Value0/0. Authentication state remains isolated for the continuing hunt. GraphQL/MCP/dashboard/apply are source-traced and existing-test checked only; their exact live missing-host behavior and authenticated Render hosting are unverified. Finding is scheduled, not implemented or shipped.
