import type { MemberView } from "@/features/team/types";

/** The translation-key pair a refused control shows, or null when the control
 *  is allowed. Pure and exported so the mapping is testable directly: Radix
 *  renders tooltip content into a portal that jsdom never mounts, so the row
 *  test can assert that a control is disabled but not what its tooltip says. */
export function memberActionReasonKeys(member: MemberView): {
  removeReason: string | null;
  roleReason: string | null;
} {
  // Self wins over owner in the copy: on your own owner row, "you cannot act on
  // yourself" is the reason you would hit first, and it names Leave workspace
  // (w5/m102) — the exit that replaces self-removal.
  if (member.isSelf) {
    return {
      removeReason: "team.removeSelfReason",
      roleReason: "team.changeOwnRoleReason",
    };
  }
  if (member.isOwner) {
    return {
      removeReason: "team.removeOwnerReason",
      roleReason: "team.changeOwnerRoleReason",
    };
  }
  return { removeReason: null, roleReason: null };
}
