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
 *  be empty when the identity provider is unwired or the lookup misses. */
export interface MemberView {
  subject: string;
  userId: string;
  email: string;
  role: Role;
  createdAt: string | null;
}

/** A pending (unaccepted) invite — Render's pendingInvites shape. */
export interface InviteView {
  id: string;
  email: string;
  role: Role;
  expiresAt: string | null;
}

/** A workspace the caller belongs to, with their role in it. */
export interface WorkspaceView {
  id: string;
  name: string;
  plan: string | null;
  role: Role | null;
}
