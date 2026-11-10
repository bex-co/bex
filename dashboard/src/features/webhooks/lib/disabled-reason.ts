// bex-api serves `disabledReason` as one of two fixed English sentences
// (lego/backend/internal/webhooks/service.go, worker.go). The dashboard shows
// them in the viewer's language; an unknown future reason stays raw, never
// hidden (w21/022).
const MANUAL = "disabled manually";
const AUTO = "disabled automatically after repeated delivery failures";

/**
 * The reason line to show beside a disabled webhook, or null for none. A manual
 * disable shows none: the user just toggled the switch, which already says so.
 */
export function disabledReasonText(
  reason: string | null | undefined,
  t: (key: "webhooks.disabledReasonAuto") => string,
): string | null {
  const raw = (reason ?? "").trim();
  if (!raw || raw === MANUAL) return null;
  if (raw === AUTO) return t("webhooks.disabledReasonAuto");
  return raw;
}
