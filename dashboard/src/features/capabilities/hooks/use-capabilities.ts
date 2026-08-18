import { useQuery } from "@apollo/client/react";
import { ViewerCapabilitiesDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/**
 * The caller's effective permissions in the active workspace (w9/m84). The
 * dashboard reads these to disable controls the server would refuse, so a member
 * without the permission sees a disabled control with a reason instead of a field
 * that 403s on save (docs/ADR024-members.md § the contributor boundary).
 *
 * Every boolean is **permissive while unknown** — true until the server says
 * false. The server stays authoritative, so a stale/absent read must never block
 * an admin (a false negative is worse than briefly showing a contributor a
 * control that then 403s, which is exactly today's behavior). Once a definitive
 * answer arrives (`loaded`), the booleans are the server's.
 */
export interface Capabilities {
  /** The caller's UPPERCASE workspace role, or null when unresolved. */
  role: string | null;
  canView: boolean;
  canViewLogs: boolean;
  canOperate: boolean;
  canCreate: boolean;
  canViewSensitive: boolean;
  canManageKeys: boolean;
  canManage: boolean;
  canManageBilling: boolean;
  /** The query is in flight. */
  loading: boolean;
  /** A definitive server answer has arrived — gate the "reason" copy on this so a
   *  control is never labeled forbidden before we actually know. */
  loaded: boolean;
}

// Unknown (loading, skipped, errored, or store-off) reads as allowed: the server
// is the real gate, so we only ever DISABLE on a definitive `false`.
const permit = (value: boolean | undefined): boolean => value !== false;

export function useCapabilities(): Capabilities {
  const { currentWorkspaceId } = useWorkspace();
  const { data, loading } = useQuery(ViewerCapabilitiesDocument, {
    variables: { ownerId: currentWorkspaceId },
    // Until a workspace is known there is nothing to scope to; a null ownerId
    // would resolve the caller's default server-side, but we prefer the explicit
    // active workspace so a switch re-scopes the gate.
    skip: !currentWorkspaceId,
    // Partial data on error (errorPolicy "all") keeps the gate permissive rather
    // than throwing; cache-first so a warmed nav renders without a refetch.
    errorPolicy: "all",
    fetchPolicy: "cache-first",
  });
  const c = data?.viewerCapabilities ?? null;
  return {
    role: c?.role ?? null,
    canView: permit(c?.canView),
    canViewLogs: permit(c?.canViewLogs),
    canOperate: permit(c?.canOperate),
    canCreate: permit(c?.canCreate),
    canViewSensitive: permit(c?.canViewSensitive),
    canManageKeys: permit(c?.canManageKeys),
    canManage: permit(c?.canManage),
    canManageBilling: permit(c?.canManageBilling),
    loading,
    loaded: c != null,
  };
}
