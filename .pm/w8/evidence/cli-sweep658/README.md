# CLI sweep 658 — Key Value create validation and lifecycle (live, 2026-10-19)

- `--plan bogus` → server 400 `invalid request body at /plan`; `--memory-policy bogus` and `--persistence-mode bogus` → client enum refusals listing the accepted values.
- Valid free create `qa658-kv` (`red-dbbbec5fsg2s73ako7o0`, region `fsn1`, version 8): `creating` → `available` within ~40s.
- Data plane not exercised this sweep: the local `redis-cli` binary is absent, so no PING/SET/GET was run (earlier sweeps covered the KV data path). Control-plane lifecycle only.
- Deleted via CLI; `keyvalues list` verified zero `qa658` residue.

No bug filed.
