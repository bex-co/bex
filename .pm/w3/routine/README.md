# w3 routines

Routines `/loopx w3` runs on schedule whenever the queue is drained; new board items always go first. A command is run verbatim: a slash command with its arguments, a shell command, or a task in plain words. `when: always` (the default) runs every interval; `when: changed` also waits for one of its `paths` (any path when empty) to change and backs off after empty runs. Edit with `/pm routine`.

Budget: 4 runs/day

The `pin-gated-recheck` row was removed on 2026-11-19 (w5/m502): `python3 scripts/loopx-guard.py board w3` now lists a parked item whose ``pin must carry `<sha>` `` or `date gate:` cleared under `blocked.ready`, and `/loopx w3` takes it first, so no w3 gate is left for the routine to read.

| name | command | every | when | paths |
| --- | --- | --- | --- | --- |
