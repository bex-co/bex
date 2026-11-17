import { describe, expect, it } from "vitest";
import { workspaceRowActions } from "@/features/organizations/lib/workspace-row-actions";

// ADR115 D3 and Render's `JoinTeamEvent`: "Org Owners can add themselves to
// any workspace as an admin. Other org members can add themselves to public
// workspaces only"; a Guest joins none.
describe("workspaceRowActions", () => {
  it.each([
    // role, privacy, in it, actions
    ["OWNER", "public", false, ["join", "joinAsAdmin"]],
    ["OWNER", "invite_only", false, ["joinAsAdmin"]],
    ["MEMBER", "public", false, ["join"]],
    ["MEMBER", "invite_only", false, []],
    ["GUEST", "public", false, []],
    ["GUEST", "invite_only", false, []],
    [null, "public", false, []],
    ["OWNER", "invite_only", true, ["open"]],
    ["MEMBER", "public", true, ["open"]],
    ["GUEST", "invite_only", true, ["open"]],
  ] as const)(
    "%s on a %s workspace (member: %s) → %j",
    (role, privacy, viewerIsMember, actions) => {
      expect(workspaceRowActions(role, { privacy, viewerIsMember })).toEqual(
        actions,
      );
    },
  );
});
