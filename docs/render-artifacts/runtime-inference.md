# Capture — Render New Service runtime inference (w2/m87)

**Captured:** 2026-09-02 · **Method:** inspection of the public production dashboard JavaScript linked by `dashboard.render.com`, cross-checked against Render's public language, Docker, monorepo, and deploy documentation. An authenticated Render account was not required to inspect this client control flow.

## Captured production assets

The login shell referenced these content-addressed assets at capture time:

| Asset | SHA-256 |
| --- | --- |
| [`index-CQyw6QAr.js`](https://dashboard.render.com/assets/index-CQyw6QAr.js) | `bb6608520f0c06daa9994a4c829c5ac1f67f56228ecd0908312b1f389fb165bd` |
| [`SignInOutcome-CCgvEdPU.js`](https://dashboard.render.com/assets/SignInOutcome-CCgvEdPU.js) | `b495f771439e4b92a90a76e5589ad86c52a356d0243241a5de9abef11e21dfd8` |

The observations below paraphrase client behavior; no production source is vendored in this repository.

## Observed behavior

- On repository selection, Render initializes the environment, build command, and start command from the repository's `primarySuggestion`.
- Root Directory is normalized on blur. Render then makes a no-cache request to its private dashboard GraphQL `rootDirSuggestion` operation with repository, provider, owner, branch, and root-directory inputs.
- Before applying a root-directory suggestion, the form checks whether the runtime/environment, build command, start command, or static publish path has been touched. A later suggestion does not replace an explicit edit. A shared update helper additionally guards each suggested command with its own touched state.
- The client receives one `primarySuggestion`; it does not contain the server-side manifest-ranking algorithm. Render's public [language-support](https://render.com/docs/language-support), [Docker](https://render.com/docs/docker), and [monorepo](https://render.com/docs/monorepo-support) documentation establishes supported runtime and Root Directory behavior but does not specify how conflicting manifests are ranked.

## bex parity decision

bex follows the same semantic lifecycle: repository selection produces a suggestion, changing Root Directory re-runs inference, and an explicit runtime, build-command, start-command, or Dockerfile-path edit prevents a later result from rewriting the coupled build fields. bex requests after a 400 ms Root Directory debounce instead of Render's blur trigger, so the result appears while typing rather than after focus leaves the field.

Because Render's multi-manifest ranking remains server-side and unpublished, bex uses a deterministic conservative rule: `Dockerfile` wins; exactly one native runtime wins; conflicting native runtimes return no suggestion. Python's `requirements.txt` and `pyproject.toml` count as two signals for the same runtime, not a conflict. Returning no suggestion preserves manual selection and does not block creation.

The operation remains dashboard-only GraphQL. Render's corresponding operation is private dashboard GraphQL, and its public REST API and MCP server do not publish a runtime-inference contract. Adding public REST or MCP adapters would invent a surface rather than close a Render parity gap.
