# Changed Blueprint paths — independent source replay, sweep282

HEAD `f5b0e1170606053f3d6428e82764e011e20ccfd6 passes the original unsigned-number compiler probe and the actual canonical-intent patch probe after fixes `809472bdc`and`62c74aae6`. [Exact commands, assertions and records](results.json), [test events](events.jsonl), and [overlay mapping](overlay.json). No hosted request, resource, product edit, commit or push occurred.

The unchanged [numeric probe](numeric-probe.go.txt) admits all 12 root-service, nested-service and env-group cases: signed-64 maximum, first unsigned value, unsigned-64 maximum and its quoted control. Every typed value keeps all digits. Eight additional ungrouped/nested service/group checks preserve the two unsigned values. Five bounded-field controls keep valid values or source-located refusals as expected.

The unchanged [intent probe](intent-probe.go.txt) now observes exactly one canonical write for each changed source, tier and replica count, with the desired CR and one deploy row. The fourth artificial helper case, idleTTL, still observes no row write. The completed field audit confirms there is no supported Blueprint writer for idleTTL, so that direct helper input is excluded from the authored-field acceptance claim and does not create another defect. Disk persistence, row-failure and unmanaged controls are covered by the newly committed tests selected in the same run.

The selected newly committed deferred-reference location regression also passes. There are **38 named parent/subtest pass events** across the selected tests, including parents; no package-wide or hosted claim is made.

Replay: keep the two probe source files private, map them to the destination paths in `overlay.json` using real local absolute paths, set `GOWORK=off` and `GOTOOLCHAIN=go1.26.8`, then run the `results.json` command from `lego/backend`. `<owned-temp>` is a placeholder for the private files. Assertions about the recorded typed values and write counts are preserved in the result interpretation above; the original probes also log observations that must be checked rather than treating a Go pass alone as acceptance.

[104](../../done/104.md) still needs both actual clients to admit the unsigned values against a deployed API. [m66](../../blocked/m66/README.md) still needs hosted inline and persistent-sync source-update replay, real runtime/commit verification, no-op and complete cleanup. The deployed revision remains unknown. These receipts do not close either item.
