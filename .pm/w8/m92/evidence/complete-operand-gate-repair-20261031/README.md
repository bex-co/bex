# Complete operand gate and projection repair — 2026-10-31

The revised publisher retains full Timescale/IMMV data through coherent logical export, incremental restore checks, actual Free resource configuration and the pinned CNPG1.30.1 rollout requirement. Exact PostgreSQL server/client/libpq versions are asserted unchanged. DuckDB unit/restart-only/superuser-only setting observations follow its pinned contract.

Reconciliation preserves an unknown-major existing image and its declared older operand profile without adopting webhook defaults; observed-major selection and logical-export image/command agree. The new old-image counterfactual fails when its preservation is removed. An initially broad preservation failed the full suite (legacy pg_stat_statements and convergence); the final narrow correction retains that history and passes all244 envtest specs. Final full make test passed235.314s and all-module lint/reachability passed200.247s.

The21 exact tested inputs are recorded. Unrelated workspace sums remain unstaged. No complete SQL/ordinary-owner restore, actual CNPG1.30.1 matrix, published two-architecture image or production adoption is claimed. The bounded PG13 build stopped before SQL, and its larger local retry has not started. These remain original qualification work.
