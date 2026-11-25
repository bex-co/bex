# Private network communication patterns

Architecture examples and implementation notes for bex’s private network. Read [private-networking.md](private-networking.md) first for current platform behavior and limits.

## Gateway pattern

**Flow:** Public **Web Service** → one or more **Private Services** (or other internal targets).

- Expose only the gateway on the public internet.
- From the gateway process, call backends using **internal hostname and port**: `http://[internal-hostname]:[port]/...`
- Keep sensitive APIs and admin surfaces on Private Services; restrict security groups / auth at the gateway and at each service.

## Worker-to-DB

**Flow:** **Background Worker** → **Managed Postgres** (or **Key Value**) via **internal URL**.

- Workers **cannot** receive inbound private connections (no internal hostname); they are always clients.
- Store the datastore **internal URL** from the Dashboard (**Connect > Internal**) in an env var your worker reads at runtime.
- Use connection pooling appropriate for worker concurrency.

## Service mesh (lightweight)

**Flow:** Multiple **Private Services** (and optionally internal ports on Web Services) call each other by **internal hostnames**.

- Standardize on one scheme (`http` vs `https`) per hop; some clients require an explicit `http://` or `https://` prefix.

## URL construction

Use a consistent template:

```text
http://[internal-hostname]:[port]/path
```

- Replace `[internal-hostname]` and `[port]` with values from **Connect > Internal**.
- Add query strings and headers as needed; preserve trailing slashes if your upstream is sensitive to them.

## Blueprint wiring

In `render.yaml`, link services for private access using **`fromService`** on the consumer:

- Use properties such as **`host`** or **`hostport`** when the template needs the private service’s internal hostname or `host:port` (exact key names follow your Blueprint schema version—align with current bex Blueprint docs).

Ensure the producer service type supports private networking and that region/workspace match the consumer.

## Per-instance discovery

Use per-instance discovery only when the application must implement custom instance selection, retries, health aggregation, or per-instance metrics. Prefer the normal internal hostname for ordinary service-to-service calls. Follow [private-networking.md](private-networking.md) for the current discovery hostname and resolver behavior.

## Crossing private-network boundaries

When resources intentionally span regions, workspaces, or isolated project environments, either colocate them or design an explicit supported integration path such as an authenticated public endpoint or data replication. Do not expose an otherwise private service merely as a troubleshooting shortcut.
