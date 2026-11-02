# Verified immutable image metadata reuse — 2026-11-01

Repeated official runtime metadata reads contributed to the actual Docker Hub429 failures. A bounded resolver-local cache now reuses only successful, fully verified pinned metadata, keyed by registry, repository and digest. Independent returned ENV/history copies protect callers; failures and cancelled calls are never cached. Network reads occur outside the mutex, and different registries/digests remain separate. The unchanged twelve-case utility cohort now shares one real resolver.

The real request-count regression fails on the old resolver (six reads) and passes with the cache (three reads). Caller mutation, new subjects/endpoints, empty or corrupt data, errors, cancellation, the64-entry bound and concurrent reads are covered. Full Go build-package, race, scoped lint and independent checks pass. The registry verifier, retry policy, all original programs/cases/oracles and image pins remain unchanged.

This reduces duplicate traffic; unique subjects and BuildKit still require upstream access. The failed cohorts stay failed, and a fresh complete current-source baseline is required. No new runtime, Job, hosted or adoption acceptance is claimed.
