### services list
```
$ render services -o json
Error: json: cannot unmarshal bool into Go struct field Service.service.autoDeploy of type client.AutoDeploy
```

### services show
```
$ render services -o json
Error: json: cannot unmarshal bool into Go struct field Service.service.autoDeploy of type client.AutoDeploy
```

### postgres list
```
$ render postgres -o json
Render CLI vdev

Manage Render Postgres databases (alias: pg)

USAGE
  render postgres <subcommand> [flags]

SUBCOMMANDS
  create               Create a new Render Postgres database
  delete               Delete a Render Postgres database
  get                  Get details of a Render Postgres database
  list                 List Render Postgres databases
  resume               Resume a suspended Render Postgres database
  suspend              Suspend a Render Postgres database
  update               Update a Render Postgres database

FLAGS
  -h, --help              Show help for this command
      --confirm           Skip all confirmation prompts
  -o, --output <FORMAT>   Set output format to interactive, json, yaml, or text. Auto-switches to text on non-TTY (default: "interactive")

DETAILS
Manage Render Postgres databases.

Use "render postgres <subcommand> --help" for more information about a command.
```

### postgres get
```
$ render postgres get pg-cli-test -o json
Error: postgres not found
```

### keyvalues list
```
$ render keyvalues -o json
Render CLI vdev

Manage Render Key Value instances (alias: kv)

USAGE
  render keyvalues <subcommand> [flags]

SUBCOMMANDS
  create               Create a new Render Key Value instance
  delete               Delete a Render Key Value instance
  get                  Get details of a Render Key Value instance
  list                 List Render Key Value instances
  resume               Resume a suspended Render Key Value instance
  suspend              Suspend a Render Key Value instance
  update               Update a Render Key Value instance

FLAGS
  -h, --help              Show help for this command
      --confirm           Skip all confirmation prompts
  -o, --output <FORMAT>   Set output format to interactive, json, yaml, or text. Auto-switches to text on non-TTY (default: "interactive")

Use "render keyvalues <subcommand> --help" for more information about a command.
```

### keyvalues get
```
$ render keyvalues get kv-cli-test2 -o json
Error: Multiple Key Value instances found with name 'kv-cli-test2'. Pass the Key Value ID, or use --environment <id|name> to disambiguate.
```

### workspaces list
```
$ render workspaces -o json
[
  {
    "email": "",
    "id": "tea-d9bgud1jg4r664ajub10",
    "name": "tea-d9bgud1jg4r664ajub10",
    "type": "team"
  }
]```

### workspace current
```
$ render workspace current -o json
{
  "email": "",
  "id": "tea-d9bgud1jg4r664ajub10",
  "name": "tea-d9bgud1jg4r664ajub10",
  "type": "team"
}```

### projects list
```
$ render projects -o json
null```

### whoami
```
$ render whoami -o json
Error: failed to get current user: received response code 404: 404 page not found

```

### postgres list (explicit)
```
$ render postgres list -o json
{
  "data": [
    {
      "connectionPool": "",
      "createdAt": "0001-01-01T00:00:00Z",
      "dashboardUrl": "",
      "databaseName": "",
      "databaseUser": "",
      "diskAutoscalingEnabled": false,
      "highAvailabilityEnabled": false,
      "id": "",
      "ipAllowList": [],
      "name": "",
      "owner": {
        "email": "",
        "id": "",
        "name": "",
        "type": ""
      },
      "plan": "",
      "readReplicas": [],
      "region": "",
      "role": "",
      "status": "",
      "suspended": "",
      "suspenders": null,
      "updatedAt": "0001-01-01T00:00:00Z",
      "version": "",
      "projectId": null
    }
  ]
}```

### keyvalues list (explicit)
```
$ render keyvalues list -o json
{
  "data": [
    {
      "id": "",
      "name": "",
      "plan": "",
      "region": "",
      "status": "",
      "createdAt": "0001-01-01T00:00:00Z",
      "updatedAt": "0001-01-01T00:00:00Z",
      "ownerId": "",
      "projectId": null,
      "environmentId": null,
      "ipAllowList": []
    },
    {
      "id": "",
      "name": "",
      "plan": "",
      "region": "",
      "status": "",
      "createdAt": "0001-01-01T00:00:00Z",
      "updatedAt": "0001-01-01T00:00:00Z",
      "ownerId": "",
      "projectId": null,
      "environmentId": null,
      "ipAllowList": []
    }
  ]
}```

### deploys list
```
$ render deploys list whoami-cli-test -o json
Error: failed to list deploys: json: cannot unmarshal string into Go struct field Deploy.deploy.image of type struct { Ref *string "json:\"ref,omitempty\""; RegistryCredential *string "json:\"registryCredential,omitempty\""; Sha *string "json:\"sha,omitempty\"" }
```

### deploys create
```
$ render deploys create whoami-cli-test --confirm -o json
unknown error
```

### restart
```
$ render restart whoami-cli-test --confirm -o json
Error: failed to restart resource: unknown resource type
```

### logs
```
$ render logs -r whoami-cli-test --limit 5 -o json
Error: error processing arguments: json: cannot unmarshal bool into Go struct field Service.service.autoDeploy of type client.AutoDeploy
```

### jobs list
```
$ render jobs list whoami-cli-test -o json
Error: received response code 404: 404 page not found

```

### jobs create
```
$ render jobs create whoami-cli-test --start-command echo hi --plan free --confirm -o json
Error: unknown flag: --plan
Usage:
  render jobs create [serviceID] [flags]

Examples:
  # Create a job for a service
  render jobs create srv-abc123 --start-command "bundle exec rake task"

  # Create a job with a specific plan
  # See https://render.com/docs/one-off-jobs for available job plans
  render jobs create srv-abc123 --start-command "npm run worker" --plan-id plan-srv-006

Flags:
  -h, --help                   help for create
      --plan-id string         Set the plan ID for the job (Optional)
      --start-command string   Set the job start command

Global Flags:
      --confirm         Skip all confirmation prompts
  -o, --output string   Set output format to interactive, json, yaml, or text. Auto-switches to text on non-TTY (default "interactive")

```

### environments
```
$ render environments prj-doesnotexist -o json
Error: unknown error
```

### blueprints validate
```
$ render blueprints validate examples/whoami-app.yaml -o json
Error: validation request failed with status 400: {"error":"bad request"}

```

### workflows list
```
$ render workflows list -o json
Error: received response code 404: 404 page not found

```

### login
```
$ render login --confirm -o json
Success: CLI is already authenticated.
```

### logout
```
$ render logout --confirm -o json
You are authenticated via the RENDER_API_KEY environment variable.
This command cannot remove environment variable credentials.
To revoke access, delete the API key from your Render Dashboard.
```

### psql non-interactive
```
$ render psql pg-cli-test --command SELECT 1; --output text
run:5: command not found: timeout
```

### ssh
```
$ render ssh whoami-cli-test --confirm -o json
run:5: command not found: timeout
```

### kv-cli
```
$ render kv-cli kv-cli-test --confirm -o json
run:5: command not found: timeout
```

### pgcli
```
$ render pgcli pg-cli-test --confirm -o json
run:5: command not found: timeout
```

### docs
```
$ render docs
run:5: command not found: timeout
```

### skills
```
$ render skills -o json
run:5: command not found: timeout
```

### ea
```
$ render ea -o json
run:5: command not found: timeout
```

### psql non-interactive
```
$ render psql pg-cli-test --command SELECT 1; --output text
Error: IP address (162.224.81.143) not in allow list for 
(exit 1)
```

### ssh
```
$ render ssh whoami-cli-test --confirm -o json
Error: `render ssh` can only be used in interactive mode
(exit 1)
```

### kv-cli
```
$ render kv-cli kv-cli-test --confirm -o json
Error: `render kv-cli` can only be used in interactive mode
(exit 1)
```

### pgcli
```
$ render pgcli pg-cli-test --confirm -o json
Error: `render pgcli` can only be used in interactive mode
(exit 1)
```

### docs
```
$ render docs
(exit 0)
```

### skills
```
$ render skills -o json
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x2 addr=0x8 pc=0x1051907a8]

goroutine 1 [running]:
github.com/render-oss/cli/pkg/tui.(*StackModel).Push(0x105fdb8b8?, {{0x105fd8100, 0x7b350f9527a0}, {0x0, 0x0}, {0x1054d25b0, 0x6}})
	/Volumes/nvme4tbfish/projects/bex9/cli/pkg/tui/stack.go:105 +0x28
github.com/render-oss/cli/cmd.init.func16(0x1060d8520?, {0x1054d07c1?, 0x4?, 0x1054d071d?})
	/Volumes/nvme4tbfish/projects/bex9/cli/cmd/skills.go:53 +0x170
github.com/spf13/cobra.(*Command).execute(0x1060d8520, {0x7b350fcf1b60, 0x2, 0x2})
	/Users/tianpan/.go/pkg/mod/github.com/spf13/cobra@v1.8.1/command.go:985 +0x804
github.com/spf13/cobra.(*Command).ExecuteC(0x7b350fc1c608)
	/Users/tianpan/.go/pkg/mod/github.com/spf13/cobra@v1.8.1/command.go:1117 +0x344
github.com/spf13/cobra.(*Command).Execute(...)
	/Users/tianpan/.go/pkg/mod/github.com/spf13/cobra@v1.8.1/command.go:1041
github.com/render-oss/cli/cmd.Execute()
	/Volumes/nvme4tbfish/projects/bex9/cli/cmd/root.go:338 +0x98
main.main()
	/Volumes/nvme4tbfish/projects/bex9/cli/main.go:9 +0x1c
(exit 2)
```

### ea
```
$ render ea -o json
Render CLI vdev

Use early access commands

USAGE
  render ea <subcommand> [flags]

SUBCOMMANDS
  objects              Manage object storage in early access
  sandbox              Manage sandboxes
  sandbox-groups       Manage sandbox groups

FLAGS
  -h, --help              Show help for this command
      --confirm           Skip all confirmation prompts
  -o, --output <FORMAT>   Set output format to interactive, json, yaml, or text. Auto-switches to text on non-TTY (default: "interactive")

EXAMPLES
  # List early access object storage resources
  render ea objects list --region=oregon

DETAILS
These commands are in early access and are subject to change.

Use "render ea <subcommand> --help" for more information about a command.
(exit 0)
```

### services instances
```
$ render services instances whoami-cli-test -o json
Error: failed to list instances: 404 Not Found
(exit 1)
```

### services update
```
$ render services update whoami-cli-test --num-instances 2 --confirm -o json
Error: --num-instances is not supported for update (use the dashboard to change instance count)
(exit 1)
```

### services delete
```
$ render services delete whoami-cli-test --confirm -o json
Error: json: cannot unmarshal bool into Go struct field Service.service.autoDeploy of type client.AutoDeploy
(exit 1)
```

### postgres suspend
```
$ render postgres suspend pg-cli-test --confirm -o json
Error: postgres not found
(exit 1)
```

### postgres update
```
$ render postgres update pg-cli-test --plan free --confirm -o json
Error: postgres not found
(exit 1)
```

### keyvalues suspend
```
$ render keyvalues suspend kv-cli-test --confirm -o json
Error: Multiple Key Value instances found with name 'kv-cli-test'. Pass the Key Value ID, or use --environment <id|name> to disambiguate.
(exit 1)
```

### keyvalues update
```
$ render keyvalues update kv-cli-test --plan free --confirm -o json
Error: Multiple Key Value instances found with name 'kv-cli-test'. Pass the Key Value ID, or use --environment <id|name> to disambiguate.
(exit 1)
```

### deploys cancel
```
$ render deploys cancel whoami-cli-test dep-doesnotexist --confirm -o json
Error: failed to cancel deploy: unknown error
(exit 1)
```

### jobs cancel
```
$ render jobs cancel whoami-cli-test job-doesnotexist --confirm -o json
Error: failed to cancel job: received response code 404: 404 page not found

(exit 1)
```


## Re-verification: KeyValue fix (2026-07-15, commit dfff3034)

Re-ran against a freshly rebuilt dev-9 bex-api (`6993b3dd`) after the
already-merged `dfff3034` shipped. Full `keyvalues` lifecycle, real data:

### keyvalues create (post-fix)
```
$ render keyvalues create --name kv-fixed-verify --confirm -o json
{"data":{"id":"kv-fixed-verify","name":"kv-fixed-verify","plan":"free","region":"","status":"creating","createdAt":"2026-07-15T05:17:04Z","updatedAt":"0001-01-01T00:00:00Z","ownerId":"","projectId":null,"environmentId":null,"ipAllowList":[]}}
```

### keyvalues list (post-fix)
```
$ render keyvalues list -o json
{"data":[{"id":"kv-fixed-verify","name":"kv-fixed-verify","plan":"free","region":"","status":"creating","createdAt":"2026-07-15T05:17:04Z","updatedAt":"0001-01-01T00:00:00Z","ownerId":"","projectId":null,"environmentId":null,"ipAllowList":[]}]}
```

### keyvalues get (by name, post-fix)
```
$ render keyvalues get kv-fixed-verify -o json
{"data":{"id":"kv-fixed-verify","name":"kv-fixed-verify","plan":"free","region":"","status":"creating", ...}}
```

### keyvalues update / suspend / resume / delete (post-fix)
```
$ render keyvalues update kv-fixed-verify --plan free --confirm -o json
{"data":{...},"diff":{}}
$ render keyvalues suspend kv-fixed-verify --confirm -o json
{"data":{...},"meta":{"suspended":true}}
$ render keyvalues resume kv-fixed-verify --confirm -o json
{"data":{...,"status":"available"}}
$ render keyvalues delete kv-fixed-verify --confirm -o json
{"data":{...},"meta":{"deleted":true}}
$ render keyvalues list -o json
{"data":[]}
```

Every `keyvalues` subcommand now works end to end. Spot-checked the other
root causes the same commit touches (out of this pass's KeyValue-focused
scope, but recorded for accuracy):

```
$ render whoami -o json
Name:
Email:
(exit 0 — route exists, no more 404; but a machine caller's email/name
resolve empty — GET /v1/users returns {"email":"","name":""})

$ render services -o json
null   (was: json: cannot unmarshal bool ... AutoDeploy — now clean)

$ render postgres list -o json
{"data":[]}   (was: zero-valued garbage — now a clean, correctly-shaped empty list)

$ render postgres get pg-cli-test -o json
Error: No Postgres database named 'pg-cli-test' in workspace tea-d9bhfihjg4r47t9b5ujg. To search another workspace, run `render workspace set <name|ID>`, or pass the Postgres database ID instead.
(was: an opaque RC3 failure — now a real, specific Render-shaped error,
confirming RC1's error-envelope fix too)
```

`go build ./...` and `go test ./internal/{core,apps,deploys,postgres,keyvalue,workspaces,logs,api}/...` all pass on `6993b3dd`.

## RC11: KeyValue owner/options wire-shape bug — found + fixed (2026-07-15, this session)

User directive: "are those features really working well? try run them again
and fix problems" — re-verified the "✅ Fixed" `keyvalues` rows field-by-field
instead of trusting exit-code/id-presence checks, and found a real,
previously-missed bug.

### Pre-fix: silently empty/missing fields
```
$ render keyvalues create --name kv-real-test --confirm -o json
{
  "data": {
    "id": "kv-real-test", "name": "kv-real-test", "plan": "free",
    "region": "", "status": "creating", ...
    "ownerId": "",             <- WRONG: raw REST had a real tenant id here
    ...(no maxmemoryPolicy or persistenceMode keys at all)
  }
}
$ curl .../v1/key-value/kv-real-test   # raw REST, same instance
{"id":"kv-real-test",...,"maxmemoryPolicy":"allkeys_lru","persistenceMode":"journal_snapshot",...,"ownerId":"tea-d9bhnchjg4r9asmbuq4g"}
```
The CLI's own KeyValueDetail decode target expects `owner: {id,name,type}`
(nested) and `options: {maxmemoryPolicy,persistenceMode}` (nested) — bex-api
sent flat `ownerId`/`maxmemoryPolicy`/`persistenceMode`, so those fields
silently zero-valued/vanished client-side even though the raw wire data was
correct. `keyvalues create --ip-allow-list` also 400'd outright:
```
$ render keyvalues create --name kv-iptest --ip-allow-list "cidr=10.0.0.0/8,description=internal" --confirm -o json
Error: unknown error
# via logging proxy: RESP 400: {"error":"bad request body"}
# (ipAllowList as []{cidrBlock,description} objects failed to decode into bex's []string)
```

### Fix
`lego/backend/internal/keyvalue/rest.go`: new REST-boundary-only wire types
(`renderKeyValue`, `keyValueOwner`, `keyValueOptionsView`, `ipAllowEntry`) +
`toRenderKeyValue` mapper, used by every single-item handler (create/get/
patch/suspend/resume/ip-allow-list PUT); POST body decode now accepts
Render's `ipAllowList` shape via an embedded-struct wire decode
(`createKeyValueWire`-equivalent inline struct); all three hand-written
`{"error":"bad request body"}` literals replaced with a `writeBadRequestBody`
helper routed through `core.WriteErr` (they were bypassing RC1's fix
entirely). `KeyValueView`/`CreateKeyValueRequest` (the GraphQL/MCP-shared
core types) are unchanged — this is a REST-adapter-only translation, same
altitude as the existing `keyValueWithCursor` list envelope.

### Post-fix: full lifecycle, every field verified live
```
$ render keyvalues create --name kv-real-verify --ip-allow-list "cidr=10.0.0.0/8,description=verify" --confirm -o json
{
  "data": {
    "id": "kv-real-verify", "name": "kv-real-verify", "plan": "free",
    "ownerId": "tea-d9bhu49jg4rdiv01qi90", "ownerType": "team",
    "ipAllowList": [{"cidrBlock": "10.0.0.0/8", "description": ""}],
    "maxmemoryPolicy": "allkeys_lru", "persistenceMode": "journal_snapshot",
    ...
  }
}
```
(`description` is honestly empty, not fabricated — the KeyValue CRD,
`lego/types/v1alpha1/keyvalue_types.go`, has no per-CIDR description field to
store it in.) Same full field set confirmed present across list/get/update/
suspend/resume/delete. `go build ./...` and `go test ./...` (whole backend,
fresh, not cached) both green. `scripts/cli-compat.sh verify` strengthened
with a `checkFields` helper (asserts a whole field set together, not one at a
time — the exact gap that let RC11 slip past the original "fixed" claim) and
re-run clean twice in a row with no leftover test resources.

Not fixed this pass: Postgres has the identical bug (unverified but highly
likely — same generated-type pattern, `PostgresDetail.Owner Owner`) — filed
as `.pm/w8/005.md`.

## whoami populated-email case — verified (2026-07-15, harness wiring, no code change)

RC6's one remaining caveat was that the populated-email path (`ownerEmail`'s
Kratos-admin lookup) had never been observed live — the dev harnesses never
set `BEX_KRATOS_ADMIN_URL`, so a machine (API-key) caller always saw
`{"email":"","name":""}`. Wired it into dev-9's `up.sh` (a self-healing
`kratos-admin` port-forward on `:$KRATOS_ADMIN_PORT` = 57090, `57000 + N*10`,
plus `BEX_KRATOS_ADMIN_URL=http://localhost:57090` in bex-api's env; dev-4
got the identical change at `:57040`), re-ran `up.sh`, and re-verified:

### Pre-wiring baseline (same key, same session)

```
$ scripts/cli-compat.sh whoami -o json
Name:
Email:
```

### Post-wiring

```
$ scripts/cli-compat.sh whoami -o json
Name:
Email: cli-compat-1784094481-56006@example.com

$ scripts/cli-compat.sh workspaces -o json
[{"email":"cli-compat-1784094481-56006@example.com","id":"tea-d9bhu49jg4rdiv01qi90","name":"tea-d9bhu49jg4rdiv01qi90","type":"team"}]
```

The email is exactly the `CLI_COMPAT_EMAIL` `bootstrap-key.sh` registered —
the API-key caller correctly reports the human who minted the key, resolved
through `workspaces.ownerEmail`'s earliest-admin Kratos-admin lookup, and
`GET /v1/owners` (`render workspaces`) populates via the same lookup,
confirming the two routes share one code path. `Name:` stays empty by design
(Kratos' default identity schema carries only email — an honest subset).
Checklist row `whoami` flipped ◐ → ✅.
