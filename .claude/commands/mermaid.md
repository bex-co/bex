---
description: Draw a concise, syntax-verified mermaid architecture diagram from a description
argument-hint: "<what to diagram — a description, or a repo component/doc>"
allowed-tools: Bash(npx -y @mermaid-js/mermaid-cli:*), Write, Read
---

Draw a mermaid architecture diagram in markdown for: $ARGUMENTS

## Conventions

- **Boxes are services/components**: `svc[bex-api]`. Use `[(...)]` for datastores. Humans (user, developer, operator) are triangles: `user@{ shape: tri, label: "user" }`.
- **A box that is not a long-running service must say so in its label** — readers assume boxes are services and ask "where is this running?". Mark scheduled/ephemeral work (`cron["backup pod (spawned nightly, exits when done)"]`) and inert config objects (`secret["Secret foo (k8s object, created once)"]`). Draw humans as triangles: `operator@{ shape: tri, label: "operator" }`. Never draw a manual procedure as a peer box of running infrastructure: give runbook/recovery flows their own subgraph whose title says it's manual and where it runs (`subgraph "disaster recovery — manual runbook, any docker host"`), with the human actor inside.
- **Arrows are dependency direction**: `A --> B` means A depends on (calls, reads, deploys to) B — never the reverse.
- **Concise but to the point**: only load-bearing services and edges. No styling, no colors, no legend. Label an edge (`A -->|gRPC| B`) only when the relationship isn't obvious. Default to `flowchart TB`; use `LR` only if the graph is much wider than deep. Use `subgraph` only for real boundaries (cluster, node, network, trust zone, automated vs. manual) — subgraphs are how the diagram answers "where does this run?". An edge may target a whole subgraph by id (`subgraph cluster["app cluster"]` … `op --> cluster`).
- If $ARGUMENTS refers to this repo, read the relevant docs/code first (`docs/ADR002-architecture.md` is the map) — don't diagram from guesswork.

Syntax gotchas that break rendering: quote labels containing `(`, `)`, `[`, `{`, or `-->`-like text (`a["Queue (SQS)"]`); never name a node bare `end` or `graph`; subgraph titles with spaces need quotes.

## Verify (mandatory, before answering)

1. Write the diagram body (no ` ```mermaid ` fence) to a `.mmd` file in the scratchpad.
2. Run: `npx -y @mermaid-js/mermaid-cli -i <file>.mmd -o <file>.svg` — exit 0 means the syntax is valid. (First run downloads a headless browser; that's expected.)
3. On failure, read the parse error, fix the diagram, and re-verify. Never output a diagram that hasn't passed.

## Output

A single ` ```mermaid ` fenced block, followed by at most 2 sentences explaining the key dependency flow. If the user asked to put the diagram into a file, insert the verified block there instead.
