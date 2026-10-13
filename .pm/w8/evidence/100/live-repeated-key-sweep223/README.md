# Repeated-key location gap now reproduced live — sweep223

Both published Bex and the unmodified Render pin return HTTP200/valid:false with exit1 for the same second invalid VALUE entry, but highlight services[0].envVars[0] or envVarGroups[0].envVars[0] (5:9), rather than the authored invalid envVars[1] (9:9). Four native mislocations reproduce the two current-source mismatches. First-invalid controls now locate index0 correctly in both clients, and both-valid repeated literals remain accepted: eight controls pass.

All twelve commands have exact unchanged native multipart request/response capture, and the native errors equal the server errors. Sweep220 had returned the older service container/group header even for first-invalid controls. This replay observes a behavior change to the keyed locator while retaining its repeated-key gap. The desired platform pin advanced to 3d3ef082c358, but deployed revision is unobserved; no revision attribution or full wider sweep208 acceptance is claimed.

Six diagnostics rerun actual current ValidateBlueprint with fake Kubernetes/versioned storage at the recorded current source head. The passing diagnostic exit proves execution and records the mismatch; it is not a fixed regression or full suite. No hosted apply, non-REST live verification or resource creation occurred. All forwarding servers and copied client configs were removed. Existing issue100 stays todo; no duplicate or QA product implementation/ship.

- [Twelve live native commands and exact wire](native-wire-results.json)
- [Six current-source states](current-source-results.json)
- [Current diagnostic](current-source-probe_test.go.txt)
- [Diagnostic events](diagnostic-events.jsonl)
- [Capture driver](native-capture-driver.py.txt)
- [Versions and limitations](metadata.json)
