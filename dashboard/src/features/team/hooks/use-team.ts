import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  WorkspaceMembersDocument,
  WorkspaceInvitesDocument,
  type WorkspaceMembersQuery,
  type WorkspaceInvitesQuery,
} from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import type {
  MemberView,
  InviteView,
  Role,
  SeatUsage,
} from "@/features/team/types";

function toMembers(
  raw: WorkspaceMembersQuery["workspaceMembers"] | undefined,
): MemberView[] {
  return (raw ?? [])
    .filter((m): m is NonNullable<typeof m> => !!m?.subject)
    .map((m) => ({
      subject: m.subject as string,
      userId: m.userId ?? "",
      email: m.email ?? "",
      role: (m.role as Role) ?? "VIEWER",
      createdAt: m.createdAt,
      mfaEnabled: m.mfaEnabled ?? false,
    }));
}

function toInvites(
  raw: WorkspaceInvitesQuery["workspaceInvites"] | undefined,
): InviteView[] {
  return (raw ?? [])
    .filter((i): i is NonNullable<typeof i> => !!i?.id)
    .map((i) => ({
      id: i.id as string,
      email: i.email ?? "",
      role: (i.role as Role) ?? "VIEWER",
      expiresAt: i.expiresAt,
    }));
}

export interface UseTeamResult {
  members: MemberView[];
  invites: InviteView[];
  /** Seat consumption (w1/m33) — null until the query resolves. */
  seats: SeatUsage | null;
  loading: boolean;
  error: Error | undefined;
  /** Whether the caller may manage members (invites query succeeds only for
   *  admins, so a forbidden invites read is the signal the caller is read-only). */
  canManage: boolean;
  refetch: () => Promise<unknown>;
}

/**
 * Reads a workspace's members and pending invites from bex-api. The members
 * query is viewer-and-up; the invites query is admin-only, so its authorization
 * error (surfaced, not thrown, via `errorPolicy: "all"`) is how the UI learns
 * the caller can only view — it hides the invite/role/remove controls.
 */
export function useTeam(workspaceId: string | null): UseTeamResult {
  const skip = !workspaceId;
  const variables = { workspaceId: workspaceId ?? "" };

  const membersQ = useQuery(WorkspaceMembersDocument, {
    variables,
    skip,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });
  const invitesQ = useQuery(WorkspaceInvitesDocument, {
    variables,
    skip,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });

  const members = useMemo(
    () => toMembers(membersQ.data?.workspaceMembers),
    [membersQ.data],
  );
  const invites = useMemo(
    () => toInvites(invitesQ.data?.workspaceInvites),
    [invitesQ.data],
  );
  // Seat usage rides the members document (one request, both viewer-and-up).
  const rawSeats = membersQ.data?.workspaceSeatUsage;
  const seats = useMemo<SeatUsage | null>(
    () =>
      rawSeats && typeof rawSeats.used === "number"
        ? { used: rawSeats.used, limit: rawSeats.limit ?? 0 }
        : null,
    [rawSeats],
  );

  const canManage = !invitesQ.error && !skip;

  async function refetch() {
    await Promise.all([membersQ.refetch(), invitesQ.refetch()]);
  }

  return {
    members,
    invites,
    seats,
    // A skipped query reports loading:false, so an unresolved workspace would
    // otherwise read as "loaded, zero members" and flash the empty state.
    loading: skip || membersQ.loading,
    error: membersQ.error,
    canManage,
    refetch,
  };
}
