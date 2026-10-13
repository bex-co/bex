# Recent Blueprint fixes — native production acceptance, sweep212

Both published Bex 0.3.2 and unmodified Render 2.27/a764810a7682 validate 61 fresh harmless documents against api.bex.co in the same isolated QA human grant/workspace tea-daif693dqjvc73e7as3g. All 122 validity/exit decisions match the intended current contract. The desired API pin is c1dab29d39c9; deployed API/operator SHA is unknown and is not inferred from the pin. No resource is applied.

- Static diagnostics: 32 checks preserve 16 valid controls and locate all 16 invalid second-rule fields at the exact authored root/project path and line/column.
- Shadowed copies: 16 checks now refuse all eight formerly accepted mixed literal/reference copies, both entry orders, consumer-first and empty literal. Duplicate-literal/unique-key positives and pure-chain negatives keep their decisions.
- Group copies: 16 checks accept linked root/nested/zero/empty literals and direct/local-override controls; missing and unlinked values still refuse.
- Reference diagnostics: 36 checks correctly decide all cases and locate all 28 pooled/missing-sibling refusals at authored env entries, including first/second and root/project placements. Eight direct/known-value controls remain valid.
- Generic env diagnostic neighbors: 8 checks retain correct admission; four service-IMAGE/group-PLAN refusals still mislocate unrelated/nonexistent fields. These are the existing active w8/100 generic branches, not a new issue. Ordinary-key refusal and literal-IMAGE controls retain their decisions.
- Key Value guidance: 14 checks correctly decide three ordinary refusals, the three recommended-property positives and the schema-level password-property refusal through each client. All six semantic refusal messages recommend connectionString/host/port without password; the recommended values validate.

Expected pointers resolve to actual authored nodes; line/column comes from local PyYAML composition of the exact JSON/YAML bytes, rather than an assumed previous fixture line. All 44 fixed static/reference error locations agree. Seven exact native forwarding captures cover corrected static location, shadowed-copy refusal, linked-empty acceptance, reference location, accurate guidance, its port control and the remaining generic group error. Each unchanged native ownerId/file multipart receives HTTP 200; native decoded validity/errors equal the complete API reply. Authorization is omitted. Fixture names are fresh valid lowercase names; original structures/controls are retained.

This removes primary native-deployment gates for existing 098/099/101/102 and the reference-only part of 100. Remaining caller/sibling/current-state/reapply/non-REST acceptance is explicit in the corresponding active notes; none is marked done here. Hosted apply/runtime is untested and the CLI exposes no apply command. All capture configs/servers/threads/children are removed, quota is six immutable baseline services with no terminating resources or datastores. No QA product implementation, commit, push, release or external issue submission occurs.

Evidence:

- [metadata-cleanup.json](metadata-cleanup.json)
- [native-capture-driver.py.txt](native-capture-driver.py.txt)
- [native-matrix-driver.py.txt](native-matrix-driver.py.txt)
- [native-matrix.json](native-matrix.json)
- [native-wire.json](native-wire.json)
- [sweep212-098-KV-guidance-connectionString.yaml](sweep212-098-KV-guidance-connectionString.yaml)
- [sweep212-098-KV-guidance-env-only.yaml](sweep212-098-KV-guidance-env-only.yaml)
- [sweep212-098-KV-guidance-host.yaml](sweep212-098-KV-guidance-host.yaml)
- [sweep212-098-KV-guidance-missing-property.yaml](sweep212-098-KV-guidance-missing-property.yaml)
- [sweep212-098-KV-guidance-password.yaml](sweep212-098-KV-guidance-password.yaml)
- [sweep212-098-KV-guidance-port.yaml](sweep212-098-KV-guidance-port.yaml)
- [sweep212-098-KV-guidance-unsupported-hostport.yaml](sweep212-098-KV-guidance-unsupported-hostport.yaml)
- [sweep212-099-group-copy-linked-empty.yaml](sweep212-099-group-copy-linked-empty.yaml)
- [sweep212-099-group-copy-linked-literal.yaml](sweep212-099-group-copy-linked-literal.yaml)
- [sweep212-099-group-copy-linked-zero.yaml](sweep212-099-group-copy-linked-zero.yaml)
- [sweep212-099-group-copy-local-override.yaml](sweep212-099-group-copy-local-override.yaml)
- [sweep212-099-group-copy-missing-group-key.yaml](sweep212-099-group-copy-missing-group-key.yaml)
- [sweep212-099-group-copy-nested-linked.yaml](sweep212-099-group-copy-nested-linked.yaml)
- [sweep212-099-group-copy-plain-literal.yaml](sweep212-099-group-copy-plain-literal.yaml)
- [sweep212-099-group-copy-unlinked-group.yaml](sweep212-099-group-copy-unlinked-group.yaml)
- [sweep212-100-generic-neighbor-control-IMAGE.yaml](sweep212-100-generic-neighbor-control-IMAGE.yaml)
- [sweep212-100-generic-neighbor-group-PLAN.yaml](sweep212-100-generic-neighbor-group-PLAN.yaml)
- [sweep212-100-generic-neighbor-service-IMAGE.yaml](sweep212-100-generic-neighbor-service-IMAGE.yaml)
- [sweep212-100-generic-neighbor-service-ORDINARY_VALUE.yaml](sweep212-100-generic-neighbor-service-ORDINARY_VALUE.yaml)
- [sweep212-100-reference-project-direct-control.yaml](sweep212-100-reference-project-direct-control.yaml)
- [sweep212-100-reference-project-known-image.yaml](sweep212-100-reference-project-known-image.yaml)
- [sweep212-100-reference-project-missing-image.yaml](sweep212-100-reference-project-missing-image.yaml)
- [sweep212-100-reference-project-missing-plain.yaml](sweep212-100-reference-project-missing-plain.yaml)
- [sweep212-100-reference-project-missing-plan.yaml](sweep212-100-reference-project-missing-plan.yaml)
- [sweep212-100-reference-project-missing-poolword.yaml](sweep212-100-reference-project-missing-poolword.yaml)
- [sweep212-100-reference-project-missing-type.yaml](sweep212-100-reference-project-missing-type.yaml)
- [sweep212-100-reference-project-pool-reference.yaml](sweep212-100-reference-project-pool-reference.yaml)
- [sweep212-100-reference-project-second-pool-reference.yaml](sweep212-100-reference-project-second-pool-reference.yaml)
- [sweep212-100-reference-root-direct-control.yaml](sweep212-100-reference-root-direct-control.yaml)
- [sweep212-100-reference-root-known-image.yaml](sweep212-100-reference-root-known-image.yaml)
- [sweep212-100-reference-root-missing-image.yaml](sweep212-100-reference-root-missing-image.yaml)
- [sweep212-100-reference-root-missing-plain.yaml](sweep212-100-reference-root-missing-plain.yaml)
- [sweep212-100-reference-root-missing-plan.yaml](sweep212-100-reference-root-missing-plan.yaml)
- [sweep212-100-reference-root-missing-poolword.yaml](sweep212-100-reference-root-missing-poolword.yaml)
- [sweep212-100-reference-root-missing-type.yaml](sweep212-100-reference-root-missing-type.yaml)
- [sweep212-100-reference-root-pool-reference.yaml](sweep212-100-reference-root-pool-reference.yaml)
- [sweep212-100-reference-root-second-pool-reference.yaml](sweep212-100-reference-root-second-pool-reference.yaml)
- [sweep212-101-static-project-combined-valid.yaml](sweep212-101-static-project-combined-valid.yaml)
- [sweep212-101-static-project-empty-header-value.yaml](sweep212-101-static-project-empty-header-value.yaml)
- [sweep212-101-static-project-empty-lists.yaml](sweep212-101-static-project-empty-lists.yaml)
- [sweep212-101-static-project-second-header-name.yaml](sweep212-101-static-project-second-header-name.yaml)
- [sweep212-101-static-project-second-header-path.yaml](sweep212-101-static-project-second-header-path.yaml)
- [sweep212-101-static-project-second-route-destination.yaml](sweep212-101-static-project-second-route-destination.yaml)
- [sweep212-101-static-project-second-route-source.yaml](sweep212-101-static-project-second-route-source.yaml)
- [sweep212-101-static-project-splat-redirect.yaml](sweep212-101-static-project-splat-redirect.yaml)
- [sweep212-101-static-root-combined-valid.yaml](sweep212-101-static-root-combined-valid.yaml)
- [sweep212-101-static-root-empty-header-value.yaml](sweep212-101-static-root-empty-header-value.yaml)
- [sweep212-101-static-root-empty-lists.yaml](sweep212-101-static-root-empty-lists.yaml)
- [sweep212-101-static-root-second-header-name.yaml](sweep212-101-static-root-second-header-name.yaml)
- [sweep212-101-static-root-second-header-path.yaml](sweep212-101-static-root-second-header-path.yaml)
- [sweep212-101-static-root-second-route-destination.yaml](sweep212-101-static-root-second-route-destination.yaml)
- [sweep212-101-static-root-second-route-source.yaml](sweep212-101-static-root-second-route-source.yaml)
- [sweep212-101-static-root-splat-redirect.yaml](sweep212-101-static-root-splat-redirect.yaml)
- [sweep212-102-shadowed-copy-consumer-first-pure-chain.yaml](sweep212-102-shadowed-copy-consumer-first-pure-chain.yaml)
- [sweep212-102-shadowed-copy-consumer-first.yaml](sweep212-102-shadowed-copy-consumer-first.yaml)
- [sweep212-102-shadowed-copy-empty-shadowed-literal.yaml](sweep212-102-shadowed-copy-empty-shadowed-literal.yaml)
- [sweep212-102-shadowed-copy-literal-then-reference.yaml](sweep212-102-shadowed-copy-literal-then-reference.yaml)
- [sweep212-102-shadowed-copy-literals-only.yaml](sweep212-102-shadowed-copy-literals-only.yaml)
- [sweep212-102-shadowed-copy-pure-reference-chain.yaml](sweep212-102-shadowed-copy-pure-reference-chain.yaml)
- [sweep212-102-shadowed-copy-reference-then-literal.yaml](sweep212-102-shadowed-copy-reference-then-literal.yaml)
- [sweep212-102-shadowed-copy-unique-control.yaml](sweep212-102-shadowed-copy-unique-control.yaml)
