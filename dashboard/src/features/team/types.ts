// bex-native projections of bex-api's workspace membership surface
// (backend/internal/members — w4/m12; docs/ADR012-auth.md role matrix). Roles are
// Render's UPPERCASE enum on the wire; the ladder order matches the FGA model.

export const ROLES = [
  "VIEWER",
  "CONTRIBUTOR",
  "DEVELOPER",
  "ADMIN",
  "BILLING",
] as const;

export type Role = (typeof ROLES)[number];

/** An accepted member. `subject` is the raw identity id — kept as the mutation
 *  key (role change / remove), a bex-native contract distinct from Render's
 *  `userId` surface (docs/render-artifacts/owners-api.md). `userId` is the
 *  opaque own- id and `email` the resolved identity email (w6/m10) — both may
 *  be empty when the identity provider is unwired or the lookup misses.
 *  `mfaEnabled` mirrors Render's per-member otpEnabled (w1/m33) — honest-false
 *  on a lookup miss, like email. */
export interface MemberView {
  subject: string;
  userId: string;
  email: string;
  role: Role;
  createdAt: string | null;
  mfaEnabled: boolean;
  /** False when Kratos lookup misses; true when resolved or identity reader is unwired (w4/070). */
  identityResolved: boolean;
  /** True on the row named by the workspace's owner binding — the member the
   *  server refuses to remove or demote (w5/m101, OWNER_CANNOT_BE_REMOVED). */
  isOwner: boolean;
  /** True on the caller's own row — the server refuses self-removal and self
   *  role change (CANNOT_REMOVE_SELF / CANNOT_CHANGE_OWN_ROLE). Server-derived
   *  so the UI never has to match subjects against session data itself. */
  isSelf: boolean;
}

/** A pending (unaccepted) invite — Render's pendingInvites shape. */
export interface InviteView {
  id: string;
  email: string;
  role: Role;
  expiresAt: string | null;
}

/** Seat consumption — Render's owner.usage.users {used, limit} (w1/m33).
 *  `used` counts accepted members plus outstanding invites (the same formula
 *  the server's invite cap enforces); `limit` 0 means unlimited. */
export interface SeatUsage {
  used: number;
  limit: number;
}
