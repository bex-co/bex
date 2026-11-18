# w3 routines

Routines `/loopx w3` runs on schedule whenever the queue is drained; new board items always go first. A command is run verbatim: a slash command with its arguments, a shell command, or a task in plain words. `when: always` (the default) runs every interval; `when: changed` also waits for one of its `paths` (any path when empty) to change and backs off after empty runs. Edit with `/pm routine`.

Budget: 4 runs/day

| name | command | every | when | paths |
| --- | --- | --- | --- | --- |
| pin-gated-recheck | For each item under `.pm/w3/blocked/` whose only remaining gate is a production deploy carrying a named commit, check whether the latest `chore(deploy): pin platform images to <sha>` commit on origin/main contains it (`git merge-base --is-ancestor <commit> <sha>`). For each that does, run `/pm unblock` on it with that finding and ship the board move, so the loop's next pick sends it to its own worker. Leave items gated on a user decision, a date or an outside event alone, and do not run the live checks yourself. | 6h | always |  |
