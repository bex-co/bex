import type { MemberView } from "@/features/team/types";

/** The translation-key pair a refused control shows, or null when the control
 *  is allowed. Pure and exported so the mapping is testable directly: Radix
 *  renders tooltip content into a portal that jsdom never mounts, so the row
 *  test can assert that a control is disabled but not what its tooltip says. */
export function memberActionReasonKeys(member: MemberView): {
  removeReason: string | null;
  roleReason: string | null;
} {
  // "You cannot act on yourself" names Leave workspace (w5/m102), the exit
  // that replaces self-removal. The workspace's creator is an ordinary Admin
  // (w5/m372), so no other row is refused up front.
  if (member.isSelf) {
    return {
      removeReason: "team.removeSelfReason",
      roleReason: "team.changeOwnRoleReason",
    };
  }
  return { removeReason: null, roleReason: null };
}
