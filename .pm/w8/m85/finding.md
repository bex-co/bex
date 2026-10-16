# Native Ruby ignores committed lockfile version pins

**Major · operator Ruby source selection.** Both native clients create Free Ruby fixtures with independently verified Discourse version files. The root lock pins Ruby3.4.7p58, but both first builds select4.0. Removing only the probe's version assertion gives healthy build/runtime snapshots at the wrong major line. [Complete native/version/source/cleanup evidence](evidence/cli-sweep354/README.md).

The first-build exit1 is caused by the harness's explicit expected-line assertion, after exact source-file checks. No Bundler dependency installation or Discourse startup failure is claimed. The reproducible product defect is a supported committed version pin being ignored. Correct Bex behavior maps this pin onto reviewed Ruby3.4; its intentional patch mapping remains intact.

## Contract and failing boundary

[Render's specialized Ruby version contract](https://render.com/docs/ruby-version) prioritizes the root lock Ruby-version block, then .ruby-version, then .tool-versions, then a Gemfile directive. The supplied Gemfile permits>=3.4 and<4.1, so using only its range selects4.0 even though the higher-priority lock requests3.4. Other Ruby source files are absent in the actual checkout. The unchanged pinned CLI transports Ruby/runtime/source/env/commands; six shared API configuration comparisons pass. No Ruby override or PATH repair is supplied.

`lego/operator/internal/build/nativeversion.go:80–81` currently reads only fromGemfile then .ruby-version. `fromGemfile:335–357` resolves the actual permissive requirement to the highest matching reviewed line4.0. It never reads Gemfile.lock/gems.locked or the Ruby entry in .tool-versions. `nativeresolve.go:50–70` feeds that choice into the generated base-image rewrite. native.go's reviewed Ruby table and nativeDockerfile then run build/start on the selected image. Both healthy diagnostic snapshots confirm4.0 with exact committed files.

The real resolver reproduces3 missing-input cases: the public root lock and equivalent gems.locked select4.0 rather than3.4, and an authored .tool-versions Ruby3.3.6 selects default3.4. Three literal/file/default controls pass. Four existing maintained selector/refusal/rewrite/JS tests still pass (29 named events). The additional signal types are source-only; root Gemfile.lock is the hosted failure.

## Concrete correction

Add structured Ruby lock-version extraction for Gemfile.lock/gems.locked, handling the normal ruby version plus patchlevel suffix, and Ruby .tool-versions parsing at the selected application root. Resolve the complete documented priority before the existing Gemfile fallback. A lock without a Ruby-version block can fall through; a recognized unsupported request must use the existing named refusal rather than silently changing lines. Define deterministic handling of simultaneous aliases and malformed inputs.

Reuse the reviewed-line/range/error helpers and existing regular-file/root protections. Preserve catalog versions, patch mapping, defaults when no signal exists, primary Node/Go/Python/Rust/Elixir, Ruby's JavaScript companion and optional Bun. `cmd/native-resolve/main.go` is the single resolve entrypoint and nativeBuildPreparer/nativeDockerfile supply the common consumer; no public field or CLI fork is needed. Native Ruby web/private/worker/cron and supported explicitly Ruby static input converge here; only Free web was live exercised.

Update ADR060's Ruby table and nativeversion_test.go:62, which currently assert Gemfile priority. The earlier m51 decision explicitly implemented the Gemfile/.ruby-version subset; it did not certify these missing inputs. [m51](../done/m51/README.md) retains its completed reviewed-line/old-input scope and [m77](../done/m77/README.md) its companion scope. This is newly exercised Ruby source-selection work, with a recorded priority update, not a patch-selection complaint or an undocumented change to the existing policy.

## Dedupe and acceptance

Open/blocked/done board, DO_NOT_DO, source/history and freshly fetched main contain no owner or implementation for these missing Ruby sources. Research/fetched779c825e2 product source is identical; serving revision remains unobserved. The earlier failed local Render relay attempt is conclusively unallocated and excluded; a fresh direct Render create succeeds. Both actual fixtures and all4 children are cleaned, with baseline/quota restored.

Six tasks total170m. Fresh same-class native fixtures must honor their independently verified committed pin on the first deployment without removing the assertion, changing source or adding a version override. Add maintained resolver and generated-image execution regressions for lock/alias/tool-version, priority, root/nested and unsupported/malformed neighbors, preserve the existing companions, document current contract versus intentional pin policy, and close only after deployed acceptance and cleanup. No product fix is implemented.
