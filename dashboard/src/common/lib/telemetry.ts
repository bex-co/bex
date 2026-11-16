/**
 * No-op-safe event sink (w2/m66): forwards to GA4's `gtag` when the document
 * loaded it (VITE_GA_MEASUREMENT_ID, dashboard/AGENTS.md), else does nothing.
 * Events carry only bounded, PII-free labels — matching the SSH gateway's
 * no-identifying-labels rule (docs/ADR035-ssh.md) — so GA needs no scrub.
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
  if (typeof window === "undefined") return;
  window.gtag?.("event", event, props);
}
