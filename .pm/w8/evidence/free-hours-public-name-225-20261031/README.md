# w8/225 — public service name through Resume(public ID)

The production fix shipped independently as6aaa7cd58 while this worker validated the same change. The merged regression strengthens that commit's existing test instead of adding a duplicate: it calls `Resume` using a minted public service ID, gives the App a distinct internal name and public service-name label, fixes the month clock, and requires the exact public-name message, unchanged error code/month parameters and unchanged suspension state.

The final merged focused tests and scoped lint pass. An actual original-production overlay fails specifically because its message names the internal `tea-example-customer-api` instead of `customer-api`. The earlier independent source, successful pre-merge checks and capacity-stopped attempt remain separate historical records; a stopped compile is not a failing test or a pass. All owned compiler processes and temporary files were removed. The final test source hash is131c733673d0b16fa4404ff888de7cd6b0dd91016add465cdc4442e5dc91bfe2.

The complete backend suite against CI-pinned real Postgres/OpenFGA/OpenBao/Loki is required by the pre-push ship gate. Its outcome will be recorded after actual execution. No production fixture or authentication was used for this regression.

Original bytes are retained losslessly as gzip, with source identities in `CUSTODY.json`. This evidence distinguishes the initial conservative resource gate, the explicitly amended local storage budget and the actual merged verification.


## Full backend verification — 2026-10-31

The full real-dependency suite passed with exit0/no resource stop, including the strengthened public-ID regression. It shipped as`d58e9663e`, alongside the three independent store-fixture clock corrections at`c680cd195`. [Full-suite and shipment records](../store-clock-fixtures-20261031/README.md#full-backend-gate-and-shipment--2026-10-31) preserve the final result and PM-only rebase equivalence.
