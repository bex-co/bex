/** Normalize older/defensive GraphQL payloads without ever rendering [object Object]. */
export function failureReasonText(value: unknown): string | null {
  if (typeof value === "string") {
    const text = value.trim();
    if (!text || /^\[object [^\]]+\]$/i.test(text)) return null;
    if (text.startsWith("{") && text.endsWith("}")) {
      try {
        const decoded: unknown = JSON.parse(text);
        if (decoded && typeof decoded === "object") {
          return failureReasonText(decoded);
        }
      } catch {
        // It is an ordinary human-readable message that happens to use braces.
      }
    }
    return text;
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const record = value as Record<string, unknown>;
  for (const key of ["message", "reason", "code"] as const) {
    const candidate = record[key];
    if (typeof candidate === "string" && candidate.trim()) {
      return candidate.trim();
    }
  }
  return null;
}

// Generic lifecycle words carry no diagnostic value over the "ended with an
// error" fallback, so they are suppressed. Anything more specific (e.g.
// "sandbox create failed") is a real, non-sensitive reason worth showing.
const GENERIC_STATUSES = new Set([
  "",
  "failed",
  "error",
  "canceled",
  "cancelled",
  "unknown",
]);

/** A descriptive lifecycle status used as the failure detail when the backend
 *  recorded no explicit failureReason (many failures set only `status`). */
export function failureStatusText(status: unknown): string | null {
  if (typeof status !== "string") return null;
  const text = status.trim();
  if (!text || GENERIC_STATUSES.has(text.toLowerCase())) return null;
  return text.charAt(0).toUpperCase() + text.slice(1);
}
