import {
  chooseWorkspace,
  normalizeWorkspaces,
  workspaceSelectionKey,
} from "../workspace-selection";

describe("workspace selection", () => {
  const workspaces = [
    { id: "tea-alpha", name: "Alpha", plan: "hobby", role: "admin" },
    { id: "tea-bravo", name: "Bravo", plan: "team", role: "viewer" },
  ];

  it("restores only a workspace returned for the current caller", () => {
    expect(chooseWorkspace(workspaces, "tea-bravo")?.id).toBe("tea-bravo");
    expect(chooseWorkspace(workspaces, "tea-foreign")?.id).toBe("tea-alpha");
  });

  it("drops malformed nullable rows from the API boundary", () => {
    const normalized = normalizeWorkspaces([
      null,
      {
        __typename: "Workspace",
        id: "not-a-workspace",
        name: "Nope",
        plan: null,
        role: null,
        createdAt: null,
      },
      {
        __typename: "Workspace",
        id: "tea-valid",
        name: "  Valid  ",
        plan: null,
        role: null,
        createdAt: null,
      },
    ]);
    expect(normalized).toEqual([
      { id: "tea-valid", name: "Valid", plan: null, role: null },
    ]);
  });

  it("partitions harmless persistence by native login session", () => {
    expect(workspaceSelectionKey("session-a")).toBe(
      "bex.mobile.workspace.session-a",
    );
  });
});
