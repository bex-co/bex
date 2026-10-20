# CLI sweep 1030 — overlapping deploy triggers supersede cleanly (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- Two deploy triggers in immediate succession both answer 201. The deploy list then shows three rows: the initial `created` deploy, the first triggered deploy `canceled` (superseded), and the newest `queued`. The older pending deploy is settled as canceled rather than left dangling — the supersede behavior the m119/m121 milestones hardened, observed here from the trigger side.
