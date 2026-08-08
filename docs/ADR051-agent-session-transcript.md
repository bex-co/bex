# ADR051 — Agent-session conversation transcript persistence

**Status:** Accepted (2026-08-07); implemented in **w3/m77**, **mechanism revised 2026-08-08 (w3/m77 follow-up) to the log-harvest path** (see Decision). Splits the D10 "headless recorder" decision out of [ADR047](ADR047-cloud-coding-agent-sessions.md) into a dedicated ADR because transcript persistence is a distinct concern from the session lifecycle. ADR047 D3/D9 own the **live-attach** conversation plane (shipped w3/m43); this ADR owns the **durable chat history** for the shipped **fire-and-forget** product, which the live-attach tee never captures.

> **Revision (2026-08-08).** The first w3/m77 implementation had the Completer trigger a gateway `/agent-record` endpoint that made a **separate** network dial to the driver's `:8787/stream`. That path was never verified on prod and prod sessions kept showing "No conversation yet." It is replaced by the **log-harvest** path below: the Completer reads the driver's on-disk transcript log over the **same `pods/exec` boundary it already uses for the status read** — the one proven working in prod (sessions finalize through it). The gateway `/agent-record` endpoint, its `RecordSecret`, and `BEX_AGENT_RECORD_URL` are removed. The live-attach tee (ADR047 D9) is unchanged.

---

## Context

### The product surface

An agent session's conversation — the plans, reasoning, tool calls, terminal output, and diffs the agent streams as it works — is a first-class deliverable, not just a live view. The dashboard's session detail page (`dashboard/src/features/agent-sessions`) renders it from the AI SDK **UI-message stream** via `useChat` with `resume: true`, and ADR047 D9 makes the stream's replay mode "the single history source for terminal and running sessions alike." So a completed session must be able to replay its whole conversation long after its sandbox is gone.

### The gap (found in prod 2026-08-06)

The w3/m43 conversation plane persists a transcript only as a **side effect of a live attach**. The sole writers of `agent_session_transcripts` are the gateway tee on the `GET` splice and the `POST` turn (`lego/backend/internal/sshgateway/agentattach/agentattach.go` — both call `store.AppendAgentSessionTranscript`), and both run only while a browser is attached to a still-running driver.

But the shipped **phase-1 product is fire-and-forget** (ADR047 D8): the driver runs its turn **headless with no client stream** (bex-api sets `BEX_AGENT_EXIT_AFTER_TURN=0` and no attach happens — `lego/backend/internal/agentsessions/service.go`; `runHeadlessTurn`, `lego/agent-image/driver/src/session.ts`), and the Completer tears the sandbox down the moment the turn finishes (`lego/backend/internal/agentsessions/completion.go` → `teardown` → `CancelAgentSessionSandbox`). So unless a viewer happened to hold the conversation page open for the **entire** run, nothing is ever teed; by the time the session is opened it is terminal, the pod (and its in-memory hub) is gone, and `GET /stream` replays an **empty** transcript then `[DONE]` — the dashboard shows **"No conversation yet."** for every completed fire-and-forget session.

ADR047 D3's "the gateway tees the session stream into a transcript" and D9's replay-mode claim both silently assumed a persister the phase-1 path never runs. This ADR specifies that persister.

### What already exists (the fix is mostly wiring)

- **The driver keeps the whole turn available until teardown.** It retains the full in-memory `#history` and serves `GET /stream` on port 8787 for as long as the pod lives (`lego/agent-image/driver/src/stream-hub.ts`; the fire-and-forget failure path in `main.ts` is explicit that the driver MUST stay alive so the Completer can read it). `hub.attach` replays the **entire** history — plus `[DONE]`, since a headless turn closes the hub — to any attacher that arrives before teardown. One attach any time before teardown captures the complete transcript in a single shot.
- **The gateway already holds the exact tee.** `spliceDriverStream` dials `http://<podIP>:8787/stream` and appends every part idempotently by the driver's emission ordinal (`ON CONFLICT (session_id, seq) DO NOTHING`, `store/agentsessions.go`). Gateway→driver:8787 ingress is sanctioned by the cluster-wide `sandbox-agent-driver-ingress` Cilium policy. Nothing in the tee needs a browser except the pod/ns/session identity, which today rides the attach ticket.
- **The Completer already crosses the boundary every tick.** It reads the driver status file over the gateway sandbox-exec boundary (`c.Sandbox.ReadSessionStatus` → `sandbox/service.go` `mintAndDial` → gateway `:8081` exec) under the trusted system subject `system:agent-session-completer`, and it owns the teardown. bex-api never holds `pods/exec`.
- **The store is ready.** `AppendAgentSessionTranscript` (idempotent, seq-keyed), `AgentSessionTranscript` (replay read), and `AgentSessionTranscriptMaxSeq` (resume cursor) all exist; the `GET /stream` replay already serves terminal sessions from them.

The only missing piece is a **non-browser trigger** for the tee.

---

## Decision

**Persistence is decoupled from live attach by a server-side recorder the Completer triggers before teardown.**

### Core path

```mermaid
flowchart TB
  dev@{ shape: tri, label: "developer" }

  subgraph dash["dashboard (TanStack)"]
    chat["useChat (resume: true)"]
  end

  subgraph api["bex-api process"]
    verbs["agent-session verbs (create / attach-ticket)"]
    comp["Completer (background loop, 15s tick)"]
  end

  cpdb[("control-plane Postgres: agent_sessions + agent_session_transcripts")]

  subgraph gw["isolated ssh-gateway — sole session ingress, sole pods/exec holder"]
    exec["exec listener :8081 (pods/exec: reads status.json + session.jsonl)"]
    attach["attach listener :8083 (GET /stream replay + tee)"]
  end

  subgraph sb["&lt;ws&gt;-sandbox namespace — gVisor, default-deny"]
    driver["session driver (writes /var/log/bex-agent/session.jsonl; serves :8787 /stream)"]
    agent["agent binary (ACP server)"]
  end

  dev --> chat
  dev -->|"POST create session"| verbs
  verbs -->|"create sandbox"| driver
  driver -->|"stdio ACP"| agent

  comp -->|"1: poll status via pods/exec"| exec
  exec -->|"cat status.json"| driver
  comp -->|"2: harvest transcript via pods/exec (before teardown)"| exec
  exec -->|"tail session.jsonl"| driver
  comp -->|"3: append parts (seq-seeded, idempotent)"| cpdb
  comp -->|"4: teardown sandbox"| driver

  chat -->|"5a: mint attach ticket"| verbs
  chat -->|"5b: GET /stream + HMAC ticket"| attach
  attach -->|"replay transcript (terminal session)"| cpdb
  verbs --> cpdb
```

### Mechanism (log harvest over the proven exec boundary)

- **The Completer harvests-before-teardown** in `completion.go` `teardown`, immediately before `CancelAgentSessionSandbox`. It reads the driver's per-part log through **`ReadSessionTranscript`** — the exact same `mintAndDial` → gateway `:8081` `pods/exec` path it already uses each tick for `ReadSessionStatus`, just `tail`-ing `/var/log/bex-agent/session.jsonl` instead of `cat`-ing `status.json`. This is the decisive reliability choice: if a session finalizes at all, that exec path is working, so the harvest works too — no separate, unverified gateway→driver `:8787` dial.
- **The driver already produces the source.** It writes every UI-message part to the log via `logPart` (`lego/agent-image/driver/src/session.ts`), redacted, and the log survives scrub (`credentials.ts` replaces credential bytes in place, never deletes). bex-api parses each `{…,"part":{…}}` line and keeps only the `.part` payload — the shape the `GET /stream` replay re-frames for a Vercel AI SDK client.
- **Idempotent + seq-seeded.** The Completer skips a turn already present (`AgentSessionTranscriptTurnRecorded` — so a live-teed turn or a retry is a no-op) and seeds `seq` from `AgentSessionTranscriptMaxSeq`, so a redispatched (steered) turn concatenates onto prior turns instead of colliding. bex-api never gains `pods/exec` — the gateway alone holds it (unchanged ADR035 trust design).
- **Failure is honest, never a hang.** The harvest is best-effort and logged; on any error the session still finalizes and tears down (empty transcript rather than a stranded `running` row). A lost sandbox (`ErrNotFound`) has no log to read — the same constraint the status read already lives with.

### No frontend change

The dashboard already renders terminal-session history through the `GET /stream` replay (`dashboard/src/features/agent-sessions`, `useChat` with `resume: true`); once the Completer populates `agent_session_transcripts`, "No conversation yet." is replaced by the real conversation with **zero client change**.

---

## Alternatives considered

- **Gateway `/agent-record` endpoint that dials the driver's `:8787/stream`** (the original w3/m77 implementation): the Completer POSTed an exec-secret-signed ticket to a new gateway route, which resolved the pod IP and reverse-proxied+teed the driver's live `/stream` replay verbatim. Its appeal was byte-exact parts (composes with the live tee). **Superseded 2026-08-08**: it introduced a second, never-prod-verified network path (gateway→driver:8787 direct dial + a new listener route + `BEX_AGENT_RECORD_URL`), and prod sessions stayed blank. The log-harvest rides the already-working status-read boundary instead. The trade-off accepted: harvested parts are **redacted and re-wrapped** (not the verbatim `data:` bytes ADR047 D3 promises) — fine for terminal replay, where the client just needs the parts.
- **Driver pushes parts to the gateway** (a new driver→gateway write path): rejected — it puts untrusted tenant code on the transcript write path (the store comment deliberately keeps the driver out, `store/agentsessions.go`), and the sandbox's **sole** open in-cluster egress port is the credential broker's `:8082`.
- **Create-time recorder** (record the whole turn live from dispatch): rejected as heavier — a long-lived per-session server-side subscriber with its own reconnection/lifecycle — when a one-shot harvest before teardown suffices because the driver's log is complete by then.

---

## Consequences

- The gateway `/agent-record` route, `agentattach.Server.RecordSecret`, and `BEX_AGENT_RECORD_URL` are removed; capture is a bex-api-only concern riding the existing exec boundary. No new env var and no new gateway listener/route.
- Phase-2 live attach is unaffected: the live tee optimizes real-time viewing; the harvest is the universal backstop; both write the same idempotent `(session_id, seq)` rows, and the turn guard keeps them from double-writing.
- Retention rides the existing audit sweep (`PruneAgentSessionTranscripts`, `BEX_AUDIT_RETENTION_DAYS` lineage) and the `ON DELETE CASCADE` with the session.
- Multi-turn/redispatch sessions concatenate because each turn's harvest seeds from `AgentSessionTranscriptMaxSeq`; the harvest is bounded (`tail -c` + a parts cap) against a pathological driver log.
- Residual limit: capture happens at completion, so a session whose sandbox is lost before teardown (`ErrNotFound`) has no log to harvest — that one failure mode still yields an empty transcript.
