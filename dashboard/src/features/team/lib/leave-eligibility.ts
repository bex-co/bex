import type { MemberView } from "@/features/team/types";

/** Why the Leave action is unavailable, as a translation key — or null when the
 *  caller may leave. Mirrors the server's refusals (w5/m102): the workspace
 *  owner is stranded by the owner binding, and the last admin would leave a
 *  workspace nobody can administer. */
export type LeaveBlockedReason =
  | "team.leaveOwnerReason"
  | "team.leaveLastAdminReason"
  | null;

export interface LeaveEligibility {
  /** False when the caller has no membership row here — there is nothing to
   *  leave, so the action is not rendered at all (as opposed to refused). */
  isMember: boolean;
  blockedReason: LeaveBlockedReason;
}

/**
 * Decides whether the caller may leave, from the members list they already
 * read. Pure, so the mapping is testable without a DOM or a server.
 *
 * This is a UI pre-check, never the enforcement: bex-api refuses both cases
 * with coded errors regardless of what the page believes (`OWNER_CANNOT_LEAVE`,
 * and the shared last-admin rule). Its job is to avoid offering an action the
 * server would reject — the m50 rendered-but-disabled pattern — and the inline
 * server error remains the backstop for the race where somebody else's role
 * changes while this page is open.
 */
export function leaveEligibility(members: MemberView[]): LeaveEligibility {
  const me = members.find((m) => m.isSelf);
  if (!me) return { isMember: false, blockedReason: null };
  if (me.isOwner) {
    return { isMember: true, blockedReason: "team.leaveOwnerReason" };
  }
  const admins = members.filter((m) => m.role === "ADMIN").length;
  if (me.role === "ADMIN" && admins <= 1) {
    return { isMember: true, blockedReason: "team.leaveLastAdminReason" };
  }
  return { isMember: true, blockedReason: null };
}
