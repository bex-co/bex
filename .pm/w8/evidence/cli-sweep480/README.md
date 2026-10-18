# Branch switch as saved deploy input — sweep480

Passing functional coverage, no new product issue; ship state unchanged (gates re-checked this sweep: `main` still red at `e6457568b`, same failed run 39120931451, no newer commit — held commit `f62d6740e` remains local, nothing pushed). Same isolated session as sweeps 454–479, `bex-canary`, HEAD checkout build, 2026-10-18.

Branch as a saved deploy input: native service created on `--branch main` goes `live` at commit `039c3477…`; `services update --branch ag/test-branch` persists (`branch: ag/test-branch`); the next deploy **resolved that branch's distinct commit** `a78686b1…` ("Test branch commit") — the switch took effect at the source level. That deploy then `build_failed`; the failure reason was not captured before cleanup (the build-log text filter returned no line and the fixture was deleted in the same step), so the cause — most plausibly the test branch's own content — is recorded here as explicitly unverified rather than guessed. The prior release kept serving HTTP 200 throughout.

Cleanup: exact-owned `srv-dbai9lhn2qrc73fmlog0` deleted by ID (404-verified); baseline deltas are concurrent foreign churn. No plan mutations. Session remains active; next inline sweep is 481; `f62d6740e` + evidence 477–480 await a green main.
