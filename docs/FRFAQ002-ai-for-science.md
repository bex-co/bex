# PRFAQ — AI for Science at Bex

**Status:** Draft for review (growth TPM research artifact, 2026-08-20). Not an ADR; nothing here authorizes implementation. All customer quotes are illustrative placeholders pending real customer approval.

**Research base:** the OpenEvolve multi-fidelity Gaussian-process campaign (`openevolve-sandbox` onboarding doc, 2026-08-19 300-iteration reproduction), [jataware/open-coscientist](https://github.com/jataware/open-coscientist), [google-research/era](https://github.com/google-research/era) (arXiv:2509.06503), the [UQ Lab at the University of Houston](https://uq.uh.edu/) as design-partner anchor, and a shipped-vs-proposed audit of bex's own platform (ADR014/042/047/059/062, tiers.yaml, ADR053).

---

## Press Release

### Bex launches AI for Science: research labs run agentic discovery campaigns on managed, metered, isolated infrastructure

_HOUSTON, TX — [target: Q4 2026]_ — Bex, the open-source AI-native cloud, today announced **AI for Science at Bex**, a program that lets small research groups run LLM-driven scientific computing — evolutionary program search, hypothesis-generation pipelines, and AI-written empirical software — without owning a cluster, a queue, or a security review.

A new class of scientific workload has arrived faster than university infrastructure can absorb it. Google's ERA system (published in Nature) showed an LLM plus tree search can write expert-level scientific software — by executing untrusted, machine-generated code thousands of times. OpenEvolve-style campaigns evolve Gaussian-process models by letting Claude mutate Python programs while a fixed evaluator scores them. Co-scientist pipelines run tournaments of LLM agents over the literature for hours. Every one of these systems ships with the same holes: the sandbox is left as an exercise for the reader, state lives in memory until the process dies, API keys sit in plaintext env vars, and nobody can say what a run cost.

Bex already built exactly those missing pieces — for coding agents. AI for Science points them at research:

- **Isolated execution for generated code.** Every candidate program runs in a gVisor-isolated, egress-controlled sandbox in a per-workspace namespace — the concrete implementation of the sandbox interface that research frameworks declare abstract. Sandboxes are driven by REST, CLI, or MCP, so the campaign controller can be an agent.
- **Model keys that never touch untrusted code.** The lab's Anthropic or Gemini key is held in the platform vault; the sandbox sees only a placeholder, and the gateway injects the real key on the upstream hop with per-session request, duration, and concurrency budgets. A mutated candidate program cannot exfiltrate what it never received.
- **Durable, honest campaigns.** Results and checkpoints land in managed Postgres with one-command logical export; idle workspaces hibernate to encrypted object storage and resume in seconds; cron schedules recurring campaign batches; every run is metered to the second and billed at 30% below Render list price. A 49-minute, 300-iteration reference campaign costs cents of compute — the lab's own LLM spend stays the dominant, and now visible, line item.
- **From campaign to community.** The same workspace serves the lab's public face: the reproducibility artifact page as a static site, the digital-twin inference API as an autoscaled service that scales to zero between demos, the paper's companion endpoint deployed from the repo's `render.yaml`.

The design-partner lab is the Uncertainty Quantification Lab at the University of Houston (uq.uh.edu) — five researchers doing Gaussian-process surrogate modeling, Bayesian optimization, and digital twins for infrastructure and energy systems, with no dedicated cluster.

> _"Our loops are hundreds of short model evaluations with idle time between campaigns — the worst possible shape for owning hardware and the best possible shape for scale-to-zero. Bex let a PhD student launch an evolutionary kernel-search campaign in an afternoon, with the API key custody and the bill both handled by the platform."_ — placeholder quote, PI, UQ Lab @ UH (pending approval)

> _"Agent frameworks for science all say 'bring your own sandbox.' We are the sandbox — plus the key vault, the meter, and the checkpoint store."_ — placeholder quote, bex

Labs can start at bex.co with a GitHub sign-in; self-hosting the full open-source platform remains a first-class exit.

---

## External FAQ (customers)

### 1. What is AI for Science at Bex?

A supported pattern — documentation, blueprints, and platform features — for running agentic scientific computing on bex: campaigns where an LLM proposes candidate programs or hypotheses, an evaluator executes and scores them in isolation, and the archive of results is durable, reproducible, and billed transparently. It is not a new product surface; it is bex's existing sandbox, agent-session, data, and metering products composed for research workloads.

### 2. Who is it for?

Small computational-science groups (2–10 people) with no dedicated cluster and no ops staff: UQ and surrogate-modeling labs, computational biology and epidemiology groups, anyone adopting co-scientist or ERA-style tooling. The reference persona is the UQ Lab at UH: Python/PyTorch-first, publishes code openly on GitHub, workloads are embarrassingly parallel batches of moderate jobs, price-sensitive, and currently limited to workstations and shared university machines.

### 3. What reference workloads does it serve, concretely?

| Workload | Shape | What bex supplies |
| --- | --- | --- |
| **OpenEvolve multi-fidelity GP search** (the onboarding doc) | ~300 iterations × (LLM mutation → sandboxed fit/score), 4 islands, MAP-Elites archive, ~49 min, ~$40 API cap | gVisor sandbox replaces the DIY Docker rig; the credential-injecting model proxy replaces LiteLLM key handling; Postgres + export holds the archive/checkpoints; usage meter |
| **open-coscientist** (Jataware) | Long-running LangGraph tournament of 8–10 agents, in-memory state, sidecar literature MCP server | Hosted MCP sidecar as a bex service with secret injection; Postgres as the missing result store; the run itself as a sandboxed process |
| **ERA / FUTS** (Google Research, Nature) | LLM rewrites `train_and_predict`, executes it against real data, scores, expands best node — serial loop | `sandbox.py` is an abstract class in the repo; bex sandboxes are its concrete implementation, with timeouts, egress control, and per-candidate isolation |
| **Digital-twin / surrogate serving** (UQ Lab thrust 1) | Lightweight always-on inference API fed by monitoring data | Autoscaled web service (scale-to-zero), cron ingestion jobs, custom domains, managed Postgres |
| **Reproducibility artifacts** (onboarding doc §13) | Dated artifact bundles: config, best program, metrics, figures, SHA-256 | Static sites for the artifact page; Postgres export for the archive; git-based deploys for per-paper environments |

### 4. How does a campaign actually run on bex today?

The campaign controller (a script, or an agent connected over MCP per ADR025) creates sandboxes via `/v1/sandboxes*`, execs candidate evaluations inside them, and writes scores and the archive to a managed Postgres. Recurring batches schedule as `cron_job` services. Model calls route through the mandatory credential-injecting proxy (ADR062): the key lives in OpenBao, the sandbox holds a placeholder, and the gateway enforces per-session/per-workspace request counts, a session duration cap, and connection limits. Idle agent workspaces hibernate to ADR050-encrypted object storage and resume with p50 under 5 seconds. Because sandbox rootfs is not durable storage, checkpoints must be externalized each round — to Postgres, to git, or to the lab's own S3 (see FAQ 7).

### 5. What does it cost?

Compute is metered per second (`sandbox_compute_seconds`, `instance_seconds`) and priced 30% below Render, with 90%-off bandwidth (ADR030); an advisory `estimatedCost` rides every usage response, and invoices come from Stripe (ADR040). The compute for the 49-minute reference campaign is well under a dollar; the lab's LLM API spend (bounded around $40 in the reference) dominates and stays on the lab's own provider account via BYO keys. Hosted bex requires a bound payment method for all usage (ADR075); academic credit grants are a program lever under the ADR071 credits design (Proposed).

### 6. Do you have GPUs?

**No — not today.** Bex nodes are shared-vCPU x86 (Hetzner, cx33 default; the largest schedulable node is 8 vCPU / 16 GB), and there is no GPU pool, runtime, or tier anywhere in the platform. AI for Science v1 explicitly targets the CPU-bound majority of agentic-science workloads — LLM-orchestration campaigns whose heavy lifting is remote API inference, GP/statistics fitting, and classical simulation at moderate scale. Episodic GPU training (PINNs, neural operators) stays on the lab's university or colab resources for now; GPU support is a roadmap question, not a promise (see internal FAQ).

### 7. Where do large artifacts and datasets live?

Managed Postgres (with logical export to a downloadable archive) is the durable result store; shipped plans are small (up to 1 Gi RAM / 5 GB storage today), sized for archives and metrics rather than bulk data. Bex does not yet offer tenant object-storage buckets — for multi-GB checkpoints and datasets, labs bring their own S3-compatible bucket and inject the credential as an env-group secret. This is a known gap on the internal register.

### 8. Why should a lab trust the isolation?

Candidate programs are untrusted by construction — an LLM wrote them. Each runs under gVisor in a per-workspace sandbox namespace with default-deny egress (DNS-L7 allowlists), on a dedicated tainted node pool, with exec brokered through an isolated gateway using single-use tickets. The platform carries eighteen published rounds of adversarial security review (ADR028 → ADR079). One honest boundary statement: gVisor is a strong syscall-filtering boundary, not a microVM; the threat model and its residuals are public in ADR042.

### 9. Does bex make my science correct?

No, and the program will not pretend otherwise. The reference onboarding doc's own conclusion — "engineering 'ran to completion' is not scientific 'conclusion proven'" — is the program's editorial line. Bex supplies provenance-friendly mechanics (durable archives, dated artifacts, metered stop reasons, reproducible environments); evaluation design, untouched test sets, and honest claims remain the researcher's job.

---

## Internal FAQ (bex)

### 10. Why this segment? (growth thesis)

- **Workload–product fit is unusually exact.** Agentic science is sandbox-per-candidate execution plus BYO-key LLM calls plus durable archives — bex's three most differentiated shipped primitives (ADR042 sandboxes, ADR062 key proxy, managed Postgres + metering). Render, our compatibility target, has none of the first two.
- **The frameworks advertise our gap.** ERA ships `sandbox.py` as an abstract base class; open-coscientist ships no persistence and no cost metering. Every lab that adopts these tools inherits our product's problem statement verbatim.
- **Small labs are underserved and sticky.** University HPC queues don't run long-lived services, hold API keys, or bill per second; hyperscalers are too much surface for a 5-person group. Publication footprints ("experiments were run on bex") are compounding, citable distribution.
- **It monetizes the sandbox meter.** `sandbox_compute_seconds` is shipped and Stripe-wired; science campaigns are steady, bursty, scale-to-zero-friendly consumption — the exact usage shape our pricing (30% off Render, 90% off egress) wins on.
- **Warm path to industry.** The reference campaign is a Chevron case study and the anchor lab holds a Chevron Fellowship; academic design partners are the low-cost route into energy-sector UQ workloads.

### 11. What ships today vs. what's missing? (gap register)

Works today, end to end: gVisor sandboxes over REST/CLI/MCP; mandatory credential-injecting model proxy with request/duration/concurrency budgets; cron jobs; managed Postgres/KeyValue with export; encrypted workspace hibernation; full metering + Stripe; blueprints; deploy-from-chat.

| # | Gap | Severity for this segment | Disposition |
| --- | --- | --- | --- |
| 1 | **No GPUs/accelerators** (no pool, runtime, or tier; ADR003 mentions the concept only) | Disqualifying for training-heavy labs; OK for v1 | Scope v1 to CPU-bound campaigns; treat GPU pool as a separate, user-decided investment |
| 2 | Compute ceiling ~8 vCPU / 16 GB real node (cx43; cx53 uncreatable in fsn1; 32 Gi tier unschedulable) | Medium | Document honestly; revisit with provider stock (ADR053 watch) |
| 3 | **No batch-queue / job-fleet product**; one-off jobs off-roadmap (`DO_NOT_DO`) | Medium — campaigns self-orchestrate | v1: publish a campaign-controller blueprint over sandboxes + cron; do not build a queue product yet |
| 4 | **No tenant object-storage buckets** (platform buckets internal-only) | High for datasets/checkpoints | v1: document BYO-S3 via env groups; candidate roadmap item |
| 5 | No persistent sandbox volumes; rootfs-only pause (no CRIU memory snapshots) | Medium | v1: checkpoint-out-of-band pattern (already the onboarding doc's own practice) |
| 6 | Model-proxy budgets are request-count/duration, **not token-dollar caps** (ADR062 phase 2 Proposed) | Medium — labs want "$40 cap" semantics | Prioritize token metering at the proxy seam; interim: provider-side spend caps + BYO key |
| 7 | Managed Postgres shipped tiers are small (≤1 Gi / 5 GB) | Low–medium | Fine for archives/metrics; larger tiers already designed, deferred |
| 8 | Card-required-for-all-usage (ADR075) vs. academic procurement | Medium — onboarding friction | Academic credit grants via ADR071 (Proposed) as the program lever |

### 12. What is the minimum launchable package?

No new product surface. Four items: (1) a **campaign blueprint** — a worked, checked-in example running the OpenEvolve reference campaign on bex sandboxes with Postgres archiving and a BYO-S3 artifact step; (2) **docs** for the BYO-S3 pattern and the honest compute/GPU boundary; (3) **token-dollar budgets** at the model-proxy seam (the already-proposed ADR062 phase 2 — the single most-requested guarantee in this segment); (4) an **academic credits** motion once ADR071 lands. Items 1–2 are documentation-weight; only 3–4 touch code, and both are already on existing ADRs.

### 13. Any licensing or positioning constraints?

open-coscientist is MIT **+ Commons Clause** — bex must not sell hosted open-coscientist itself; the compliant motion is customers running their own instances on bex (compute, not software, is what we sell), and marketing copy must not imply a bex-managed co-scientist service. ERA is Apache-2.0 (unofficial Google release) — safe to reference and blueprint. The reference campaign's own doc warns against overclaiming synthetic-benchmark results as real discoveries; program marketing inherits that restraint (see FAQ 9) — credibility with researchers is the asset.

### 14. How do we measure success?

Primary: `sandbox_compute_seconds` and instance-seconds attributable to science workspaces; campaigns completed (sandbox sessions with ≥N execs and an exported archive); labs activated (verify → card/credit → first campaign, per ADR075's activation ladder). Secondary: MCP-driven sandbox creates (agent-run campaigns), papers/repos citing bex, conversion of academic credits to paid usage, and design-partner retention (UQ Lab running ≥1 campaign/month by two quarters post-launch).

### 15. What are the top risks?

(1) **GPU absence caps the segment** — mitigated by scoping to CPU-bound agentic campaigns, but a competitor with cheap GPU sandboxes could own the broader story. (2) **Expectation mismatch on budgets** until token-dollar caps ship — mitigated by explicit interim docs. (3) **Academic payment friction** against the card-for-all-usage gate — mitigated only when ADR071 credits land; sequencing matters. (4) **Reproducibility optics**: if we market "science on bex" and a flagship campaign fails to replicate (as the 2026-08-19 reference reproduction did), the honest-mechanics framing above is the defense — bex sells provenance, not conclusions.
