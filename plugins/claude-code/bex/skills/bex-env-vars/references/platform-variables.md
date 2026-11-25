# Runtime-specific environment-variable notes

Read [environment-variables.md](environment-variables.md) and fetch the current default-variable reference before depending on a bex-injected variable's name, availability, build/runtime scope, or semantics. Do not use this file as a platform-variable catalog.

## Worker concurrency

Services created after December 8, 2025 use updated default `WEB_CONCURRENCY` behavior compared with older services. When debugging unexpected worker counts, compare the service creation date, explicit `WEB_CONCURRENCY` overrides, and the currently documented concurrency and CPU variables.

Treat platform-provided concurrency values as starting points. Tune worker counts with both CPU and memory constraints in mind, and validate them under representative load.

## Native runtimes and Docker

Language-version environment variables primarily apply to bex native build pipelines. Docker-based services should encode toolchain versions in the image or its build configuration. Confirm the supported version-selection mechanism in the current runtime documentation before changing it.

## Application parsing

Environment-variable values arrive as strings. Parse integers with explicit error handling, and handle values such as `"false"` and the empty string deliberately in boolean logic.
