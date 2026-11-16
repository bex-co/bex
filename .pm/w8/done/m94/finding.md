# Native Python SQLite exports fail because the sqlite3 executable is absent

- **Severity / attribution:** major; operator native image assembly. Ordinary seed/import/export commands cannot deploy using a utility Render supplies.
- **Versions/context:** source `b8aa43fc3c01f82703110950cea767c1f778a9e6`; published Bex0.3.2 and unchanged Render2.27.0/a764810a7682; production api.bex.co, human-device QA grant, bex-canary/tea-daif693dqjvc73e7as3g,2026-10-17, noninteractive JSON. Serving SHA/OCI unknown.
- **Evidence:** [complete actual wire/runtime record](evidence/cli-sweep409/complete-native-failures-controls-runtime-and-cleanup.json), [results and sources](evidence/cli-sweep409/README.md). Both unchanged native creates POST201/exit0, both original first builds fail sqlite3 command not found, and source/program/SQL plus an independent Python-library three-row control pass before the failure. Both one-line install controls Live; six actual readonly CLI exports/library comparisons and six native app-log correlations pass. Local selected-base original127/control0 agrees. Exact cleanup complete.

## Reproduce and expected behavior

Use an isolated human QA login and workspace with verified Free capacity. The [factory](evidence/cli-sweep409/exact-authored-build-start-factory.py.txt) and [program](evidence/cli-sweep409/authored-SQLite-program.py.txt) generate the exact nonsecret BUILD/START values for the freshly verified [source/SQL/rows](evidence/cli-sweep409/fresh-source-SQL-and-three-row-reference.json). Each native client gets a fresh owned name:

```text
bex services create --name qa-YYYYMMDD-NONCE-sqlite --type web_service --plan free --region frankfurt --runtime python --repo https://github.com/git-fixtures/basic --branch master --build-command "$BUILD" --start-command "$START" --env-var PYTHON_VERSION=3.13 --auto-deploy=false --confirm -o json
bex deploys list "$OWNED_ID" --confirm -o json
bex logs --resources "$OWNED_ID" --type build --limit 100 --confirm -o json
```

Both exact commands, complete build bodies and outputs are retained. The original ordinary command is `sqlite3 -bail qa409.db < qa409.sql`; the running service executes `sqlite3 -readonly -json qa409.db "SELECT id,label,n FROM qa409 ORDER BY id"`. Expected: supplied executable works at native build/runtime, first build Live and exact three-row export without an installer prefix. [Render contract](https://render.com/docs/native-runtimes), [SQLite command contract](https://www.sqlite.org/cli.html).

Actual: Python's built-in SQLite library executes the same SQL and returns all expected values; external executable ENOENT and shell command not found stop both builds. This is not a missing Python module or invalid SQL. Add only `apt-get update -qq && apt-get install -y --no-install-recommends sqlite3` before the original shell command. Both native updates0 and control children Live. The controls import real database files and six runtime calls compare readonly external JSON rows with independent library rows and preserve exact built/runtime database hashes. Actual local tested executable/library3.40.1, Debian package3.40.1-2+deb12u2 arm64; Render's exact version is unverified.

## Traced boundary and repair

Pinned upstream `cmd/servicecreate.go:127–162` validates and passes commands through `pkg/service/create.go:130–155,273–283` to `pkg/service/repo.go:82–87`, unchanged POST/v1/services. `lego/backend/internal/apps/rest.go:455–505,1039–1068` accepts native command fields; `apps/service.go:2891–2930` stores AppSpec; `lego/operator/internal/controller/app_controller.go:1040–1060` passes it into build.Options. Exact readbacks prove persistence; no request repair or CLI fork is indicated.

`lego/operator/internal/build/native.go:283–320` assembles selected bases plus existing tool/default steps before source/authored RUN. It has no SQLite CLI supplier; the selected Python3.13 base has a working library but no executable. `native.go:404–407` emits the sole recipe producer; `build.go:1176–1182,936–955` consumes it; `nativeresolve.go:53–120` changes FROM/adds existing companions without supplying SQLite CLI. Actual exported recipes and selected-base execution reproduce the missing program.

Supply one reviewed immutable amd64/arm64 sqlite3 executable plus required library closure, on the existing native image PATH before source/authored build and retained at runtime. Add its pin/version to the freshness inventory and make payload changes invalidate the relevant layer. The tested Debian package is a working counterfactual, not an adopted final supplier; review a self-contained distribution or complete dynamic closure across intended bases. Preserve the healthy Python sqlite3 module, its behavior and existing app libraries instead of replacing it or treating it as missing. Coordinate delivery with m91/m93 when relevant shared supplier code exists; keep their separate consumer/acceptance boundaries. No new API field, SQLITE_VERSION product selector, tenant install toggle or launcher fork.

## Blast radius, dedupe and acceptance

One generator/preparer/BuildJob/resolve chain serves six primary native runtimes/16 reviewed base lines and native static builds. Place the supplier before the static early return; prove build/pre-deploy/runtime where that phase exists. Preserve version signals, roots, explicit env/PATH, cache behavior, Node/Python/Ruby companions and existing tools. Exclude Dockerfile/image/CNB paths. Shared REST/GraphQL/MCP/Blueprint/dashboard authoring contracts remain compatible.

All PM open/done records and targeted sqlite3 native-build history were searched: no SQLite CLI repair/fix found. Done/m82 delivered FFmpeg and did not certify this executable; m91 is PostgreSQL archive clients, m93 is Pandoc documents. Python's existing SQLite library and managed PostgreSQL do not provide this executable. [Research ledger](evidence/cli-sweep409/researched-source-and-dedupe.json).

Regressions must fail for current missing-CLI behavior and pass real generated-image seed/import plus readonly JSON export, exact SQL/source/program/three-row/library controls, required data/libraries, both architectures, runtime with network off, shared primary/companion/static/cache/env/exclusion neighbors and useful SQL failure handling. After publication both original fresh native first creates without install prefix must reach Live and pass runtime/file/log checks and exact cleanup. Existing install controls alone do not meet DoD. Full suites, generated-image local execution, other lines/architectures/live static/pre-deploy remain unverified. Product code unchanged; estimate195m across six tasks.
