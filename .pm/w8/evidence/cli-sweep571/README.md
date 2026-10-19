# Sweep 571 — blueprint generate

Production `bex-canary`; owned fixture `qa571-gen` (`srv-dbb70avo17is73furspg`), deleted after probes (REST 404; platform URL 404).

## Findings

- Export for the fixture preserved image URL, free plan, health-check path, shutdown delay, IP allowlist, and env-var name; the env value was not exported (`sync: false`).
- `bex blueprints validate` accepted both the exported service manifest and a separately generated baseline-Postgres manifest (`valid: true`).
- Empty/resource-less generate requests returned named 400; synthetic, cross-kind, and mixed selections returned uniform 404; foreign workspace returned 403.
- Duplicate selection reached the generator self-check and returned coded 503, consistent with the existing w4/193 design.
- Malformed, empty, string, array, and wrong-type JSON bodies on bex-native `POST /blueprints/generate` returned bare 500. Preview returned 400; Render-gated validate returned boundary 400. The sole raw `DecodeJSON` return is `rest.go:1381`. Filed as [w8/133](../../133.md).

No secrets or env values are reproduced; scratch manifests remain outside the repo under `/tmp/qa501`.
