# Common Bookworm native foundation

The 2026-10-29 user decision selects one Bookworm system/tools image, with selected runtimes installed above it. This factory uses the immutable `buildpack-deps:bookworm` input recorded in [upstream.json](upstream.json). That file records anonymous upstream manifest identity, not runtime compatibility or approval of a published candidate.

Build context is `deploy/`. The recipe consumes the existing `native-base` installer, locked private Node tools and reviewed Python wheel manifest directly. Runtime libraries and headers formerly inherited from language images are installed once here. The complete offline Python wheel store is present even though the common image has no primary `/usr/local` runtime. Debian's Python remains a system utility; TypeScript and Webpack retain their private Node under `/opt`.

`native-common-image.yml` calls the existing publisher with exactly one `common-bookworm` row. D3 (2026-11-01) limits production publication, full utility/family checks, scans, SPDX reports and signatures to linux/amd64. The separate common-only ARM64 local variant receives startup/core-tool checks and is unsigned; it cannot fail production admission or serve as a production candidate. Automatic per-language publication is retired; explicit singleton manual requests may finish existing work. Neither workflow sets the production common pin or adopts a per-language candidate.

Source checks, without image builds or network:

```sh
python3 -B -m unittest discover -s deploy/native-common -p '*_test.py'
python3 -B -m unittest discover -s deploy/native-base -p '*_test.py'
python3 -B -m unittest discover -s .github/scripts -p 'test_native_oci_*.py'
bash scripts/github-actions-validate.sh
```

The opt-in canonical `TestNativeUtilityGeneratedImages` gate accepts `BEX_NATIVE_TEST_COMMON_IMAGE=<actual immutable candidate>` with `BEX_BUILDKIT_INTEGRATION=1`. It reuses one common candidate for Node 24, explicit Python 3.13 and no-signal Python 3.14.8 for a service created after the Python creation-date cutoff, alongside the existing missing-supplier negative and static-site case. `BEX_NATIVE_TEST_WHEELHOUSE` must name a separately prepared flat directory of compatible Python 3.13/3.14 Poetry and original Python fixture wheels, at most 128MiB. The Python cases add only an explicitly recorded test-prerequisite COPY/pip configuration layer; the production common image gains no fixture dependencies. Original programs/data/commands remain unchanged. `BEX_NATIVE_TEST_OUTPUT` retains both untouched canonical and executed recipes, exact prerequisite hashes, command logs, actual platform/config identity and output proofs. Run production qualification on linux/amd64 with compatible wheels and its actual target platform. The common-only ARM64 smoke is a local-development check and establishes no per-runtime ARM64 qualification.

The generator copies each selected runtime tree, including a reviewed default, restores its exact environment, refreshes the loader cache offline, installs matching Python tools offline and then adds companions. No tenant apt step is allowed. The separate default probe uses the existing primary packaging/C-extension and Brotli programs, plus genuine loader-by-SONAME/version/environment checks. Original fixture programs stay unchanged. Earlier dated utility runs, including both 0b7 architectures on the 0941 image, are history in w8/m91, not an input to any later run. Affected runtime families and hosted acceptance remain open. A common-image SBOM cannot substitute for the generated exact Python 3.14.8 image's SBOM or hosted acceptance.

Bookworm runtime donors cover the first slice. Legacy Python/Ruby closures also need their distinct compatibility libraries or same-release Bookworm builds; copying an old `/usr/local` tree alone is insufficient. Trixie Elixir images require independently selected exact Bookworm OTP artifacts plus the selected Elixir tree and rebar tools. A missing prebuilt compatible donor must use a reviewed same-version build, without downgrading the selected runtime or adding a new unsupported-version result. These paths require real closure/extension/companion proofs before a common pin is populated. All 28 frozen programs, data and commands remain unchanged.

The common generated-image gate includes five independent omission subtests. Each test-only layer checks healthy prerequisites, removes exactly one of jq, rsync, Python metadata tooling, pip or Python headers, rechecks the independent prerequisites and runs the unchanged original m98/m103 command. A fresh stdout probe and the later expected failure are required; the canonical and executed recipes and omission receipt are retained. Source-only tests do not establish that these image subtests executed.

## Fresh-only qualification

Qualification never reuses an earlier image or replays retained run output. Each `native-common-image.yml` run publishes one image through `native-base-image.yml` and qualifies that image. `plan` validates the requested profile and case. `qualify` runs the complete original AMD64 utility cohort. A manual dispatch that names a family `profile` and `case` then runs that one case in `qualify-profile`, and only after `qualify` passed on the same image. A failed publication or utility run has no earlier image to fall back to.

`gh workflow run native-common-image.yml -f profile=families -f case=all` runs every family case as one `qualify-profile` matrix, up to eight hosted VMs at a time, on the one image that run published. That run is how a digest is shown to pass every case before adoption. The Node 14/16/18/20 lines run through `node-donors`, the production route that copies the published runtime trees. The `node` profile's source-fallback routes for those lines stay available as single cases, and so does the node-addon `historical-*` route, which builds on the official images without the common base. A failed case is fixed in the source, and the whole families run is dispatched again on a new image.

The dispatch mode that reused the reviewed 0941 image, its checked-in receipt and the GHCR evidence package it read were retired on 2026-11-16 (w6/m422). Production pins stay empty until separate adoption review.

The local-only discovery tag is `native-common-bookworm-arm64-local`. Its candidate tag may exist before the smoke finishes; the discovery tag moves only on smoke success. Its immutable reference is recorded as local-only, never substituted into the production output. A failed/cancelled smoke is unproved and does not fail the AMD64 production jobs. D3 requires no ARM64 family work.
