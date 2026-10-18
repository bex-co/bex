# Digest-pinned image service — sweep483

Passing functional coverage, no new product issue; ship state unchanged (gates re-run again: `main` still red at `e6457568b`, same failed run; held commit `f62d6740e` remains local, nothing pushed). Same isolated session as sweeps 454–482, `bex-canary`, HEAD checkout build, 2026-10-18.

A web service created by digest (`gcr.io/google-samples/hello-app@sha256:9b9870…`, resolved read-only via `skopeo inspect`) stores the digest reference verbatim, deploys `live`, and serves the pinned content (`Version: 2.0.0`). A same-repository tag override deploy is accepted against the digest-pinned service (tag-or-digest rule), while malformed digests are named refusals with no state change — empty digest at create: `400 image digest is invalid: invalid checksum digest format`; short digest at update: `400 … invalid checksum digest length`.

Cleanup: exact-owned `srv-dbaigkrc1vqc73a22j40` deleted by ID (404-verified); baseline deltas are concurrent foreign churn, observed read-only. No plan mutations. Session remains active; next inline sweep is 484; `f62d6740e` + evidence 477–483 await a green main.
