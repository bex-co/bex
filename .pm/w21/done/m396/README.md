# w21 · m396 — Form checks and server rules share one corpus: the Go validator and the dashboard validator must agree on every input

**Worker:** worker21 **Goal:** one corpus per validator family drives both a Go test against the real server validator and a dashboard test against the client validator, so a form that refuses a valid input or accepts an invalid one fails CI instead of surfacing as a QA finding or an English toast. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Inventory the mirrored validators and choose the first families — **DONE** | 45m | — |
| [t002](done/t002.md) | Define the corpus format and assert it against the Go validators — **DONE** | 60m | t001 |
| [t003](done/t003.md) | Assert the dashboard's validators against the same corpus — **DONE** | 60m | t002 |
| [t004](done/t004.md) | Resolve the divergences the corpus finds — **DONE** | 90m | t003 |
| [t005](done/t005.md) | Register the corpus as a test input of both suites — **DONE** | 30m | t002, t003 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t004, t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t008 |

## Definition of done

1. At least eight families have a corpus under `lego/backend/testdata/validation/` (or the location t001 settles), with boundary inputs, and a Go test asserts the real server validator on every entry.
2. A dashboard test asserts each client validator against the same files: a deliberately stricter and a deliberately looser client both fail it (self-test); a client may be more lenient only on entries marked `serverOnly`.
3. Every divergence found is fixed, marked `serverOnly` with a reason, or filed as a follow-up note.
4. `python3 scripts/ci_test_inputs.py` passes, a corpus-only change is in the `paths` of both workflows, and `yarn lint`, `yarn test` and `cd lego/backend && go test ./...` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w21` (2026-11-14), filed by `/pmx them all to w21` on 2026-11-14, re-checked at filing against `origin/main` `cc671cbfd`, item 3. Thirteen w21 notes (015, 023, 033–036, 038, 043, 045, 047, 048, 050, 053) are one defect found field by field: 38 dashboard files name a backend path in a comment while 4 tests read Go source, and those compare vocabulary constants only. The repo's rule for two-sided duplication is a drift-guard test (`rt-unify-duplicates`).
- **Goal linkage:** Render parity first in the dashboard (ADR008), and the root `AGENTS.md` rule that the dashboard consumes the API and a hand-mirrored rule needs a guard.
- **Expected outcome:** Validation drift between the forms and bex-api fails a test; new forms add a corpus entry rather than a hand-copied regex.
- **Why now:** The defect is still being found (w4/642 is the latest), each costs a QA round, and the validators are pure functions on both sides, so one corpus is cheap now and costly to retrofit after more forms copy the rules.
- **Render parity task:** included (t006). The milestone edits dashboard form rules that mirror REST, GraphQL and MCP validators; the check compares each pinned rule against Render so the corpus does not harden a divergence.

## Open decisions

Recorded as the proposal's recommendation, not a user decision. A rule where bex's server is stricter than Render's, with no entry in `docs/ADR018-render-parity.md`, stands only with the user's sign-off. The recommendation, from ADR008 Render parity first, is to conform the server to Render in a follow-up and then have the corpus pin the conformed rule; until then the corpus pins the current behavior and t006 files the follow-up. The corpus never tightens a rule.

## Findings

### Pairing table (t001, 2026-11-14)

Corpus directory: `lego/backend/testdata/validation/`, one file per family, asserted by a Go test in the owning package (`internal/{core,store,registrycreds,apps}/validation_corpus_test.go`, through `internal/validationcorpus.Check`) and by `dashboard/src/test/__tests__/validation-corpus.test.ts`. Go cannot import `internal/` across modules, so each test stays in its package; the corpus is read by path.

| Family (entries, serverOnly) | Server verdict | Client validator |
| --- | --- | --- |
| `label` (27, 0) | `core.CheckLabel` on the text a form sends (`core/errors.go:388`) | `isValidLabel` (`common/lib/utils/resource-label.ts`) |
| `cidr` (32, 0) | `core.ValidateCIDRs` (`core/cidr.go:39`) | `isValidCIDR` (`common/lib/cidr.ts`) |
| `root-dir` (34, 0): Root Directory and Docker Build Context | `store.RootDirProblem(NormalizeRootDir(v))` (`store/api.go:691`) | `rootDirError` (`services/lib/build-settings-validation.ts`) |
| `build-path` (24, 0): Dockerfile Path | `store.RootDirProblem(v)` as typed | `relativePathError` (same file) |
| `env-value` (41, 0) | `core.CheckEnvValue` (`core/config.go:176`) | `envValueRuleError` (`services/lib/environment-draft.ts`) |
| `registry-host` (44, 0) | `canonicalRegistryHost` + `validateCredentialFields` (`registrycreds/host.go`, `service.go:414`) | `registryHostError` (`registry-credentials/lib/credential-fields.ts`) |
| `repo-url` (30, 8) | `store.ValidRepo` (`store/api.go:644`) | `isValidGitUrl` (`common/lib/utils/git-url.ts`) |
| `docker-image` (63, 5) | `store.ValidateImage` (`store/image.go:42`) | `isValidDockerImageRef` (`common/lib/utils/docker-image.ts`) |
| `custom-domain` (55, 4) | `claimableHostname` (`apps/domains.go:432`) | `domainNameError` (`services/lib/custom-domain-name.ts`) |

A form that trims what it sends is judged on the trimmed text: `validationcorpus.FormTrim` is JavaScript's `String.prototype.trim` (it strips U+FEFF and leaves U+0085, the opposite of Go's `TrimSpace`). `serverOnly` marks a refused input the client may still accept because only the server can judge it (userinfo and fragments in a clone URL, a registry policy, a public suffix, a 64-character label).

Already shared before this milestone, not redone: `config-key-vectors.json` (env key, secret file name), `resource-names.json`, `postgres-identifiers.json`, `read-replica-name-vectors.json`, `cron-schedule-vectors.json`, `lowercase-runs.json`.

Excluded, with the reason: IANA time zone (browser ICU against the host's tz database, neither deterministic); SSH public key (the TS pattern is not exported, Go's `ParseAuthorizedKey` goes well past a regex); email (a permissive shape check by design, the server stays authoritative); static routes and headers (Go takes structs, TS returns i18n keys); publish directory (only the control-character half pairs with `validatePublishPath`, the escape half is an operator-module internal); resource slug (a transform with a hash, not a verdict, needs its own output corpus).

### Divergences the corpus found (t004)

Six of nine families agreed on first contact. The other three disagreed on five inputs, all fixed in the client, none in the server:

| Family | Input | Server | Client was | Fix |
| --- | --- | --- | --- | --- |
| `label` | `"\nabc"`, `"abc\n"` | accepts: the form sends `value.trim()` | refused: the control-character check ran on the untrimmed text | `isValidLabel` trims first; the old test that pinned the untrimmed rule moved to "accepts" |
| `docker-image` | `nginx@sha256:<63 hex>`, `nginx@md5:<32 hex>` | refuses: `digest.Parse` needs a registered algorithm and its exact length | accepted: the grammar's `{32,}` hex | `DIGEST_HEX_LENGTH` (sha256 64, sha384 96, sha512 128) |
| `custom-domain` | `example.c` | accepts: only an all-digit top-level label is refused | refused: required two letters | the top-level label is any label that is not all digits |

Found along the way, not a verdict divergence: `IMAGE_REF_RE`'s name component (`-*` could match nothing) backtracked exponentially, so `a`×30 followed by an uppercase letter or `!` froze the tab for about half a minute on every keystroke of the image field. The separator is now `-+` (same language, checked on 2,015,539 strings up to length 8) and `docker-image.test.ts` bounds the time.

Mutation checks: dropping the zero-padded-prefix rule from `isValidCIDR` fails `validation-corpus.test.ts` with three named cases; replacing `netip.ParsePrefix` with `net.ParseCIDR` in `ValidateCIDRs` fails `TestCIDRCorpus` on `10.0.0.0/024`, `/08` and `/00`.

### Render comparison (t006, 2026-11-14)

Sources opened that day: the pinned OpenAPI `render-public-api-1.json` (scanned for every `maxLength`, `minLength`, `pattern` and `maxItems`: none on a service, project, environment, Postgres or Key Value name, `rootDir`, `dockerfilePath`, `dockerContext`, `repo`, `imagePath`, `cidrBlock`, an environment variable value, a custom domain `name` or a registry credential field) and the render.com pages for inbound IP rules, web services, custom domains, environment variables, monorepo support, Docker, deploying an image, projects, Postgres, Key Value and deploy to Render. The quotes below were read, not recalled.

| Family | Render's rule | bex against it | Recorded in ADR018 |
| --- | --- | --- | --- |
| `label` | none stated | unknown (100 code points, no control characters) | :93 "Bex policy, Render limit unverified" for a service name; no sign-off |
| `cidr` | "Only IPv4 CIDR ranges are supported." (inbound IP rules); datastore pages say "CIDR notation" and nothing on version or count | bex accepts IPv6; ADR018:199 records `::/0` saved as written on a service "as on Render and its Terraform provider", so the docs sentence may be stale | no divergence; filed as 075 |
| `root-dir`, `build-path` | none stated (`dockerfilePath` "Defaults to ./Dockerfile", `dockerContext` "relative to the repository root") | unknown (512 bytes, no `..`, no backslash) | none; filed in 077 |
| `env-value` | reserved ports "18012", "18013", "19099"; default `PORT` 10000; no key or value limit stated | one port stricter (18014, bex's own sidecar); NUL and 131072 bytes unknown | :184 records PORT validity, not the port list; filed as 076 and 077 |
| `registry-host` | no host field (the create body takes an enum) | a bex extension, nothing to compare | :258 records the extension, not the grammar |
| `repo-url` | "full `https://` URL"; no SSH rule, no length rule | https equal; GitHub SSH spellings and the 2048-byte cap unverified | ADR026:104 records the SSH spelling; filed in 077 |
| `docker-image` | "any registry Render can reach"; no length or private-host rule | unknown (512 bytes, private and single-label hosts refused) | :97 records the policy; filed in 077 |
| `custom-domain` | Punycode for Unicode names, `*.example.org` wildcards; silent on label length, IP literals, numeric TLDs | unknown; the public-suffix refusal is a bex superset | :196 records the public-suffix refusal; filed in 077 |

The corpus therefore pins current server behavior and claims parity for none of the unknown rows. The one place the first draft claimed otherwise, `cidr.json` saying host bits are accepted "as Render does", had no Render source and is reworded.

REST, GraphQL and MCP reach one validator per family through the service layer: `checkDisplayName` (`apps/service.go:5778`) and `TrimmedLabel` / `CheckDatastoreName` for labels, `core.ValidateAllowList` for allow lists, `store.RootDirProblem` through `checkRepoPath` and `checkDockerContextPath` (`apps/docker_context.go:124`) and `settings_check.go:207` for build paths, `core.CheckEnvValue` for values, `canonicalRegistryHost` for registry hosts, `store.ValidRepo` and `store.ValidateImage` for the source, and `claimableHostname` (`apps/domains.go:432`) for domains. No adapter carries its own copy.

Corrected during this task: the first draft described `build-path` as covering the Dockerfile Path and the Docker Build Context judged as typed. The Docker Build Context is normalized first (`checkDockerContextPath` calls `store.NormalizeRootDir`), so it belongs to `root-dir`; `build-path` is the Dockerfile Path alone.

Follow-ups filed: [075](../../075.md) (IPv4-only CIDR against the IPv6 entries accepted), [076](../../076.md) (reserved port 18014), [077](../../077.md) (limits with no Render source). Each leaves the corpus pinning today's behavior until it ships.

### Test coverage (t008)

Each piece and the test that fails when it is reverted (the mutations were run, not assumed):

| Piece | Fails when reverted |
| --- | --- |
| Server verdict of a family changes | `Test{Label,CIDR,EnvValue,RootDir,BuildPath,RepoURL,DockerImage,RegistryHost,CustomDomain}Corpus`; replacing `netip.ParsePrefix` with `net.ParseCIDR` in `ValidateCIDRs` fails `TestCIDRCorpus` on `/024`, `/08`, `/00` |
| Client verdict of a family changes | `validation-corpus.test.ts` "agrees with the client validator on every case"; dropping the zero-padded-prefix rule from `isValidCIDR` fails it with three named cases |
| A family without a client, or a client without a family | "has a client validator for every family, and a family for every client" |
| A corpus too thin to detect a stricter or looser client | per-family "holds enough cases of each kind", "fails a client one case stricter", "fails a client one case looser" |
| `serverOnly` leniency, and only that | `corpusMismatches` tests (four cases on a synthetic corpus); `validationcorpus.load` refuses a valid entry marked `serverOnly` |
| `FormTrim` as JavaScript's `trim` | `TestFormTrimIsJavaScriptsTrim`; swapping in `strings.TrimSpace` also fails `TestLabelCorpus` on the NEL entry |
| `Case.Text` repeat, prefix, suffix | `TestCaseTextAppliesRepeatBetweenPrefixAndSuffix` |
| `isValidLabel` trims first | corpus entries `"\nabc"`, `"abc\n"`; `resource-label.test.ts` "accepts" rows |
| Docker digest algorithm and length | corpus entries for a 63-hex sha256, `md5`, a sha256-length sha512, an upper-case digest |
| Custom domain top-level label rule | corpus entries `example.c`, `example.c0m`, `example.a-b`, `example.1a`, `example.1234` |
| `IMAGE_REF_RE` without exponential backtracking | `docker-image.test.ts` "refuses a long near-miss name without backtracking": on the old regex it fails at 1.2 s, 3.5 s and 104 ms against a 100 ms bound |
| The corpus runs the dashboard suite | `validation-corpus.test.ts` "is a test input of the dashboard suite" (fails with one glob removed); `scripts/ci_test_inputs.py` for the rest |

### Definition of done

1. Nine families, 350 entries, boundary inputs in each (zero-padded numbers, the exact limit and one over, multibyte against byte and code-point limits, NUL, BEL, newline, padded text); each has a Go test on the real validator.
2. `validation-corpus.test.ts` asserts the nine client validators against the same files; stricter and looser self-tests fail per family; leniency only on the 17 `serverOnly` entries.
3. Three families diverged and were fixed in the client; the Render comparison filed 075, 076 and 077.
4. `python3 scripts/ci_test_inputs.py` passes; the dashboard workflow lists the corpus in both `paths` and a test holds it; `yarn lint` and `yarn test` pass; `cd lego/backend && go test ./...` runs in `scripts/ship-gates.sh` against real Postgres and OpenFGA before the push; `make lint` reports 0 issues in all four modules.
