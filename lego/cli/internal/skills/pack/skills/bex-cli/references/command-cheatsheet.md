# bex CLI Command Cheatsheet

For authentication, workspace selection, exact command discovery, and non-interactive output guidance, first read [bex-access.md](bex-access.md).

## Services

```bash
# List all services
bex services
bex services -o json --confirm

# Filter by type (via jq)
bex services -o json --confirm | jq '.[] | select(.type == "web_service")'
```

## Deploys

```bash
# Trigger deploy (interactive)
bex deploys create

# Trigger deploy (non-interactive)
bex deploys create srv-xxx --confirm -o json

# Deploy and wait for completion
bex deploys create srv-xxx --wait --confirm -o json

# Deploy specific commit
bex deploys create srv-xxx --commit abc123 --wait --confirm

# Deploy specific Docker image
bex deploys create srv-xxx --image ghcr.io/org/app:v1.2.3 --wait --confirm

# List deploys
bex deploys list srv-xxx -o json --confirm
```

## Logs

```bash
# View recent logs
bex logs -r srv-xxx

# Stream logs (follow mode)
bex logs -r srv-xxx --tail

# Logs in JSON format
bex logs -r srv-xxx -o json
```

## SSH

```bash
# SSH into running instance
bex ssh srv-xxx

# Ephemeral shell (isolated, no running service process)
bex ssh srv-xxx --ephemeral
bex ssh srv-xxx -e
```

## Database (psql)

```bash
# Interactive psql session
bex psql db-xxx

# Single query
bex psql db-xxx -c "SELECT NOW();"

# JSON output
bex psql db-xxx -c "SELECT id, name FROM users LIMIT 5;" -o json

# CSV output (via psql passthrough)
bex psql db-xxx -c "SELECT id, email FROM users;" -o text -- --csv

# Quiet output (no headers, unaligned)
bex psql db-xxx -c "SELECT count(*) FROM users;" -o text -- -t -A
```

## Blueprints

```bash
# Validate render.yaml in current directory
bex blueprints validate

# Validate specific file
bex blueprints validate path/to/render.yaml
```

## Skills

```bash
# Interactive skill management
bex skills

# Install skills
bex skills install

# Update skills
bex skills update

# List installed skills
bex skills list
```

## Non-Interactive Patterns

Always include `--confirm` and `-o <format>` for CI/CD:

```bash
# Deploy and check exit code
bex deploys create srv-xxx --wait --confirm -o json
if [ $? -ne 0 ]; then
  echo "Deploy failed"
  exit 1
fi

# Get service URL
bex services -o json --confirm | jq -r '.[] | select(.id == "srv-xxx") | .serviceDetails.url'
```

## Config

```bash
# Config file location
cat ~/.bex/cli.yaml

# Override config path
export BEX_CLI_CONFIG_PATH=/path/to/cli.yaml
```
