import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnvGroupActions } from "@/features/env-groups/components/env-group-actions";
import type { EnvGroupView } from "@/features/env-groups/types";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

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
      projects={[]}
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

// w21/036: the rename field applies the server's label rule before submit.
describe("EnvGroupActions — Rename dialog (w21/036)", () => {
  it("refuses a 101-character name inline and disables Save", async () => {
    const user = userEvent.setup();
    renderActions(WORKSPACE_GROUP);

    await user.click(screen.getByRole("button", { name: /manage/i }));
    await user.click(screen.getByRole("menuitem", { name: /rename/i }));
    const dialog = await screen.findByRole("dialog");
    const input = within(dialog).getByLabelText("Group name");
    await user.clear(input);
    await user.click(input);
    await user.paste("a".repeat(101));

    expect(
      within(dialog).getByRole("button", { name: "Save name" }),
    ).toBeDisabled();
    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      "up to 100 characters",
    );
  });
});

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

// w4/270: without a bounded column the dialog's implicit grid track grew to
// the long scope label's width, pushing title, fields and actions off screen.
describe("EnvGroupActions — bounded Move/Clone dialogs (w4/270)", () => {
  it.each([/move/i, /clone/i])(
    "%s keeps its content inside the dialog's column",
    async (item) => {
      const user = userEvent.setup();
      renderActions(WORKSPACE_GROUP);

      await user.click(screen.getByRole("button", { name: /manage/i }));
      await user.click(screen.getByRole("menuitem", { name: item }));

      expect(await screen.findByRole("dialog")).toHaveClass(
        "grid-cols-[minmax(0,1fr)]",
      );
    },
  );
});

// w3/m281: moving a group into or out of a protected environment is an
// Admin's move, as on Render.
describe("EnvGroupActions — protected placement (w3/m281)", () => {
  const environments = [
    { id: "env-prod", name: "production", protectedStatus: "protected" },
    { id: "env-dev", name: "dev", protectedStatus: "unprotected" },
  ] as never;
  const REASON =
    "Only an admin can create resources in a protected environment or move them into or out of it.";

  function renderWith(group: EnvGroupView, moveGroup = vi.fn()) {
    render(
      <EnvGroupActions
        group={group}
        environments={environments}
        projects={[]}
        renameGroup={vi.fn()}
        moveGroup={moveGroup}
        cloneGroup={vi.fn()}
        deleteGroup={vi.fn()}
        busy={false}
        onDeleted={vi.fn()}
        onCloned={vi.fn()}
      />,
    );
    return moveGroup;
  }

  afterEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  });

  it("keeps a non-admin's Move out of a protected environment focusable with the reason", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    const user = userEvent.setup();
    renderWith({ ...WORKSPACE_GROUP, environmentId: "env-prod" });
    await user.click(screen.getByRole("button", { name: /manage/i }));
    const move = screen.getByRole("menuitem", { name: /move/i });
    expect(move).toHaveAttribute("aria-disabled", "true");
    await user.click(move);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("disables a protected destination for a non-admin and says why", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    const user = userEvent.setup();
    renderWith(WORKSPACE_GROUP);
    await user.click(screen.getByRole("button", { name: /manage/i }));
    await user.click(screen.getByRole("menuitem", { name: /move/i }));
    const picker = await screen.findByRole("combobox", {
      name: /environment/i,
    });
    expect(picker).toHaveAccessibleDescription(REASON);
  });

  it("lets an admin move out of a protected environment", async () => {
    const user = userEvent.setup();
    renderWith({ ...WORKSPACE_GROUP, environmentId: "env-prod" });
    await user.click(screen.getByRole("button", { name: /manage/i }));
    const move = screen.getByRole("menuitem", { name: /move/i });
    expect(move).not.toHaveAttribute("aria-disabled");
    await user.click(move);
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
  });
});
