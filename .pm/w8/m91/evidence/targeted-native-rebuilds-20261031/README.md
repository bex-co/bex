# Three fixed native rebuild requests — 2026-10-31

Root independently checked all71 frozen plan files and133 execution files, every actual singleton request/direct HTTP200 run identity, and all30 native Git input blobs/modes against fixed2cc. Each request was submitted once; no mutation retry, full matrix, rerun or cancellation occurred. Complete public metadata and prior stopped preflight are retained losslessly.

| Runtime | Source | Run | Actual native job | Recorded state |
| --- | --- | --- | --- | --- |
| python-3.11-bookworm | `d95dcd885` | 41190561558 | 124506595632 | matrix passed; build queued |
| python-3.13-bookworm | `d95dcd885` | 41190693730 | 124506987325 | matrix passed; build queued |
| node-26-bookworm | `e48dd4a66` | 41190806242 | 124507306637 | matrix passed; build queued |

The exact full official bases and prior failure history are in the retained receipts. Queued jobs are not successful builds or supplier approvals. All original hosted/adoption acceptance remains open.
