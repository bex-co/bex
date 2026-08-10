/**
 * Minimal, no-op-safe event sink (w2/m66). The dashboard has no analytics
 * pipeline yet; this is the single seam a future one wires into. Events carry
 * only bounded, PII-free labels — matching the SSH gateway's no-identifying-
 * labels rule (docs/ADR035-ssh.md) — so adding a real backend later needs no
 * scrub. Until then `trackEvent` is intentionally a no-op.
 *
 * The activation funnel these events describe has a server-side counterpart the
 * gate can't see from the browser (whether the eventual SSH auth succeeds): the
 * gateway's `bex_ssh_gateway_authentications_total{result="rejected_key"}` vs
 * `{result="accepted"}` — the metric that root-caused the w2/m65 dead-end. "The
 * gate is working" looks like `rejected_key`'s share falling as `accepted` rises.
 */
export type TelemetryEvent = "ssh_gate_shown" | "ssh_gate_cta_clicked";

export function trackEvent(
  event: TelemetryEvent,
  props?: Record<string, string>,
): void {
  // No-op sink. Wire a real telemetry backend here; callers already pass only
  // bounded, PII-free labels, so no scrubbing is needed at that point.
  void event;
  void props;
}
