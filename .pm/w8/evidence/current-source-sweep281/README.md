# w8/106 — original producer/consumer replay after the fix

The original sweep266 counterexample now passes on source HEAD `f5b0e1170606053f3d6428e82764e011e20ccfd6`, including the actual operator-produced parked state. This is independent source acceptance of fix `b4403670e`, with no production requests or new fixture. [Results and exact commands](results.json), [producer events](producer.jsonl), and [consumer events](consumer.jsonl).

The unchanged [operator probe](operator-probe.go.txt) runs four real lifecycle variants: new image, plain restart, restart with a served configuration snapshot, and that snapshot with a newer saved environment. Each first serves release 3 and then auto-hibernates; all preserve releaseGeneration 3 and activeRevision rev-3. The four emitted App JSON files are included beside this record. The unchanged [backend probe](backend-probe.go.txt) consumes the snapshot-plus-saved App JSON, preserving the module boundary.

Both previously failing parked-completed verdicts now return Live: before and after the deadline. Awake-completed, unserved-parked, and legacy-unknown controls retain their expected outcomes. Existing generation, cancellation, user-suspend, stalled-rollout, build/pre-deploy failure and newly committed served-proof guard tests pass. The two packages pass **43 named parent/subtest events**; this count includes parent events and is not 43 distinct top-level tests.

Replay: save the two `.go.txt` sources as private Go files and overlay them onto `lego/operator/internal/controller/qa266_restart_wake_probe_test.go` and `lego/backend/internal/store/qa266_parked_status_probe_test.go`. Set `GOWORK=off`, `GOTOOLCHAIN=go1.26.8`, and `QA266_PROBE_DIR` to a new private directory. Run the operator command from `results.json` first, then the backend command using that same directory. Replace `<owned-temp>` with the private overlay locations. No repository Go changes are needed.

At sweep281 this did not close [106](../../done/106.md). A fresh hosted two-client restart must settle its exact row Live while parked without a public wake, followed by owned cleanup. The deployed revision is unknown; the recorded platform image pin still names older source, so this replay makes no claim about production rollout. No deployment, release, commit or push was performed by QA.

The later [sweep287 live acceptance](../sleep-pending-env-sweep287/README.md) completes the parked two-client gate and moves106 to done.
