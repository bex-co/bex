# m87 runtime-detection evidence

Verified on 2026-09-02 against the local dev-2 stack, a real GitHub App
installation, and repositories visible to the connected `bex-co` and `puncsky`
accounts. No external repository was created or modified for this check.

## Real-repository probe matrix

| Expected outcome | Repository / root | Observed manifest | Result |
| --- | --- | --- | --- |
| Docker | `bex-co/bex` / `examples/hello-go` | `Dockerfile` (also `go.mod`) | `docker`; Docker precedence confirmed |
| Go | `bex-co/bex` / `lego/backend` | `go.mod` | `go` |
| Node | `bex-co/bex` / `mobile` | `package.json` | `node` |
| Python | `puncsky/stargately-ledger` / repository root | `requirements.txt` | `python` |
| Ruby | `puncsky/engineering-blogs` / repository root | `Gemfile` | `ruby` |
| Elixir | `bex-co/BlockScout` / repository root | `mix.exs` | `elixir` |
| Rust | `bex-co/bex` / `examples/hello-rust` | `Cargo.toml` | `rust` |
| Conflicting native manifests | `bex-co/discourse` / repository root | `Gemfile` and `package.json` | unknown (`runtime: null`) |
| Unrecognized root | `bex-co/bex` / repository root | no supported top-level manifest | unknown (`runtime: null`) |

The direct GraphQL checks returned no errors. Expected GitHub failures are represented
as nullable detection fields rather than GraphQL errors.

## Wizard verification

The New Web Service wizard was driven in Chromium against the dev-2 dashboard and
API:

- Selecting `bex-co/BlockScout` pre-selected Elixir and filled the Elixir build and
  start commands.
- Selecting `bex-co/bex`, entering `lego/backend`, and then changing Root Directory
  to `mobile` re-inferred Go and then Node.
- After explicitly selecting Ruby, changing Root Directory to a Go or Node subtree
  left Ruby selected.
- Automated form coverage confirms that an explicit build command or start command
  is likewise preserved when a later Root Directory result arrives.
- Entering a nonexistent Root Directory produced an expected GitHub 404 behind the
  query, left the current runtime unchanged, showed no alert, and emitted no browser
  console error.

Local screenshots (gitignored test artifacts):

- `../../../.playwright-mcp/m87-runtime-repo-pick.png`
- `../../../.playwright-mcp/m87-wizard-root-reinfer.png`
- `../../../.playwright-mcp/m87-runtime-manual-choice.png`
- `../../../.playwright-mcp/m87-runtime-ruby.png`
- `../../../.playwright-mcp/m87-runtime-rust.png`

## Render comparison and surface stance

The current public Render dashboard bundle establishes the client lifecycle:
repository selection applies its primary runtime/build/start suggestion; Root
Directory blur requests a fresh private-GraphQL suggestion; and touched runtime,
build, start, or static-publish fields protect an explicit edit from a later result.
The capture method, asset hashes, and detailed comparison are recorded in
[the runtime-inference artifact](../../../docs/render-artifacts/runtime-inference.md).
Render's public [language support](https://render.com/docs/language-support),
[Docker](https://render.com/docs/docker), and
[monorepo support](https://render.com/docs/monorepo-support) documentation establish
the supported native runtimes and Root Directory model, but neither those docs nor
the client bundle expose the server-side multi-manifest ranking algorithm.

The conservative implemented rule is therefore: `Dockerfile` wins; one unique native
runtime wins; multiple distinct native-runtime signals produce unknown rather than a
guess. `requirements.txt` and `pyproject.toml` are two signals for the same Python
runtime and are not considered ambiguous.

The detection operation remains dashboard-only GraphQL. Render's equivalent is a
private dashboard GraphQL operation, with no public REST or MCP runtime-inference
contract. Adding REST/MCP adapters would widen the public surface without improving
parity.

## Automated gates

The `/simplify` pass ran with separate reuse, quality, and efficiency reviews.
Accepted cleanups reuse the shared optional-string GraphQL field helper, make
Docker precedence explicit, narrow the dashboard hook's public result, and cache
the bounded runtime verdict instead of full directory listings. Identical cold
requests are coalesced; unknown results have a five-second TTL; the complete
best-effort GitHub probe has a five-second deadline.

Final results after those cleanups:

- `cd lego/backend && go test ./...` — pass.
- `cd lego/operator && make lint-backend` — pass (`0 issues`).
- `cd dashboard && yarn lint` — pass (includes typecheck and unused-code checks).
- `cd dashboard && yarn test` — pass (393 test files, 2,893 tests).
