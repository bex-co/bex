# CLI sweep 1102 — port lives top-level at create, in serviceDetails on read (live, 2026-10-20)

Two disposable free web services attempted; one created (top-level port) and deleted (204), the other refused. No new bug filed.

- Create with `serviceDetails.port` → coded 400 `unknown field "port"`; create with top-level `port: 8080` → 201 and the read reports `serviceDetails.port: 8080`. Port is a top-level create field (mirroring the CLI's create body, per `rest.go`'s comment) that the read model nests into `serviceDetails` — an asymmetry, but the refusal names the problem and the accepted placement works.
