# Literal Blueprint values pass; hook acceptance remains partial — sweep229

Both native clients validate each of three fresh Free declarations (distinct initial/regular commands, empty regular command, equal command text). The supported inline Blueprint API provides the setup because the pinned CLI has validation and no apply command. All three initial deployments reach Live/HTTP200 with the exact literal PATH /usr/bin:/bin and a Unicode/newline marker at build and runtime. These are authored App/Blueprint literals, rather than the mutable service env setup used by the earlier native-create fixtures. Secret-file literal transport and live non-REST writes are not covered.

On the distinct-command fixture, published Bex and pinned Render each natively deploy the same original commit with --wait, exit0 and a complete returned Live deployment object. After each, the current pre-deploy log source contains the regular marker and the runtime preserves the exact literal values. This confirms regular-command selection in the two observed later deployments. Initial-hook events report success, but initial marker output is absent after the first rollout; a wider window does not supply it. Success events and missing logs cannot certify its total execution count. No new defect or full m63 closeout is claimed from that absence. Empty/equal later-deploy and reapply controls remain unexercised. A separate runtime counter is the next acceptance measurement.

Native production POST bodies were not intercepted. Exact native argv, stdout/stderr, returned deployment objects and inline API setup bodies/replies are retained. The first setup decoder assumed a Render-shaped service instead of the stack adapter's AppView; the successful response's exact ID was reconciled through own detail without repeating the POST. That harness error is excluded from product findings.

All three owned services and five recorded deploy children are reconciled, with detail/list/former HTTP404, six baseline services and quota6/terminating0. Comparison configs are gone. The existing bex Blueprint row is read only and untouched; service-level repo fields with no top-level repo avoid persistent inline registration. Kubernetes descendant teardown remains unobserved. No QA product edit, commit/push or external submission.

- [Native validation/deploy, inline setup, runtime and cleanup](owned-native-api-runtime-cleanup.json)
- [Independent read-only literal controls](passive-literal-controls.json)
- [First-hook event and log diagnostics](first-hook-readback-diagnostics.json)
- [Fixture and native deploy driver](fixture-and-native-deploy-driver.py.txt)
- [Cleanup driver](cleanup-driver.py.txt)
- [Passive reconciliation driver](reconciliation-driver.py.txt)
- [Versions and boundaries](metadata.json)
