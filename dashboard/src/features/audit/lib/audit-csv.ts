import type { ApolloClient } from "@apollo/client";
import { AuditLogsDocument } from "@/graphql/definitions";
import type { AuditEvent } from "@/features/audit/types";
import { toAuditEvents } from "@/features/audit/lib/audit-events";

// Render's audit-log export (render.com/docs/audit-logs): a workspace admin
// picks a start and end date, at most 90 days apart, and downloads a CSV of
// timestamp, actor, event, status and metadata (w3/m305).

/** Render's longest export range, in days. */
export const MAX_EXPORT_DAYS = 90;

/** auditLogs' per-call cap (internal/audit, Render's audit-specific 1000). */
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

/**
 * Every event of ownerId inside the range, newest first, paged past the last
 * id as the table's own "Load more" does.
 */
export async function fetchAuditRange(
  client: ApolloClient,
  ownerId: string,
  start: string,
  end: string,
  signal?: AbortSignal,
): Promise<AuditEvent[]> {
  const { startTime, endTime } = exportRangeTimes(start, end);
  const events: AuditEvent[] = [];
  let cursor: string | undefined;
  for (let page = 0; page < MAX_EXPORT_PAGES; page++) {
    const { data } = await client.query({
      query: AuditLogsDocument,
      variables: {
        ownerId,
        startTime,
        endTime,
        cursor,
        limit: EXPORT_PAGE_SIZE,
      },
      // An export is read once; it must not fill the cache with every row.
      fetchPolicy: "no-cache",
    });
    signal?.throwIfAborted();
    const batch = toAuditEvents(data?.auditLogs);
    events.push(...batch);
    if (batch.length < EXPORT_PAGE_SIZE) return events;
    cursor = batch[batch.length - 1].id;
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

/** The event's fields beyond Render's four columns, as one JSON object. */
function metadata(event: AuditEvent): string {
  const entries: [string, string | string[]][] = [
    ["resource", event.resource],
    ["targetName", event.targetName],
    ["from", event.displayNameFrom],
    ["to", event.displayNameTo],
    ["domain", event.domain],
    ["envGroupId", event.envGroupId],
    ["envGroupName", event.envGroupName],
    ["actorMethod", event.actorMethod],
    ["relation", event.relation],
    ["oauthClientId", event.oauthClientId],
    ["oauthAudience", event.oauthAudience],
    ["oauthScopes", event.oauthScopes],
  ];
  return JSON.stringify(
    Object.fromEntries(entries.filter(([, value]) => value.length > 0)),
  );
}

/** Render's export columns: timestamp, actor, event, status, metadata. */
export function auditEventsToCSV(events: AuditEvent[]): string {
  const rows = [["timestamp", "actor", "event", "status", "metadata"]];
  for (const event of events) {
    rows.push([
      event.timestamp,
      event.actor,
      event.action,
      event.status,
      metadata(event),
    ]);
  }
  return rows.map((row) => row.map(csvCell).join(",")).join("\r\n") + "\r\n";
}
