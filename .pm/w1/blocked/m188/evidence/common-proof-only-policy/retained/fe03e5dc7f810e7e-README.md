# Common Bookworm native foundation

The 2026-10-29 user decision selects one Bookworm system/tools image, with selected runtimes installed above it. This factory uses the immutable `buildpack-deps:bookworm` input recorded in [upstream.json](upstream.json). That file records anonymous upstream manifest identity, not runtime compatibility or approval of a published candidate.

Build context is `deploy/`. The recipe consumes the existing `native-base` installer, locked private Node tools and reviewed Python wheel manifest directly. Runtime libraries and headers formerly inherited from language images are installed once here. The complete offline Python wheel store is present even though the common image has no primary `/usr/local` runtime. Debian's Python remains a system utility; TypeScript and Webpack retain their private Node under `/opt`.

`native-common-image.yml` calls the existing publisher with exactly one `common-bookworm` row. Both architecture utility checks, full scans, SPDX reports and signing gates remain shared. Automatic per-language publication is retired; explicit singleton manual requests may finish existing work. Neither workflow sets the production common pin or adopts a per-language candidate.

Source checks, without image builds or network:

```sh
python3 -B -m unittest discover -s deploy/native-common -p '*_test.py'
python3 -B -m unittest discover -s deploy/native-base -p '*_test.py'
python3 -B -m unittest discover -s .github/scripts -p 'test_native_oci_*.py'
bash scripts/github-actions-validate.sh
```

The opt-in canonical `TestNativeUtilityGeneratedImages` gate accepts `BEX_NATIVE_TEST_COMMON_IMAGE=<actual immutable candidate>` with `BEX_BUILDKIT_INTEGRATION=1`. It reuses one common candidate for Node 24, explicit Python 3.13 and no-signal Python 3.14.8 for a service created after the Python creation-date cutoff, alongside the existing missing-supplier negative and static-site case. `BEX_NATIVE_TEST_WHEELHOUSE` must name a separately prepared flat directory of compatible Python 3.13/3.14 Poetry and original Python fixture wheels, at most 128MiB. The Python cases add only an explicitly recorded test-prerequisite COPY/pip configuration layer; the production common image gains no fixture dependencies. Original programs/data/commands remain unchanged. `BEX_NATIVE_TEST_OUTPUT` retains both untouched canonical and executed recipes, exact prerequisite hashes, command logs, actual platform/config identity and output proofs. Run each architecture separately with its compatible wheels and corresponding Docker engine/platform; host-only execution does not establish both architectures.

The generator copies each selected runtime tree, including a reviewed default, restores its exact environment, refreshes the loader cache offline, installs matching Python tools offline and then adds companions. No tenant apt step is allowed. The separate default probe uses the existing primary packaging/C-extension and Brotli programs, plus genuine loader-by-SONAME/version/environment checks. Original fixture programs stay unchanged. These real image checks have not yet been executed for the common foundation. A common-image SBOM cannot substitute for the generated exact Python 3.14.8 image's SBOM or hosted acceptance.

Bookworm runtime donors cover the first slice. Legacy Python/Ruby closures also need their distinct compatibility libraries or same-release Bookworm builds; copying an old `/usr/local` tree alone is insufficient. Trixie Elixir images require independently selected exact Bookworm OTP artifacts plus the selected Elixir tree and rebar tools. A missing prebuilt compatible donor must use a reviewed same-version build, without downgrading the selected runtime or adding a new unsupported-version result. These paths require real closure/extension/companion proofs before a common pin is populated. All 28 frozen programs, data and commands remain unchanged.
