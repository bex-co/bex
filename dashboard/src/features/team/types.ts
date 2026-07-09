// bex-native projections of bex-api's workspace membership surface
// (backend/internal/members — w4/m12; docs/auth.md role matrix). Roles are
// Render's UPPERCASE enum on the wire; the ladder order matches the FGA model.

export const ROLES = [
  "VIEWER",
  "CONTRIBUTOR",
  "DEVELOPER",
  "ADMIN",
  "BILLING",
] as const;

export type Role = (typeof ROLES)[number];

/** An accepted member. bex keys membership by identity subject (no per-member
 *  email store yet), so `subject` is what we show — Render shows name+email. */
export interface MemberView {
  subject: string;
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
