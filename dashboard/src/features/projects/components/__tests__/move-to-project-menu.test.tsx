import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { MoveToProjectMenu } from "@/features/projects/components/move-to-project-menu";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

const moveTo = vi.fn(async () => true);
const removeFromProject = vi.fn(async () => true);

vi.mock("@/features/projects/hooks/use-move-to-project", () => ({
  useMoveToProject: () => ({
    projects: [
      { id: "prj-1", name: "First", serviceIds: ["srv-1"] },
      { id: "prj-2", name: "Second", serviceIds: [] },
    ],
    currentProjectId: () => "prj-1",
    moveTo,
    removeFromProject,
    busyId: null,
  }),
}));

// The environment srv-1 is in now; a protected one gates the move (w3/m281).
const scope = vi.hoisted(() => ({ protectedStatus: "unprotected" }));
vi.mock("@/features/env-groups/hooks/use-env-group-scope-index", () => ({
  useEnvGroupScopeIndex: () => ({
    environments: [
      {
        id: "evm-prod",
        name: "production",
        protectedStatus: scope.protectedStatus,
        serviceIds: ["srv-1"],
        databaseIds: [],
        keyValueIds: [],
        envGroupIds: [],
      },
    ],
  }),
}));

function renderMenu(onMoveRequested?: (proceed: () => void) => void) {
  return render(
    <DropdownMenu>
      <DropdownMenuTrigger>
        <span>open</span>
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        <MoveToProjectMenu
          kind="service"
          resourceId="srv-1"
          resourceName="web"
          onMoveRequested={onMoveRequested}
        />
      </DropdownMenuContent>
    </DropdownMenu>,
  );
}

async function choose(user: ReturnType<typeof userEvent.setup>, label: string) {
  await user.click(screen.getByText("open"));
  // Drive the submenu by keyboard: jsdom's zero-size rects defeat Radix's
  // pointer-intent grace area, so a pointer click on a sub item never
  // selects. ArrowRight opens the submenu; Enter selects the focused item.
  const sub = await screen.findByText("Move to project");
  sub.focus();
  await user.keyboard("{ArrowRight}");
  const item = await screen.findByRole("menuitem", { name: label });
  item.focus();
  await user.keyboard("{Enter}");
}

describe("MoveToProjectMenu stranded-link interception (w4/m238)", () => {
  beforeEach(() => {
    moveTo.mockClear();
    removeFromProject.mockClear();
    scope.protectedStatus = "unprotected";
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  });

  it("moves directly when no host intercepts", async () => {
    const user = userEvent.setup();
    renderMenu();
    await choose(user, "Second");
    expect(moveTo).toHaveBeenCalledTimes(1);
    expect(moveTo).toHaveBeenCalledWith("srv-1", "web", "prj-2");
  });

  it("defers the write to the host's proceed", async () => {
    const user = userEvent.setup();
    let proceed: (() => void) | undefined;
    const onMoveRequested = vi.fn((p: () => void) => {
      proceed = p;
    });
    renderMenu(onMoveRequested);
    await choose(user, "Second");
    expect(onMoveRequested).toHaveBeenCalledTimes(1);
    expect(moveTo).not.toHaveBeenCalled();
    proceed?.();
    expect(moveTo).toHaveBeenCalledTimes(1);
    expect(moveTo).toHaveBeenCalledWith("srv-1", "web", "prj-2");
  });

  it("defers Remove from project the same way", async () => {
    const user = userEvent.setup();
    let proceed: (() => void) | undefined;
    renderMenu((p) => {
      proceed = p;
    });
    await choose(user, "Remove from project");
    expect(removeFromProject).not.toHaveBeenCalled();
    proceed?.();
    expect(removeFromProject).toHaveBeenCalledTimes(1);
    expect(removeFromProject).toHaveBeenCalledWith("srv-1", "web");
  });
});

describe("MoveToProjectMenu protected placement (w3/m281)", () => {
  beforeEach(() => {
    moveTo.mockClear();
    scope.protectedStatus = "protected";
  });

  it("keeps a non-admin's move out of a protected environment focusable with the reason and runs nothing", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    const user = userEvent.setup();
    renderMenu();
    await user.click(screen.getByText("open"));
    const item = await screen.findByRole("menuitem", {
      name: "Move to project",
    });
    expect(item).toHaveAttribute("aria-disabled", "true");
    await user.keyboard("{ArrowDown}");
    expect(item).toHaveFocus();
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Only an admin can create resources in a protected environment or move them into or out of it.",
    );
    await user.keyboard("{Enter}");
    expect(moveTo).not.toHaveBeenCalled();
  });

  it("lets an admin move a resource out of a protected environment", async () => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
    const user = userEvent.setup();
    renderMenu();
    await choose(user, "Second");
    expect(moveTo).toHaveBeenCalledWith("srv-1", "web", "prj-2");
  });
});
