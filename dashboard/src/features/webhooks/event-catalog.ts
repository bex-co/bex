// Presentation metadata for the outbound-webhook event vocabulary (w1/m49).
// The vocabulary itself is SERVED by bex-api (`webhookEventTypes`) and never
// hardcoded; this catalog only maps each served key to a human label and a
// Render-shaped group for the picker (docs/render-artifacts/webhooks-ui.md).
// Grouping/labels are presentation, so they live dashboard-side per ADR006's
// thin-adapter rule — a key the backend serves that this catalog doesn't know
// yet degrades to its raw key under the "Other" group, never hidden or thrown.

/** i18n suffixes: group `webhooks.group.<key>`, event `webhooks.event.<type>`. */
const GROUPS: Record<string, string[]> = {
  cronJobRun: ["cron_job_run_started", "cron_job_run_ended"],
  deploy: [
    "build_started",
    "build_ended",
    "deploy_started",
    "deploy_ended",
    "image_pull_failed",
    "pre_deploy_started",
    "pre_deploy_ended",
    "commit_ignored",
  ],
  autoDeploy: ["auto_deploy_enabled", "auto_deploy_disabled"],
  serviceAvailability: [
    "server_failed",
    "server_available",
    "server_restarted",
  ],
  scaling: [
    "autoscaling_started",
    "autoscaling_ended",
    "autoscaling_config_changed",
    "instance_count_changed",
    "plan_changed",
    "branch_changed",
  ],
  maintenanceMode: ["maintenance_mode_enabled", "maintenance_mode_uri_updated"],
  postgres: [
    "postgres_created",
    "postgres_restarted",
    "postgres_credentials_created",
    "postgres_credentials_deleted",
    "postgres_backup_started",
  ],
  suspension: ["service_suspended", "service_resumed"],
};

/** Keys Render lists as bare rows (no group); anything else unknown → "other". */
const SINGLES = new Set<string>(["branch_deleted", "job_run_ended"]);

const EVENT_GROUP: ReadonlyMap<string, string> = new Map(
  Object.entries(GROUPS).flatMap(([group, events]) =>
    events.map((eventType): [string, string] => [eventType, group]),
  ),
);

const KNOWN = new Set([...EVENT_GROUP.keys(), ...SINGLES]);

/** Human-label translation key for a known event; unknown future keys remain raw. */
export function eventLabelKey(type: string): string | null {
  return KNOWN.has(type) ? `webhooks.event.${type}` : null;
}

export interface CatalogEvent {
  type: string;
  /** i18n key for the human label, or null for unknown keys (render `type` raw). */
  labelKey: string | null;
}

export interface CatalogEntry {
  /** null = a bare single row; otherwise the group's i18n suffix. */
  groupKey: string | null;
  events: CatalogEvent[];
}

function toEvent(type: string): CatalogEvent {
  return {
    type,
    labelKey: eventLabelKey(type),
  };
}

/**
 * Arrange the served vocabulary into the picker's tree: known groups (only
 * those with at least one served event), known singles, then an "other"
 * group for anything the catalog hasn't met. Entry order and in-group order
 * follow the catalog, not the server, so the tree is stable as the
 * vocabulary grows; unknown keys sort alphabetically.
 */
export function catalogEntries(served: string[]): CatalogEntry[] {
  const servedSet = new Set(served);
  const entries: CatalogEntry[] = [];

  for (const [groupKey, events] of Object.entries(GROUPS)) {
    const present = events.filter((e) => servedSet.has(e));
    if (present.length > 0) {
      entries.push({ groupKey, events: present.map(toEvent) });
    }
  }
  for (const single of [...SINGLES].sort()) {
    if (servedSet.has(single)) {
      entries.push({ groupKey: null, events: [toEvent(single)] });
    }
  }
  const unknown = served.filter((e) => !KNOWN.has(e)).sort();
  if (unknown.length > 0) {
    entries.push({ groupKey: "other", events: unknown.map(toEvent) });
  }
  return entries;
}
