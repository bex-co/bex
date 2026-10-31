# Qualification follows generator and fixture changes — 2026-10-31

The common workflow now includes native generator/resolver/assembly Go files and the original fixture tree in its push paths. Previously those changes could leave both-architecture generated-image qualification untriggered. The workflow still makes one common publication; it creates no automatic C-runtime publication matrix.

[Root source review and custody](ROOT-REVIEW.json) preserve all33 frozen payloads and exact three-file source. A test parses the actual YAML and checks generator/nested-fixture positives and unrelated controller/backend/PM negatives. The committed pre-fix YAML fails six required selections; the corrected workflow and focused/full package, Python, lint and Actions checks pass. This is trigger/source verification, not actual generated-image or hosted acceptance.
