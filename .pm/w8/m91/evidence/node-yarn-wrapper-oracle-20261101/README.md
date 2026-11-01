# Node Yarn wrapper oracle correction — 2026-11-01

The actual local ARM64 donor Node14 case reaches a valid Yarn installation, then fails because the qualification oracle expects `bin/yarn.js`. Canonical `nativeYarnInstall` links `/usr/local/bin/yarn` to the reviewed archive shell wrapper `/opt/yarn-v1.22.22/bin/yarn`. The actual tools-build proof reports that wrapper and the correct package/version. The captured failure is preserved in full; it is not a successful family qualification.

The correction requires exact canonical wrapper equality. A regression accepts the actual identity and rejects the JavaScript file, another root, another version and an unresolved symlink. All other driver assertions, nine positive manifests and original13/28 programs/data remain unchanged. Ten Python controls and the complete Go build-package suite pass. The existing Go output-controls test runs the Python suite in CI.

The failed attempt also exposed missing host buildx plugin discovery in its isolated Docker config. A separately reviewed private controller correction supplies only the verified plugin directory. The fallback cleanup removed every owned runner, builder container, state volume and labeled image/container; fresh exact absence checks pass. Original failed RESULT bytes remain unchanged.

This source correction requires a fresh complete same-image utility cohort on both CI architectures before the strict family workflow can treat the amended qualification source as current. The retained source1feba baseline remains historical evidence. Image0941 approval is unchanged; production pins, full family/phase qualification, adoption and original hosted acceptance remain open. A later local diagnostic run must bind its actual amended source and binary explicitly.
