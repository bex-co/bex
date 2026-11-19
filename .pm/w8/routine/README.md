# w8 routines

Routines `/loopx w8` runs on schedule whenever the queue is drained; new board items always go first. A command is run verbatim: a slash command with its arguments, a shell command, or a task in plain words. `when: always` (the default) runs every interval; `when: changed` also waits for one of its `paths` (any path when empty) to change and backs off after empty runs. Edit with `/pm routine`.

Budget: 48 runs/day

| name | command | every | when | paths |
| --- | --- | --- | --- | --- |
| cli-qa | `/qa-find-bugs-cli w8 with the bex CLI. When the correct behavior is unclear, learn it from Render (render.com docs and API reference, the pinned Render CLI) before filing.` | 30m | always | lego/cli |
