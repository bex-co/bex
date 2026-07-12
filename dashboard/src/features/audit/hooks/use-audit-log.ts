import { useMemo, useState } from "react";
import { useApolloClient, useQuery } from "@apollo/client/react";
import {
  AuditLogsDocument,
  type AuditLogsQuery,
} from "@/graphql/definitions";
import type { AuditEvent } from "@/features/audit/types";

const PAGE_SIZE = 20;

type RawEvent = NonNullable<AuditLogsQuery["auditLogs"]>[number];

function toEvents(raw: AuditLogsQuery["auditLogs"] | undefined): AuditEvent[] {
  return (raw ?? [])
    .filter((e): e is NonNullable<RawEvent> & { id: string } => !!e?.id)
    .map((e) => ({
      id: e.id,
      timestamp: e.timestamp ?? "",
      actor: e.actor ?? "",
      actorMethod: e.actorMethod ?? "",
      action: e.action ?? "",
      status: e.status === "denied" ? "denied" : "success",
      resource: e.resource ?? "",
    }));
}

type ErrorKind = "forbidden" | "unavailable" | "error";

// Resolver errors reach Apollo verbatim (graphql-go has no error-formatting
// layer here) — `core.ErrForbidden`/`core.ErrAuditUnavailable`'s own message
// text, matched the same way api-keys-panel.tsx classifies its own errors.
function classify(error: Error | undefined): ErrorKind | null {
  if (!error) return null;
  const message = error.message.toLowerCase();
  if (message.includes("forbidden")) return "forbidden";
  if (message.includes("audit log store not configured")) return "unavailable";
  return "error";
}

export interface UseAuditLogResult {
  events: AuditEvent[];
  loading: boolean;
  loadingMore: boolean;
  error: Error | undefined;
  forbidden: boolean;
  unavailable: boolean;
  hasMore: boolean;
  loadMore: () => void;
}

/**
 * Reads a workspace's audit trail (`auditLogs`) newest-first, admin-only
 * (RelCanManage). A 403 sets `forbidden` — mirrors use-team.ts's canManage
 * pattern: hide the panel, don't show an error. A 503
 * (`core.ErrAuditUnavailable`, control-plane store not wired) sets
 * `unavailable`, a distinct state from a generic `error`.
 *
 * `auditLogs` is a bare list with no Apollo pagination field policy, so
 * `loadMore` re-queries imperatively past the last-loaded event's id (the
 * surface's keyset cursor, internal/store/audit.go) and appends the result
 * itself rather than relying on cache merging. `hasMore` tracks whether the
 * most recently fetched page (initial or appended) came back full-sized.
 */
export function useAuditLog(workspaceId: string | null): UseAuditLogResult {
  const client = useApolloClient();
  const skip = !workspaceId;

  const { data, loading, error } = useQuery(AuditLogsDocument, {
    variables: { ownerId: workspaceId ?? "", limit: PAGE_SIZE },
    skip,
    fetchPolicy: "network-only",
    errorPolicy: "all",
  });
  const firstPage = useMemo(() => toEvents(data?.auditLogs), [data]);

  const [morePages, setMorePages] = useState<AuditEvent[]>([]);
  const [lastPageSize, setLastPageSize] = useState<number | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [loadMoreError, setLoadMoreError] = useState<Error | undefined>();

  const events = useMemo(() => [...firstPage, ...morePages], [firstPage, morePages]);
  const hasMore = lastPageSize === null ? firstPage.length === PAGE_SIZE : lastPageSize === PAGE_SIZE;

  async function loadMore() {
    if (!workspaceId || loadingMore || events.length === 0) return;
    setLoadingMore(true);
    setLoadMoreError(undefined);
    try {
      const cursor = events[events.length - 1].id;
      const result = await client.query({
        query: AuditLogsDocument,
        variables: { ownerId: workspaceId, cursor, limit: PAGE_SIZE },
        fetchPolicy: "network-only",
        errorPolicy: "all",
      });
      const page = toEvents(result.data?.auditLogs);
      // Defensive only — the exclusive keyset cursor shouldn't ever hand back
      // an id already loaded. Bounded to the current tail, not the full
      // history, so a click's cost doesn't grow as a user pages deeper in.
      const seenTail = new Set(events.slice(-PAGE_SIZE).map((e) => e.id));
      setMorePages((prev) => [...prev, ...page.filter((e) => !seenTail.has(e.id))]);
      setLastPageSize(page.length);
      setLoadMoreError(result.error);
    } catch (err) {
      setLoadMoreError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      setLoadingMore(false);
    }
  }

  const kind = classify(error ?? loadMoreError);

  return {
    events,
    loading: loading && events.length === 0,
    loadingMore,
    error: kind === "error" ? (error ?? loadMoreError) : undefined,
    forbidden: kind === "forbidden",
    unavailable: kind === "unavailable",
    hasMore,
    loadMore: () => void loadMore(),
  };
}
