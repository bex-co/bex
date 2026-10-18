# Registry-credential refusal + host log filter + env parsing — sweep484

Passing functional coverage, no new product issue; ship state unchanged (gates re-run: `main` still red at `e6457568b`, run 39120931451; held commit `f62d6740e` remains local). Same isolated session as sweeps 454–483, `bex-canary`, HEAD checkout build, 2026-10-18.

An unknown `--registry-credential` refuses before any resource exists ("no registry credential found with ID or name … (exact match required)"). Env-var parsing is exact at the edges: `--env-var 'QA484EQ=a=b=c'` stores value `a=b=c` (split on first `=` only) and `--env-var 'QA484EMPTY='` stores the empty string, both confirmed on the env-vars wire. Request-log `--host` filtering matches the planted request under the service hostname, misses honestly for an unknown host, and host discovery offers exactly the produced hostname.

Cleanup: exact-owned `srv-dbaii0rc1vqc73a22jt0` deleted by ID (404-verified); baseline deltas are concurrent foreign churn, observed read-only. No plan mutations. Session remains active; next inline sweep is 485; `f62d6740e` + evidence 477–484 await a green main.
