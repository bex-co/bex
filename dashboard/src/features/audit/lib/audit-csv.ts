import { config } from "@/config/config";

// Render's audit-log export (render.com/docs/audit-logs): a workspace admin
// picks a start and end date, at most 90 days apart, and downloads a CSV of
// timestamp, actor, event, status and metadata (w3/m305). Its rows are the
// REST list's own (`GET /v1/owners/{ownerId}/audit-logs`, Render's shape), so
// the status column is Render's success|error and the metadata is Render's
// string map: a denial is `error` with `metadata.outcome: "denied"`.

/** Render's longest export range, in days. */
export const MAX_EXPORT_DAYS = 90;

/** The REST list's per-call cap (internal/audit, Render's audit-specific 1000). */
const EXPORT_PAGE_SIZE = 1000;

/** Pages read before an export gives up and asks for a shorter range. */
const MAX_EXPORT_PAGES = 20;

export class AuditExportTooLargeError extends Error {
  constructor() {
    super("audit export exceeds the row budget");
    this.name = "AuditExportTooLargeError";
  }
}

/** Whole days from start to end, both "YYYY-MM-DD" and inclusive. */
export function exportRangeDays(start: string, end: string): number {
  const ms = Date.parse(`${end}T00:00:00Z`) - Date.parse(`${start}T00:00:00Z`);
  return Math.round(ms / 86_400_000) + 1;
}

/** The UTC instants an inclusive "YYYY-MM-DD" range covers. */
export function exportRangeTimes(
  start: string,
  end: string,
): { startTime: string; endTime: string } {
  // The store compares microseconds, so the last second is covered whole.
  return {
    startTime: `${start}T00:00:00Z`,
    endTime: `${end}T23:59:59.999999Z`,
  };
}

/** One row of Render's audit-log list, as bex-api's REST answers it. */
export interface RenderAuditLog {
  timestamp: string;
  event: string;
  status: string;
  actor: { type: string; id?: string; email?: string };
  metadata: Record<string, string>;
  /** bex-api's extra beside Render's actor: the person has deleted their account. */
  actorDeleted?: boolean;
}

/** A workspace's audit-log list, Render's `list-owner-audit-logs`. */
export function ownerAuditLogsPath(ownerId: string): string {
  return `/v1/owners/${encodeURIComponent(ownerId)}/audit-logs`;
}

/**
 * Every row inside the range of the audit-log list at `path` (a workspace's,
 * `ownerAuditLogsPath`, or an organization's, w7/m464), newest first, from
 * Render's REST list paged by its cursor: both answer `auditLogWithCursor`
 * rows to the same parameters. The session cookie authorizes it, as it does
 * GraphQL.
 */
export async function fetchAuditLogRange(
  path: string,
  start: string,
  end: string,
  signal?: AbortSignal,
): Promise<RenderAuditLog[]> {
  const { startTime, endTime } = exportRangeTimes(start, end);
  const logs: RenderAuditLog[] = [];
  let cursor: string | undefined;
  for (let page = 0; page < MAX_EXPORT_PAGES; page++) {
    const query = new URLSearchParams({
      startTime,
      endTime,
      limit: String(EXPORT_PAGE_SIZE),
    });
    if (cursor) query.set("cursor", cursor);
    const response = await fetch(`${config.apiBaseUrl}${path}?${query}`, {
      credentials: "include",
      headers: { Accept: "application/json" },
      // An export is read once; it must not fill the HTTP cache either.
      cache: "no-store",
      signal,
    });
    if (!response.ok) {
      throw new Error(`audit export read failed (${response.status})`);
    }
    const batch = (await response.json()) as {
      cursor: string;
      auditLog: RenderAuditLog;
    }[];
    signal?.throwIfAborted();
    logs.push(...batch.map((item) => item.auditLog));
    if (batch.length < EXPORT_PAGE_SIZE) return logs;
    cursor = batch[batch.length - 1].cursor;
  }
  throw new AuditExportTooLargeError();
}

/**
 * A spreadsheet treats a cell starting with one of these as a formula; names
 * in the log are tenant-controlled, so such a cell is prefixed to stay text.
 */
const FORMULA_LEAD = /^[=+\-@\t\r]/;

function csvCell(value: string): string {
  const text = FORMULA_LEAD.test(value) ? `'${value}` : value;
  return /[",\n\r]/.test(text) ? `"${text.replaceAll('"', '""')}"` : text;
}

/**
 * Render's actor column: a person's email; a deleted user as Render names one,
 * `Deleted User ID#<id>` (w9/099); else the id REST answers (a person whose
 * email could not be read, an API key's client id); a system row has neither.
 */
function csvActor(log: RenderAuditLog): string {
  if (log.actor.email) return log.actor.email;
  if (log.actorDeleted && log.actor.id)
    return `Deleted User ID#${log.actor.id}`;
  return log.actor.id || "";
}

/** Render's export columns: timestamp, actor, event, status, metadata. */
export function auditLogsToCSV(logs: RenderAuditLog[]): string {
  const rows = [["timestamp", "actor", "event", "status", "metadata"]];
  for (const log of logs) {
    rows.push([
      log.timestamp,
      csvActor(log),
      log.event,
      log.status,
      JSON.stringify(log.metadata ?? {}),
    ]);
  }
  return rows.map((row) => row.map(csvCell).join(",")).join("\r\n") + "\r\n";
}
