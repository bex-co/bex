# Repeated keys still mislocate the later invalid entry — sweep220

Six diagnostics call the actual current ValidateBlueprint over fake Kubernetes and versioned storage. In both a service and an environment group, VALUE appears first as a valid literal, then again as an invalid value + generateValue entry. The refusal correctly rejects the second entry, but its path and line/column identify the first entry. First-invalid controls locate correctly; both-valid duplicate controls are accepted. This is a remaining occurrence-location gap after the ordinary keyed-location fix 3d3ef082c, within existing issue100.

Twelve fresh commands through published Bex and the unmodified Render pin preserve the exact native multipart requests. All eight invalid commands exit1 with the same errors as HTTP200/valid:false; all four valid controls exit0. Production still returns the older service envVars container or group header location, including first-invalid controls. That older deployment behavior is distinct from the two current-source repeated-key mismatches; deployed revision is unknown. Valid responses include extended server pricing fields that the pinned response type does not expose; the full original wire is retained, and no new decoding issue is inferred.

No hosted apply or resource creation occurred. All forwarding servers and copied configs were removed. The source diagnostic passing exit records execution and the mismatch; it is not a fixed regression test or full suite. Fake IDs were never production mutation or cleanup targets.

- [Six current-source states](current-source-results.json)
- [Current diagnostic](current-source-probe_test.go.txt)
- [Fake storage helper](fake-storage-helper-and-prior-probe.go.txt)
- [Diagnostic events](diagnostic-events.jsonl)
- [Twelve native commands and exact wire captures](native-wire-results.json)
- [Capture driver](native-capture-driver.py.txt)
- [Versions and boundaries](metadata.json)
