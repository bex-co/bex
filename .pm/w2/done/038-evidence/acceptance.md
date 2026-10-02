# w2/038 local-process acceptance — Postgres create defaults

On 2026-10-02, all 93 current-code assertions and 14 original-code comparison assertions passed using the installed Bex v0.2.1 launcher, unchanged pinned Render v2.27.0 binary, and a privately composed Bex API backed by real envtest storage. Seventeen current-code Database CRs and two original-code CRs were created, read, deleted through REST, and verified absent with GET 404.

The current REST adapter SHA-256 was `99831ff469fa2bef2cde70f3827c5eb01cc60493df4cb20c4b2a2728081c77a3`, based on `76b2697d5975ef790c58be2c8f899b5f1fd964b8`. That source remained unchanged through compilation and acceptance. `source-manifest.json` records both adapter variants and the compiled API hash. `versions.json` records the client versions and hashes; the Render binary retained SHA-256 `35950c3439f70fbe05323fbd04ddb7cad0448dd43215ac9b4125e986e08a2e90` at pin `a764810a768202704e7206eb7b87a47211fcd98e`.

The pinned builder in `pkg/postgres/create.go:123` sets `IpAllowList` only for a nonempty input slice. Its generated `PostgresPOSTInput` field in `pkg/client/types_gen.go:2746` uses a pointer with `omitempty`. The runtime captures confirmed both binaries omitted `ipAllowList` and the Bex-only `public` field on default creation. The verified commands were:

```text
bex postgres create --confirm --name <owned-name> --plan free --version 17 -o json
render postgres create --confirm --name <owned-name> --plan free --version 17 -o json
bex postgres get <owned-id> -o json
bex postgres get <owned-id> -o text
render postgres get <owned-id> -o json
render postgres get <owned-id> -o text
```

Default create returned `public:true` and explicit rules `0.0.0.0/0` and `::/0`, identically in REST readback and the persisted CR. Both client JSON outputs retained those rules, and their text output listed them without the misleading “external connections blocked” message. Render documents `0.0.0.0/0` as its default rule. Bex also records `::/0` to preserve its existing unrestricted IPv6 behavior; this is an explicit Bex preservation decision, not a claim that Render creates the IPv6 entry. [Render Postgres access documentation](https://render.com/docs/postgresql-creating-connecting#restricting-external-access), checked 2026-10-02.

Both binaries also created a restricted resource through `--ip-allow-list cidr=203.0.113.5/32,description=owned-docs-range`; the rule and description survived request serialization, creation, persisted state, and text/JSON readback.

The composed REST matrix passed:

| Input                                 | Public | Readback rules                              |
| ------------------------------------- | ------ | ------------------------------------------- |
| Omitted allowlist and public          | true   | Both wildcard rules                         |
| Explicit empty allowlist              | false  | Empty                                       |
| Restricted allowlist                  | true   | Restricted rule preserved                   |
| `public:false`, omitted or empty list | false  | Empty                                       |
| `public:false`, restricted list       | false  | Restricted rule preserved                   |
| `public:true`, omitted list           | true   | Both wildcard rules                         |
| `public:true`, empty list             | true   | Empty, preserving the explicit Bex override |
| `public:true`, restricted list        | true   | Restricted rule preserved                   |

Public request validation rejects null before the REST adapter: `ipAllowList:null` returned HTTP 400 with `id:"bad_request"` and both `error` and `message` equal to `invalid request body at /ipAllowList`. `public:null` produced the same shape naming `/public`. Neither created a resource. These controls preserve validation; null is not public-API omission.

Native GraphQL `createDatabase` and MCP `create_postgres` retained private, empty-list defaults. Their explicit `public:true` controls retained the existing public/empty native behavior. The change was confined to REST create defaulting.

The process guard used the real `bex psql` and `render psql` commands with an owned fake `psql` executable placed first on their private PATH. Three resources received seeded Ready status and connection-info Secrets in the private envtest. For each binary, a default public create passed the CLI allowlist guard and launched the probe with `sslmode=verify-full`; an explicit-empty create returned the CLI’s `not in allow list` error and never launched it. The executable recorded only nonsecret invocation metadata and explicitly attempted no database connection. IP addresses in errors were redacted.

A second API binary was built using Go’s overlay mechanism to substitute only the original REST adapter, without modifying the checkout. Default create through both binaries then produced persisted `public:true` with empty allowlist readback, and both text renderers reported external connections blocked. `original-evidence.jsonl` records these counterexamples and successful cleanup. The original-code checks deliberately confirm the reported regression, rather than claiming the original code satisfies the repaired contract.

This acceptance used real HTTP CLI/REST/GraphQL/MCP requests and Kubernetes CR persistence, with a fixed local introspection identity and workspace resolver. It did not provision managed Postgres, exercise a Kubernetes operator, use production credentials, test FGA/database-backed authorization, or establish a database network connection. The earlier m166 protocol transition evidence remains separate. Harness adjustments were limited to respecting the existing 30-character name limit and treating explicit null as a schema refusal.

Cleanup removed all 19 created resources through the composed REST APIs, with GET 404 after every delete. Shutdown then confirmed zero Database CRs, deleted the six seeded Secrets, requested deletion of both private namespaces, stopped both APIs and their envtest control planes, and verified all exposed API/control listeners were closed. Both private OAuth fixtures, database passwords, and all CLI config directories were removed. No containers, shared cluster resources, production resources, or repository files were changed. `cleanup.json` and `sanitization.json` record the cleanup and export checks.
