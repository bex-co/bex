import { isPrivateService } from "@/features/services/lib/service-type";
import type { ServiceView } from "@/features/services/types";

/**
 * The container port bex routes to and injects as `$PORT` (w4/m121).
 *
 * `DEFAULT_SERVICE_PORT` mirrors `appv1alpha1.DefaultPort`, which is what
 * `normalizeCreateDefaults` picks when a create carries no port — so the wizard
 * pre-filling it changes nothing about what gets created, it only makes the
 * value visible and editable before submit.
 *
 * The range is the backend's 1-65535. Ports below 1024 are bindable, as on
 * Render: every pod lets an unprivileged process bind them (w6/m149).
 */
export const DEFAULT_SERVICE_PORT = 3000;
export const MIN_SERVICE_PORT = 1;
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

/**
 * How a service's open ports read beside its Service Address (w6/m148):
 *
 * - `none` — nothing to add: not a private service, the ports were not read, or
 *   the one open port IS the primary (a configured-port service keeps its
 *   familiar `<slug>:<port>` look);
 * - `pending` — an image-port private service whose image has not been read
 *   yet (`openPorts: []`), so no port is known and none is invented;
 * - `list` — the image's declared TCP ports in bex-api's order, the primary
 *   (`port`, the `<slug>:<port>` suffix) flagged.
 */
/** The service facts the open-ports helpers read. */
export type PortFacts = Pick<ServiceView, "type" | "port" | "openPorts">;

export type OpenPortsDisplay =
  | { kind: "none" }
  | { kind: "pending" }
  | { kind: "list"; ports: { port: number; primary: boolean }[] };

export function openPortsDisplay(s: PortFacts): OpenPortsDisplay {
  if (!isPrivateService(s) || s.openPorts == null) return { kind: "none" };
  if (s.openPorts.length === 0) return { kind: "pending" };
  const ports = s.openPorts.map(({ port }) => ({
    port,
    primary: port === s.port,
  }));
  if (ports.length === 1 && ports[0].primary) return { kind: "none" };
  return { kind: "list", ports };
}

/**
 * True when a private service's open ports come from its image (w6/m148):
 * discovery is pending, or the image publishes ports besides the primary. A
 * single-port image reads exactly like a configured port — and has no other
 * declared port to explain — so it keeps the configured-port copy.
 */
export function publishesImagePorts(s: PortFacts): boolean {
  return openPortsDisplay(s).kind !== "none";
}
