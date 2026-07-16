import { useMemo, useState } from "react";
import { useApolloClient, useQuery } from "@apollo/client/react";
import { AuditLogsDocument, type AuditLogsQuery } from "@/graphql/definitions";
import type { AuditEvent } from "@/features/audit/types";
import { useWorkspace } from "@/features/workspaces/context/hooks";

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
      targetName: e.targetName ?? "",
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

/** The `loadMore`-appended pages, tagged with the workspace they were read for. */
interface AppendedPages {
  workspaceId: string | null;
  events: AuditEvent[];
  /** Size of the most recently appended page (null ⇒ none appended yet). */
  lastPageSize: number | null;
  error: Error | undefined;
}

const NO_APPENDED: AppendedPages = {
  workspaceId: null,
  events: [],
  lastPageSize: null,
  error: undefined,
};

/**
 * Reads the *switcher's currently-selected* workspace's audit trail
 * (`auditLogs`) newest-first, admin-only (RelCanManage). A 403 sets `forbidden`
 * — mirrors use-team.ts's canManage pattern: hide the panel, don't show an
 * error. A 503 (`core.ErrAuditUnavailable`, control-plane store not wired) sets
 * `unavailable`, a distinct state from a generic `error`.
 *
 * Workspace-scoped exactly like useServices/useDatabases (w6/m14 t006): the
 * owner comes from WorkspaceProvider, not from a private `workspaces` query —
 * an earlier version resolved it via `useCurrentWorkspace()`, which always
 * returns `workspaces[0]` (the account's original auto-provisioned workspace),
 * so the table stayed pinned to that workspace no matter what the switcher
 * said. Skipped until the selection resolves, so a still-null owner never fires
 * the fetch-then-refetch pair; `loading` stays true for that window.
 *
 * `auditLogs` is a bare list with no Apollo pagination field policy, so
 * `loadMore` re-queries imperatively past the last-loaded event's id (the
 * surface's keyset cursor, internal/store/audit.go) and appends the result
 * itself rather than relying on cache merging. `hasMore` tracks whether the
 * most recently fetched page (initial or appended) came back full-sized. Those
 * appended pages carry the workspace they were read for, so switching drops
 * them (and any in-flight page that lands late) instead of stacking one
 * workspace's history under another's first page.
 */
export function useAuditLog(): UseAuditLogResult {
  const client = useApolloClient();
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;

  const { data, loading, error } = useQuery(AuditLogsDocument, {
    variables: { ownerId: currentWorkspaceId ?? "", limit: PAGE_SIZE },
    skip: !resolved,
    fetchPolicy: "network-only",
    errorPolicy: "all",
  });
  const firstPage = useMemo(() => toEvents(data?.auditLogs), [data]);

  const [appended, setAppended] = useState<AppendedPages>(NO_APPENDED);
  const [loadingMore, setLoadingMore] = useState(false);

  // Derived, not reset in an effect: pages read for another workspace are
  // ignored the instant the switcher moves, with no extra render pass showing
  // the previous workspace's tail.
  const scoped =
    appended.workspaceId === currentWorkspaceId ? appended : NO_APPENDED;

  const events = useMemo(
    () => [...firstPage, ...scoped.events],
    [firstPage, scoped.events],
  );
  const hasMore =
    scoped.lastPageSize === null
      ? firstPage.length === PAGE_SIZE
      : scoped.lastPageSize === PAGE_SIZE;

  async function loadMore() {
    if (!currentWorkspaceId || loadingMore || events.length === 0) return;
    const ownerId = currentWorkspaceId;
    setLoadingMore(true);
    setAppended((prev) =>
      prev.workspaceId === ownerId ? { ...prev, error: undefined } : prev,
    );
    try {
      const cursor = events[events.length - 1].id;
      const result = await client.query({
        query: AuditLogsDocument,
        variables: { ownerId, cursor, limit: PAGE_SIZE },
        fetchPolicy: "network-only",
        errorPolicy: "all",
      });
      const page = toEvents(result.data?.auditLogs);
      // Defensive only — the exclusive keyset cursor shouldn't ever hand back
      // an id already loaded. Bounded to the current tail, not the full
      // history, so a click's cost doesn't grow as a user pages deeper in.
      const seenTail = new Set(events.slice(-PAGE_SIZE).map((e) => e.id));
      setAppended((prev) => {
        const base = prev.workspaceId === ownerId ? prev.events : [];
        return {
          workspaceId: ownerId,
          events: [...base, ...page.filter((e) => !seenTail.has(e.id))],
          lastPageSize: page.length,
          error: result.error,
        };
      });
    } catch (err) {
      const failure = err instanceof Error ? err : new Error(String(err));
      setAppended((prev) => ({
        ...(prev.workspaceId === ownerId ? prev : NO_APPENDED),
        workspaceId: ownerId,
        error: failure,
      }));
    } finally {
      setLoadingMore(false);
    }
  }

  const kind = classify(error ?? scoped.error);

  return {
    events,
    loading: !resolved || (loading && events.length === 0),
    loadingMore,
    error: kind === "error" ? (error ?? scoped.error) : undefined,
    forbidden: kind === "forbidden",
    unavailable: kind === "unavailable",
    hasMore,
    loadMore: () => void loadMore(),
  };
}
