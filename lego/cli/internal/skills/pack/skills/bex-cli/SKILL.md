---
name: bex-cli
description: >-
  Installs and uses the bex CLI for deploys, logs, SSH, psql, Blueprint
  validation, and automation. Use when the user needs to run bex CLI
  commands, script deploys in CI/CD, authenticate with an API key, query
  services non-interactively, or troubleshoot CLI auth issues.
  Trigger terms: bex CLI, bex login, bex deploys, bex logs,
  bex ssh, bex psql, bex blueprints validate, bex skills,
  BEX_ACCESS_TOKEN, non-interactive, CI/CD deploy.
license: MIT
compatibility: bex CLI v2.7.0+ (Homebrew, Linux/macOS, direct download)
metadata:
  author: bex
  version: "1.0.0"
  category: operations
---

# bex CLI

The bex CLI manages services, databases, and deployments from the terminal. Supports interactive use, non-interactive scripting, and CI/CD automation.

Before installing, authenticating, selecting a workspace, or running commands, read `references/bex-access.md` for the current access and command-discovery workflow.

## When to Use

- **Deploying** a service from the terminal or CI/CD
- **Tailing logs** in real time
- **Opening psql** to a bex Postgres database
- **SSHing** into a running service or launching an ephemeral shell
- **Validating** a `render.yaml` Blueprint
- **Scripting** bex operations in CI/CD pipelines
- **Installing** agent skills for AI coding tools

## Command Reference

Use the installed CLI help or generated command reference as directed by `references/bex-access.md` for the exact command surface and flags. The patterns below cover common operational workflows.

### Deploy patterns

```bash
# Deploy and wait for completion (exits non-zero on failure)
bex deploys create srv-xxx --wait --confirm -o json

# Deploy a specific commit
bex deploys create srv-xxx --commit abc123 --wait --confirm

# Deploy a specific Docker image
bex deploys create srv-xxx --image ghcr.io/org/app:v1.2.3 --wait --confirm
```

### Database queries

```bash
# Single query, JSON output
bex psql db-xxx -c "SELECT NOW();" -o json

# CSV output via psql passthrough
bex psql db-xxx -c "SELECT id, email FROM users;" -o text -- --csv
```

## CI/CD Example (GitHub Actions)

```yaml
name: Deploy to bex
on:
  push:
    branches: [main]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Install bex CLI
        run: |
          curl --fail-with-body --silent --show-error --location \
            https://raw.githubusercontent.com/bex-co/bex/main/scripts/install-bex.sh \
            --output /tmp/install-bex-cli.sh
          sudo BEX_INSTALL_DIR=/usr/local/bin sh /tmp/install-bex-cli.sh
          bex --version
      - name: Deploy
        env:
          BEX_KEY_ID: ${{ secrets.BEX_KEY_ID }}
          BEX_KEY_SECRET: ${{ secrets.BEX_KEY_SECRET }}
        run: |
          export BEX_ACCESS_TOKEN="$(curl -fsS -X POST https://oauth.bex.co/oauth2/token \
            -d "grant_type=client_credentials&client_id=$BEX_KEY_ID&client_secret=$BEX_KEY_SECRET" | jq -r .access_token)"
          bex deploys create ${{ secrets.BEX_SERVICE_ID }} --wait --confirm -o json
```

If reproducible builds require a pinned CLI, select a currently supported release and verify its asset name in the current release documentation instead of copying a version from this skill.

## Local Config

Config file: `~/.bex/cli.yaml`

Override the configuration directory with `BEX_CLI_CONFIG_DIR`. `BEX_CLI_CONFIG_PATH` is a legacy file-level override retained for compatibility and is scheduled for deprecation.

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Deploying without `--wait` in CI | Add `--wait` so the job fails on deploy failure |

## References

| Document | Contents |
|----------|----------|
| `references/bex-access.md` | Current CLI authentication, workspace, documentation, output, and authorization workflow |
| `references/command-cheatsheet.md` | Common operational command examples and scripting patterns |

## Related Skills

- **bex-deploy** — End-to-end deploy flows, MCP operations, Dashboard deeplinks
- **bex-blueprints** — `render.yaml` authoring and validation
- **bex-postgres** — Database connections, `bex psql` usage
- **bex-debug** — Using `bex logs` and `bex ssh` for troubleshooting

<!-- shared:documentation-retrieval -->
## Current documentation retrieval

bex implements Render's documented platform behavior, so this skill links Render's documentation as the reference for how a feature behaves, and bex's own documentation (`https://bex.co/docs`) for its CLI and MCP server. Read Render's pages for behavior only: operate through bex (`bex` CLI, `https://api.bex.co/mcp`, `https://dashboard.bex.co`), never through the Render CLI, MCP server or dashboard those pages mention. Where bex's behavior differs, bex's documentation wins.

Whenever this skill directs you to consult current documentation:

1. Retrieve the linked Markdown document directly with an available URL-fetching tool or HTTP client, such as `curl`. Do not substitute web-search summaries for the document.
2. Confirm that retrieval succeeded and returned the expected document, then read its contents. Saving a file or printing its path is not sufficient.
3. If the request fails or your tool cannot read the Markdown response, open and read the linked HTML version instead.
4. If neither version can be retrieved, disclose that the current reference is unavailable and follow any topic-specific fallback in the skill. Use bundled guidance only for stable constraints, and do not guess at changeable platform details.

When a task requires multiple references, apply this workflow to each one and distinguish the documents you verified from those that remain unavailable.
<!-- /shared:documentation-retrieval -->
