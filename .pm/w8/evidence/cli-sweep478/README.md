# Root-directory switch lifecycle — sweep478

Passing functional coverage, no new product issue; one process outcome first: the held ship (`f62d6740e`, w8/m116 + sweeps 465–476) still cannot push — `scripts/ship-gates.sh` again reports `main` red on the same upstream run ([39120931451](https://github.com/bex-co/bex/actions/runs/39120931451), `test-operator` envtest at `e6457568b`); no new `main` commit has landed since. The commit stays local; nothing was force-pushed.

Coverage: Docker service on `bex-co/bex` with `--root-directory examples/hello-go` builds to `live` (HTTP 200); `services update --root-directory examples` persists (`rootDir: examples`) and opens a config deploy; the next deploy fails named and fast (`build_failed`, CLI `--wait` exit 1) with `the Dockerfile "examples/Dockerfile" does not exist in the repository at "<sha>"` in build logs, while the prior release keeps serving HTTP 200 — root directory behaves as a saved build input with honest failure isolation.

Cleanup: exact-owned `srv-dbai5j3c1vqc73a22dtg` deleted by ID (404-verified); one new foreign row observed read-only. No plan mutations. Session remains active; next inline sweep is 479; `f62d6740e` + evidence 477–478 await a green main.
