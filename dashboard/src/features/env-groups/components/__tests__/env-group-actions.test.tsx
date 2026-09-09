import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnvGroupActions } from "@/features/env-groups/components/env-group-actions";
import type { EnvGroupView } from "@/features/env-groups/types";

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    workspaces: [{ id: "tea-1", name: "tea-1" }],
    currentWorkspaceId: "tea-1",
    setCurrentWorkspaceId: vi.fn(),
  }),
}));
vi.mock("@/features/env-groups/hooks/use-env-group-scope-index", () => ({
  useWorkspaceEnvironmentIndex: () => ({
    projects: [],
    environments: [],
    loading: false,
  }),
}));

const WORKSPACE_GROUP: EnvGroupView = {
  id: "eg-1",
  name: "shared",
  ownerId: "tea-1",
  environmentId: null,
  createdAt: null,
  updatedAt: null,
  revision: "1",
  availability: null,
  serviceLinks: [],
  envVarKeys: [],
  secretFileNames: [],
};

function renderActions(group: EnvGroupView) {
  return render(
    <EnvGroupActions
      group={group}
      environments={[{ id: "env-1", name: "prod" } as never]}
      renameGroup={vi.fn()}
      moveGroup={vi.fn()}
      cloneGroup={vi.fn()}
      deleteGroup={vi.fn()}
      busy={false}
      onDeleted={vi.fn()}
      onCloned={vi.fn()}
    />,
  );
}

describe("EnvGroupActions — Move dialog (w6/m48)", () => {
  it("pre-selects Workspace (no Environment) for a workspace-scoped group", async () => {
    const user = userEvent.setup();
    renderActions(WORKSPACE_GROUP);

    await user.click(screen.getByRole("button", { name: /manage/i }));
    await user.click(screen.getByRole("menuitem", { name: /move/i }));

    expect(
      await screen.findByRole("combobox", { name: /environment/i }),
    ).toHaveTextContent("Workspace (no Environment)");
  });

  it("still pre-selects the real Environment for an environment-scoped group", async () => {
    const user = userEvent.setup();
    renderActions({ ...WORKSPACE_GROUP, environmentId: "env-1" });

    await user.click(screen.getByRole("button", { name: /manage/i }));
    await user.click(screen.getByRole("menuitem", { name: /move/i }));

    expect(
      await screen.findByRole("combobox", { name: /environment/i }),
    ).toHaveTextContent("prod");
  });
});
