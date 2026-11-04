# Required AMD64 family cases on the run-100 baseline — 2026-11-04

Each row is one `workflow_dispatch` of `native-common-image.yml` (`source=reviewed-candidate`, one profile/case) admitted against the candidate bound in `e07c7176d` ([run 100 baseline](../common-d3-run100-20261104/README.md)). Every directory retains the run's jobs API response, artifact list and every artifact member byte for byte, for failures as well as passes. Failed runs are never relabeled; their cases are re-dispatched as new runs.

| run | case | source | family job | plan | failed step | members |
| --- | --- | --- | --- | --- | --- | --- |
| [41821745043](https://github.com/bex-co/bex/actions/runs/41821745043) | [native-init-all-launches](41821745043-native-init-all-launches/) | `e07c7176d` | success | success | — | 79 |
| [41821756135](https://github.com/bex-co/bex/actions/runs/41821756135) | [go-rust-go](41821756135-go-rust-go/) | `e07c7176d` | success | success | — | 19 |
| [41821767314](https://github.com/bex-co/bex/actions/runs/41821767314) | [go-rust-rust](41821767314-go-rust-rust/) | `e07c7176d` | failure | success | measure the selected family case without claiming full-family acceptance | 10 |
| [41821780219](https://github.com/bex-co/bex/actions/runs/41821780219) | [legacy-python-3.8](41821780219-legacy-python-3.8/) | `e07c7176d` | failure | success | measure the selected family case without claiming full-family acceptance | 68 |
| [41822441488](https://github.com/bex-co/bex/actions/runs/41822441488) | [ruby-ruby34](41822441488-ruby-ruby34/) | `e07c7176d` | failure | success | measure the selected family case without claiming full-family acceptance | 13 |
| [41822453767](https://github.com/bex-co/bex/actions/runs/41822453767) | [ruby-ruby40](41822453767-ruby-ruby40/) | `e07c7176d` | failure | success | measure the selected family case without claiming full-family acceptance | 13 |
| [41822464774](https://github.com/bex-co/bex/actions/runs/41822464774) | [ruby-ruby344-creation-default](41822464774-ruby-ruby344-creation-default/) | `e07c7176d` | failure | success | measure the selected family case without claiming full-family acceptance | 13 |
| [41822476519](https://github.com/bex-co/bex/actions/runs/41822476519) | [ruby-ruby30](41822476519-ruby-ruby30/) | `e07c7176d` | failure | success | measure the selected family case without claiming full-family acceptance; Run actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 | 0 |
| [41823275421](https://github.com/bex-co/bex/actions/runs/41823275421) | [ruby-ruby31](41823275421-ruby-ruby31/) | `e07c7176d` | failure | success | pre-pull the pinned BuildKit image with bounded retries | 9 |
| [41823291985](https://github.com/bex-co/bex/actions/runs/41823291985) | [ruby-ruby32](41823291985-ruby-ruby32/) | `e07c7176d` | failure | success | materialize only complete candidate evidence packets | 0 |
| [41823312833](https://github.com/bex-co/bex/actions/runs/41823312833) | [ruby-ruby33](41823312833-ruby-ruby33/) | `e07c7176d` | skipped | failure | materialize only complete candidate evidence packets | 0 |
| [41823325132](https://github.com/bex-co/bex/actions/runs/41823325132) | [ruby-node-static-ruby34](41823325132-ruby-node-static-ruby34/) | `e07c7176d` | failure | success | pre-pull the pinned BuildKit image with bounded retries | 9 |
| [41823414107](https://github.com/bex-co/bex/actions/runs/41823414107) | [go-rust-rust](41823414107-go-rust-rust/) | `e07c7176d` | failure | success | materialize only complete candidate evidence packets | 0 |
| [41833330036](https://github.com/bex-co/bex/actions/runs/41833330036) | [legacy-python-3.8](41833330036-legacy-python-3.8/) | `a9d569642` | failure | success | pre-pull the pinned BuildKit image with bounded retries | 9 |
| [41833491516](https://github.com/bex-co/bex/actions/runs/41833491516) | [ruby-ruby34](41833491516-ruby-ruby34/) | `a9d569642` | failure | success | pre-pull the pinned BuildKit image with bounded retries | 9 |
| [41838033301](https://github.com/bex-co/bex/actions/runs/41838033301) | [ruby-ruby40](41838033301-ruby-ruby40/) | `1ba9bae3d` | success | success | — | 17 |

**Infrastructure failures (2026-11-04, retained):** this session first dispatched 8–13 cases concurrently. All runners share one host (`DO_NOT_DO.md` #RUNNER-HOSTS), whose measured link was about 2 MB/s, and another session was also pulling a kind node image. Builds were killed at their deadline mid-download, evidence checkout hit TLS receive errors, uploads got ECONNRESET, and the shared 570-second BuildKit bootstrap budget ran out. One Rust attempt timed out on a Docker Hub metadata read. None of these is a product verdict. Dispatch is now strictly one case at a time, and every affected case is queued again.

This packet is family qualification evidence only. Production adoption (`nativeCommonBase`) and the original 28 hosted cases remain open.
