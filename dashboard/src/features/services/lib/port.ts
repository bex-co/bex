/**
 * The container port bex routes to and injects as `$PORT` (w4/m121).
 *
 * `DEFAULT_SERVICE_PORT` mirrors `appv1alpha1.DefaultPort`, which is what
 * `normalizeCreateDefaults` picks when a create carries no port — so the wizard
 * pre-filling it changes nothing about what gets created, it only makes the
 * value visible and editable before submit.
 *
 * The range is narrower than the backend's 1-65535 on purpose: the container
 * runs unprivileged, so it cannot bind a port below 1024 — the same claim the
 * image hint has always made (`services.createImagePortHint`). Offering 80 in
 * the dashboard would be offering a service that can never come up.
 */
export const DEFAULT_SERVICE_PORT = 3000;
export const MIN_SERVICE_PORT = 1024;
export const MAX_SERVICE_PORT = 65535;

/**
 * Parse a port draft, returning null for anything that is not a whole number in
 * range. The caller turns null into its own inline message; the server stays
 * the authority (it refuses out-of-range with "port must be 1-65535").
 */
export function parsePort(draft: string): number | null {
  const trimmed = draft.trim();
  if (!/^\d+$/.test(trimmed)) return null;
  const value = Number(trimmed);
  if (value < MIN_SERVICE_PORT || value > MAX_SERVICE_PORT) return null;
  return value;
}
