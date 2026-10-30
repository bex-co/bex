# Native Node builds lack Python packaging and development prerequisites

Why: a normal Node service build cannot generate Python package metadata, install a local helper wheel, or compile the Python C extension used by its HTTP server, although those native tools are documented as supplied.

**Severity:** three major functional omissions, independently isolated. **Ownership:** operator native image assembly. Ordinary CLI sweep424, 2026-10-17. Published Bex 0.3.2/a6350da0 and unchanged Render 2.27.0/a764810a7682, isolated human-QA bex-canary/tea-daif693dqjvc73e7as3g, noninteractive JSON, api.bex.co/v1. Source 522dfe3cdd6787b8635b992a5d863822ce6e2e22; serving SHA/OCI unobserved. [Complete evidence](evidence/cli-sweep424/README.md).

## Three failures in one real four-stage build

Both clients use supported `services create --type web_service --plan free --runtime node --env-var NODE_VERSION=24 --auto-deploy=false --confirm -o json`, fresh names and git-fixtures/basic/master. A bounded own source pack reconstructs the app, setup.py, C source and local wheel, validating their exact composite identity. The identical build pipeline executes:

```sh
python3 setup.py egg_info
python3 qapy.py metadata
python3 -m pip install --no-index --no-deps --target qsite qa_local_data-1.0-py3-none-any.whl
python3 qapy.py wheel
python3 setup.py build_ext --inplace
python3 qapy.py build
```

Start is `node qsrv.cjs serve`. This is an explicitly authored Node/Python integration; the public repository is a source carrier.

1. **Missing setuptools:** both actual unchanged native creates POST 201/exit 0, but both original first builds fail `ModuleNotFoundError: No module named 'setuptools'`. Python 3.11 exists; setuptools/pip/header probes are false. No distribution metadata, wheel-install proof or extension output is produced; URLs return 404.
2. **Missing pip:** each client adds only a `python3-setuptools` diagnostic install line. Both native updates exit 0 and both second builds generate the exact real distribution metadata, then fail `/usr/bin/python3: No module named pip`. Wheel-install and extension proofs remain absent; URLs return 404.
3. **Missing development headers:** each client adds only `python3-pip` to that same line. Both third builds generate metadata, install/import the exact own wheel successfully, then real C compilation fails `fatal error: Python.h: No such file or directory`. Development-header probes remain false and there is no extension output; URLs return 404.
4. Add only `python3-dev` to the same line. Both fourth children become Live. Source/wheel members/build steps/start/data/Free/env remain unchanged throughout. Each of the six native updates exits 0.

The own wheel is installed from the local filesystem with no PyPI/dependency download and no interpreter-global package write. Actual setuptools invokes gcc and emits an importable extension. Six final HTTP 200 responses run both the installed pure-Python helper and real C functions through the Node server, returning independently expected weighted sums 13/13/0 and exact Unicode text. Source/composite identity, actual distribution metadata, installed module hash and matching build/runtime extension byte/hash identity are checked. Six native app events and twenty-four REST/GraphQL/MCP plus eight explicit-env readbacks agree. Both exact parents/all eight deploy children/URLs are deleted, baseline preserved and current quotas reconciled. Full native commands/results, redacted unchanged create wire and all data/proofs/cleanup are durable.

[Render's native tools](https://render.com/docs/native-runtimes) explicitly include python3-setuptools, python3-pip and python3-dev during builds and runtime. [Pip's reference](https://pip.pypa.io/en/stable/cli/pip_install/) documents local-wheel/target/no-index/no-deps installation. [Setuptools' extension guide](https://setuptools.pypa.io/en/latest/userguide/ext_modules.html) describes real C compilation against Python headers. Exact Render versions and a Render-hosted comparative run are unverified. The diagnostic Debian packages are not adopted platform supplier/version pins.

## Producer, consumer and repair

Pinned `pkg/service/create.go:272–310` and `pkg/service/update.go:361–377` serialize the supported commands. Backend `lego/backend/internal/apps/rest.go:455–505,1039–1075` decodes them; `apps/service.go:2935–2950` persists them; operator `internal/controller/app_controller.go:1048–1064` passes them into build.Options. Exact shared readbacks and diagnostic supplier controls show those layers work. Preserve their request/schema behavior.

`lego/operator/internal/build/native.go:283–320` assembles the selected language base and existing Node/media/default helpers without the missing Python suppliers. The immutable selected Node 24 base has Python 3.11 but none of the three prerequisites. `nativepnpm.go:33–62` and `nativeresolve.go:131–178` complete Node/JavaScript companions; neither supplies the missing Python packaging/development payload. `native.go:404–407` exports the sole recipe, `build.go:1176–1183,936–960` passes recipe/native inputs into BuildKit, and `nativeresolve.go:53–120` selects bases/companions. Actual source exports and selected-base real pipeline controls confirm the mechanism.

Supply reviewed immutable amd64/arm64 Python packaging/development tools and complete compatible library/module/data/header/config closure before authored native builds, retained for applicable runtime/pre-deploy/static use. Give each omission its own implementation and regression obligation:

- Setuptools must import and generate actual distribution metadata, with required packaging/distutils/pkg-resources dependencies where the chosen version needs them.
- Pip must execute noninteractively under the intended Python, install a valid own wheel into an explicit target and preserve the installed module bytes/import behavior.
- Development headers and sysconfig/compiler/linker inputs must match that Python's ABI and allow real C-extension compilation/import. A placeholder header or unrelated Python version does not satisfy this.

Review exact version/content/dependency/platform/distro/ABI/freshness and cache identities across bookworm/trixie and all intended bases. Preserve the base/primary Python interpreter, working primary Python toolchains, explicit PATH/PYTHONPATH/env and project-local tools. Coherent shared delivery may serve the three payloads, with deliberate per-base compatibility; copying an unrelated donor Python root filesystem or silently replacing a working primary interpreter is outside this repair. Preserve Node/Yarn/pnpm/Bun/media/native-init/source-root/files/cache/app libraries and the existing Dockerfile/prebuilt-image/CNB paths. Retain existing API/CLI fields and runtime selectors.

One generator/preparer/resolver/BuildJob chain serves six primary runtimes/16 reviewed image lines and native static builds. Audit actual missing/present prerequisites by base, language companion and phase; do not infer every other base has all three omissions from Node 24. Preserve REST/GraphQL/MCP/Blueprint/dashboard runtime/commands/plan/env semantics and document supplier adoption/limits in ADR060.

## Dedupe, regression and limits

Broad board matches concern Poetry/m72, Node-inside-Python/m77 and Python HTTPS/116. Those do not supply missing Python tooling inside Node. Refined package/literal-header searches across open/blocked/done and targeted source histories have no matching repair. SWIG/m100 supplies a binding generator, not these Python packaging modules or matching development headers. Each staged failure proves a separate prerequisite; none is credited solely to the combined final install.

Real generated-image regressions must independently fail before/pass after at metadata, local-only wheel installation and extension compilation/import, then run the complete Node/Python integration with independent Unicode/arithmetic/module/source/extension identities. Prove actual metadata, exact installed bytes, meaningful package/compiler failures and network-disabled artifact use. Audit intended bases/architectures/companions/static/pre-deploy/env/PATH/PYTHONPATH/cache and excluded neighbors while preserving already-working primary Python behavior. After publication both unchanged clients' original first Free Node services must become Live without an install line and pass the complete HTTP/log/shared-readback/exact cleanup checks. Code or diagnostic combined controls alone do not meet DoD.

Local selected-base four-stage results are 1/1/1/0 plus three genuine own-container-loopback HTTP requests. Full generated native images are not locally executed. Only Node 24/Free web/system Python 3.11/the named own wheel and C extension are hosted. Other bases/architectures/languages/static/pre-deploy/browser, PyPI/build isolation/venvs, broader extension APIs, full generated local images, physical teardown and serving revision remain unverified. Product implementation unchanged; this schedules three repairs.
