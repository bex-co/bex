# w2/m166 local process acceptance — 2026-10-02

The repaired backend passed real CLI writes and authenticated datastore connections through the production SNI proxy binaries. This is local integration acceptance with seeded Kubernetes resources, not a production rollout or a managed provisioning test.

## Substrate and limits

- The normal dev-2 harness could not create its first CNPG database. Its stale endpoint/CA was repaired with a TLS-verified, atomically refreshed local kubeconfig, but the shared cluster's CNPG webhook refused connections; the apiserver/scheduler were unready and operators repeatedly lost leases. Two idempotent startup attempts failed before auth databases or accounts were created. Other workstreams' namespaces and platform workloads were not repaired or redeployed.
- The fallback ran an isolated envtest Kubernetes API server, the current composed Bex REST/GraphQL/MCP API, and separately built production `pg-sni-proxy` / `kv-sni-proxy` binaries. Both proxies consumed real Kubernetes watch updates.
- Disposable Docker Postgres 17, Valkey 8, and PgBouncer 1.23.1 provided real authenticated protocols. SQL used `sslmode=verify-full` and a private test CA; Valkey used TLS, SNI, the same CA, and password authentication. Source rules selected the exact private Docker client address. The nonmatching control used 198.51.100.17/32.
- CR identities were minted by `internal/id`. Credentials, ready status, public hostnames, the pooler flag, and the replica route were seeded as harness setup. The replica hostname targeted the primary Postgres container: its **route admission/withdrawal** was tested; physical replication and read-only behavior were not.
- A fixed local introspection fixture supplied a test identity; real Ory login and cross-workspace OpenFGA authorization were not exercised here. No production credentials were used. Datastore internal checks used authenticated direct connections on the isolated Docker network; Kubernetes NetworkPolicy/Cilium behavior was not tested.
- Ready status intentionally retained the old external hostname during clear. The real proxies still withdrew it on `spec.public=false`, proving this does not depend on status convergence or connection-info suppression. Operator reconciliation, create defaults, legacy state boundaries, IPv6, protected/foreign writes, and HTTP-service semantics have separate automated coverage.

## Results

- Installed Bex **v0.2.1**, embedding Render CLI **v2.27.0**, and an unchanged upstream Render **v2.27.0** binary at pin `a764810a768202704e7206eb7b87a47211fcd98e` each completed source-allow → nonmatching deny → clear remains denied → source restore.
- **20 CLI commands exited 0.** The four clear/text pairs returned an empty list and `external connections blocked` text. Direct clients reused the same valid passwords and CA across every transition.
- **122 protocol checks matched their expected results, including 48 external denials and 14 internal successes; no probe timed out.** Admitted Valkey returned PONG and Postgres returned 42. Denied routes closed TLS rather than relying on a CLI refusal. PG primary, real PgBouncer, and the replica hostname variant followed the same rule.
- Dedicated REST PUT, GraphQL, and MCP clear/restore each changed live admission for both datastores, completing all eight logical writers with CLI-driven REST PATCH.
- REST and MCP dry-run clear left existing connections available; invalid CIDRs returned 400 and anonymous clears returned 401 without changing admission. Omitted list fields preserved existing public/private intent. Explicit `public:true` with [] admitted traffic; `public:false` with nonempty rules denied it.
- Environment rules continued to combine with resource rules using AND. An explicit resource rewrite did not remove the inherited nonmatching restriction; internal access stayed available. Removing that test environment restriction restored admission.
- Authenticated connection-info after the sequence matched the original seeded passwords. No credential, URI, token, or kubeconfig is in the sanitized evidence.

Representative commands (secrets were supplied only through private process environments/files):

```sh
bex keyvalues update <owned-red-id> --clear-ip-allow-list --confirm -o json
bex postgres update <owned-dpg-id> --clear-ip-allow-list --confirm -o json
render keyvalues update <owned-red-id> --ip-allow-list cidr=<owned-source>/32,description=owned-client --confirm -o json
psql -X -A -t -U qa -d m166 -c 'SELECT 42;'
valkey-cli --raw --tls --sni <owned-host> --cacert <private-test-ca> -h <private-proxy> -p 6379 PING
```

## Cleanup

Both owned datastore records were deleted through authenticated REST (204); detail returned 404 and list responses excluded their IDs. All four external route variants then rejected new connections while direct authenticated backends still answered, isolating route deletion from server shutdown. Seeded Kubernetes Secrets were removed. All five disposable containers, their anonymous volumes, the private Docker network, the API listener, and envtest processes were removed. Generated passwords, the test token, kubeconfig copies, connection-info captures, and private keys were deleted. An initial interrupted envtest startup's own etcd directory and certificate directory were also removed after identifying its fixture record.

The failed normal dev-2 bring-up was taken down using `scripts/dev-env.sh 2 down`. Both `dev-2` and `dev-2-auth` subsequently returned NotFound; no auth databases or accounts had been created.

Full sanitized command exits/readbacks and protocol results: `evidence.jsonl`. Binary hashes and versions: `versions.json`. Counts: `summary.json`. These report local verification only.
