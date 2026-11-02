# Native Markdown builds fail because Pandoc is absent

- **Severity / ownership:** major; native image assembly in the operator. Ordinary document-generating builds cannot deploy using a tool Render supplies.
- **Versions/context:** source `3d73a5515bcc0f9e9e1ed6b42056396cdabaa450`; published Bex0.3.2 and unchanged Render2.27.0/a764810a7682; production api.bex.co, human-device QA grant, bex-canary/tea-daif693dqjvc73e7as3g,2026-10-17, noninteractive JSON. Actual serving SHA/OCI unknown.
- **Evidence:** [complete runtime/wire record](evidence/cli-sweep407/complete-native-failures-controls-runtime-and-cleanup.json), [results and sources](evidence/cli-sweep407/README.md). Both unchanged native creates POST201/exit0, original first builds fail missing pandoc, exact source/document/program before-probes pass, tool ENOENT and URLs404. Both one-line install controls reach Live and six fresh runtime conversions/independent HTML parses pass, with six exact native log correlations. Local selected-base original127/control0 agrees.

## Reproduction and expected behavior

Use an isolated human QA CLI login and accessible workspace with verified Free capacity. The [command factory](evidence/cli-sweep407/exact-authored-build-start-factory.py.txt) and [program](evidence/cli-sweep407/authored-document-program.cjs.txt) produce exact credential-free BUILD/START values for the freshly verified [source/document](evidence/cli-sweep407/fresh-source-and-document-reference.json). Run both native binaries separately, each with a new exact ledger-owned name:

```text
bex services create --name qa-YYYYMMDD-NONCE-docs --type web_service --plan free --region frankfurt --runtime node --repo https://github.com/git-fixtures/basic --branch master --build-command "$BUILD" --start-command "$START" --env-var NODE_VERSION=24 --auto-deploy=false --confirm -o json
bex deploys list "$OWNED_ID" --confirm -o json
bex logs --resources "$OWNED_ID" --type build --limit 100 --confirm -o json
```

The actual commands, complete build bodies, resource IDs and output are retained in the evidence. Render uses the same supported flags through its own isolated config. The ordinary authored command is `pandoc --from=markdown --to=html5 --wrap=none qa407.md --output=qa407.html`. Expected: first build succeeds, service Live and fresh runtime conversions work without an installation prefix. [Render contract](https://render.com/docs/native-runtimes), [Pandoc command contract](https://pandoc.org/MANUAL.html#using-pandoc).

Actual: both initial builds terminate with command not found. Change only one build line to `apt-get update -qq && apt-get install -y --no-install-recommends pandoc` before the original conversion. Both native updates0/control deployments Live, source/document unchanged. The controls observe Pandoc2.17.1.1; no claim about Render's exact installed version. Hosted four JSON conversions/two direct HTML bodies independently preserve all expected elements/UTF8 and exact output hashes. All resources/children/URLs/local state cleaned and capacity restored.

## Root cause and concrete repair

Pinned upstream `cmd/servicecreate.go:127–162` validates and passes commands to `pkg/service/create.go:130–155,273–283`, then `pkg/service/repo.go:82–87` sends the unchanged POST. `lego/backend/internal/apps/rest.go:455–505,1039–1068` accepts nested native command fields; `apps/service.go:2891–2930` stores them in AppSpec; `lego/operator/internal/controller/app_controller.go:1040–1060` passes them into build.Options. Readbacks prove persistence; request rewriting or a CLI fork is not indicated.

`lego/operator/internal/build/native.go:283–320` assembles the selected toolchain base plus existing Node/media/default steps, then source and the authored RUN. It contains no Pandoc supplier. `native.go:404–407` emits that one producer into the preparer; `build.go:1176–1182,936–955` consumes the prepared native recipe. `nativeresolve.go:53–120` changes the selected base and adds existing companions, with no document converter. Both actual exported recipes and the selected pinned Node base reproduce that absence.

Add one reviewed, immutable, amd64/arm64 Pandoc supplier to that shared native assembly before source/authored build, retaining the executable and required data/libraries on runtime PATH. Pin it in the existing freshness inventory; version/payload changes must invalidate its layer while ordinary source/env changes preserve upstream layers. Review the [official supplier metadata](evidence/cli-sweep407/official-unexecuted-supplier-candidates.json):3.12.1 has both architecture tarballs with recorded SHA256s. They are unexecuted candidates, not adopted/tested fixes. [Official installation guidance](https://pandoc.org/installing.html#linux) describes standalone binaries and the C Lua-module limitation; verify any chosen distribution against its actual support scope and every intended base. Do not merely copy a dynamic executable without its library closure or assert all filters/PDF engines work.

## Blast radius, dedupe and regression gates

One generator/preparer/BuildJob/resolve chain serves six primary native runtimes and16 reviewed base lines, plus native static builds. Place the supplier before the static early return; prove build/pre-deploy/runtime availability where that phase exists. Preserve Node/Python/Ruby companions, version signals, roots, env/PATH and cache semantics. Dockerfile, image and CNB paths stay outside this native supplier. No API schema/toggle/launcher change is needed. REST/GraphQL/MCP/Blueprint/dashboard consume the shared behavior; their metadata/input contracts stay compatible.

All PM open/done records and targeted Pandoc fix history were searched: no repair item/fix found. Done/m82 deliberately delivered FFmpeg only and did not certify this utility family;406 independently proves ImageMagick already present. m91 concerns PostgreSQL archive clients, with distinct programs/dependencies/consumer. Yarn/uv/pnpm/Bun/Ruby/source-fetch items are different boundaries. [Research ledger](evidence/cli-sweep407/researched-source-and-dedupe.json).

Regression gates must fail on current missing-tool behavior and pass real generated-image Markdown-to-HTML conversion at build and runtime, network disabled for runtime, with required executable/data/library support, meaningful failure inputs, primary/companion/static/exclusion neighbors and both architectures. After publication replay both original fresh first native creates without an installer prefix, exact source/document/program/runtime/HTML/log proofs and cleanup. Existing install controls alone do not meet DoD. Full suites/other bases/live static/pre-deploy and candidate binaries are currently unverified. No product code changed; estimate200m across six tasks.
