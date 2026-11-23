import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatabaseRowActions } from "@/features/databases/components/database-row-actions";
import type { DatabaseView } from "@/features/databases/types";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useAdminOnlyActions } from "@/features/capabilities/hooks/use-admin-only-actions";
import { mockCapabilities } from "@/test/mocks/capabilities";
import {
  ADMIN_ONLY_REASON,
  ROLE_REASON,
  adminOnlyAs,
} from "@/test/mocks/admin-only";
import { lifecycleRows, lifecycleState } from "@/test/mocks/resource-actions";

const remove = vi.fn();
vi.mock("@/features/databases/hooks/use-delete-database", () => ({
  useDeleteDatabase: () => ({ remove, deleting: null }),
}));

// Suspend and Resume read the database's own projection, and a fresh read of
// it right before dispatch (w4/m578). `asked` records which id it was asked
// for: the list's rows offer no lifecycle and must ask for nothing.
const projection = vi.hoisted(() => ({
  state: null as unknown,
  asked: [] as (string | null)[],
}));
vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useDatastoreActions: (_kind: string, id: string | null) => {
    projection.asked.push(id);
    return projection.state;
  },
}));
const apolloQuery = vi.hoisted(() => vi.fn());
vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ query: apolloQuery }),
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-test" }),
}));

// DatabaseRowActions renders a "Move to project" submenu via useMoveToProject,
// which needs an ApolloProvider + workspace context neither this render nor
// the component under test cares about — stub it to the empty-projects shape
// (the submenu renders nothing when there are no projects).
vi.mock("@/features/projects/hooks/use-move-to-project", () => ({
  useMoveToProject: () => ({
    projects: [],
    currentProjectId: () => null,
    moveTo: vi.fn(),
    removeFromProject: vi.fn(),
    busyId: null,
  }),
}));

const DB: DatabaseView = {
  id: "shop-db",
  name: "shop-db",
  status: "available",
  plan: "free",
  version: "18",
  diskSizeGB: 1,
  createdAt: null,
  public: false,
  suspended: "not_suspended",
};

beforeEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  adminOnlyAs("admin");
  remove.mockReset();
  remove.mockResolvedValue({ status: "success" });
  projection.state = lifecycleState(DB.id, "allowed");
  projection.asked = [];
  apolloQuery.mockReset();
  apolloQuery.mockResolvedValue({
    data: { databaseActions: lifecycleRows("allowed") },
  });
});

describe("DatabaseRowActions", () => {
  it("keeps a denied delete focusable with a reason and suppresses selection", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    // Delete opens with can_create, so its decision is the role's refusal.
    adminOnlyAs("viewer");
    const user = userEvent.setup();
    render(<DatabaseRowActions database={DB} onDeleted={vi.fn()} />);

    await user.tab();
    // w4/511: the trigger names its row, so a list of rows has distinct menus.
    expect(
      screen.getByRole("button", { name: "Actions for shop-db" }),
    ).toHaveFocus();
    await user.keyboard("{Enter}{ArrowDown}");
    const removeItem = await screen.findByRole("menuitem", { name: "Delete" });
    expect(removeItem).toHaveAttribute("aria-disabled", "true");
    expect(removeItem).toHaveFocus();
    expect(await screen.findByRole("tooltip")).toHaveTextContent(ROLE_REASON);

    await user.keyboard("{Enter}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(remove).not.toHaveBeenCalled();
  });

  // w4/724: Render lets only an Admin delete a protected environment's
  // database, from the list's row menu as from the detail page's. A Developer
  // used to type the phrase and read "Forbidden".
  it.each([
    ["a list row", undefined],
    ["the detail header", { pending: null, run: vi.fn() }],
  ])(
    "disables Delete on %s for a Developer in a protected environment with Render's reason",
    async (_where, lifecycle) => {
      adminOnlyAs("developer");
      const user = userEvent.setup();
      render(
        <DatabaseRowActions
          database={DB}
          onDeleted={vi.fn()}
          lifecycle={lifecycle}
        />,
      );
      expect(useAdminOnlyActions).toHaveBeenCalledWith("database", DB.id);

      await user.click(screen.getByRole("button", { name: /^Actions for / }));
      const item = await screen.findByRole("menuitem", { name: "Delete" });
      expect(item).toHaveAttribute("aria-disabled", "true");
      await user.hover(item);
      expect(await screen.findByRole("tooltip")).toHaveTextContent(
        ADMIN_ONLY_REASON,
      );
      await user.click(item);
      expect(screen.queryByRole("alertdialog")).toBeNull();
      expect(remove).not.toHaveBeenCalled();
    },
  );

  it("asks for no lifecycle decisions on a list row, which offers no lifecycle", () => {
    render(<DatabaseRowActions database={DB} onDeleted={vi.fn()} />);
    expect(projection.asked.length).toBeGreaterThan(0);
    expect(projection.asked.every((id) => id === null)).toBe(true);
  });

  it("gates lifecycle menu items on their decisions and suppresses viewer selection", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        role: "VIEWER",
        canCreate: false,
        canOperate: false,
      }),
    );
    projection.state = lifecycleState(DB.id, "viewer");
    const run = vi.fn().mockResolvedValue({ status: "success" });
    const user = userEvent.setup();
    render(
      <DatabaseRowActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={{ pending: null, run }}
      />,
    );

    await user.click(screen.getByRole("button", { name: /^Actions for / }));
    const suspend = await screen.findByRole("menuitem", { name: "Suspend" });
    const restart = screen.getByRole("menuitem", { name: "Restart" });
    expect(suspend).toHaveAttribute("aria-disabled", "true");
    expect(restart).toHaveAttribute("aria-disabled", "true");

    await user.click(suspend);
    expect(run).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  // w4/m578: a Developer in a protected environment reads Render's reason on
  // Suspend and Resume and sends nothing; Restart stays theirs.
  it.each([
    ["Suspend", DB],
    ["Resume", { ...DB, suspended: "suspended" }],
  ])(
    "disables %s with Render's admin-only reason in a protected environment",
    async (name, database) => {
      projection.state = lifecycleState(DB.id, "adminOnly");
      const run = vi.fn().mockResolvedValue({ status: "success" });
      const user = userEvent.setup();
      render(
        <DatabaseRowActions
          database={database}
          onDeleted={vi.fn()}
          lifecycle={{ pending: null, run }}
        />,
      );

      await user.click(screen.getByRole("button", { name: /^Actions for / }));
      const item = await screen.findByRole("menuitem", { name });
      expect(item).toHaveAttribute("aria-disabled", "true");
      await user.hover(item);
      expect(await screen.findByRole("tooltip")).toHaveTextContent(
        "This action is limited to admins.",
      );
      if (database === DB) {
        expect(
          screen.getByRole("menuitem", { name: "Restart" }),
        ).not.toHaveAttribute("aria-disabled");
      }
      await user.click(item);
      expect(screen.queryByRole("dialog")).toBeNull();
      expect(run).not.toHaveBeenCalled();
      expect(apolloQuery).not.toHaveBeenCalled();
    },
  );

  it("suspends after the confirm once a fresh check still allows it", async () => {
    const run = vi.fn().mockResolvedValue({ status: "success" });
    const user = userEvent.setup();
    render(
      <DatabaseRowActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={{ pending: null, run }}
      />,
    );

    await user.click(screen.getByRole("button", { name: /^Actions for / }));
    await user.click(await screen.findByRole("menuitem", { name: "Suspend" }));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Suspend" }));
    await waitFor(() => expect(run).toHaveBeenCalledWith("suspend", DB));
    expect(apolloQuery).toHaveBeenCalledTimes(1);
  });

  it("gates delete behind the sudo type-to-confirm phrase", async () => {
    const onDeleted = vi.fn();
    const user = userEvent.setup();
    render(<DatabaseRowActions database={DB} onDeleted={onDeleted} />);

    await user.click(screen.getByRole("button", { name: /^Actions for / }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));

    const dialog = await screen.findByRole("alertdialog");
    const confirm = within(dialog).getByRole("button", {
      name: "Delete database",
    });
    expect(confirm).toBeDisabled(); // nothing typed yet

    const input = within(dialog).getByRole("textbox");
    await user.type(input, "wrong-name");
    expect(confirm).toBeDisabled();

    await user.clear(input);
    await user.type(input, "sudo delete postgres shop-db");
    expect(confirm).toBeEnabled();

    await user.click(confirm);
    expect(remove).toHaveBeenCalledWith("shop-db", "shop-db");
    expect(onDeleted).toHaveBeenCalledWith("shop-db");
  });

  it("retries a protected delete only with the server-issued phrase", async () => {
    remove
      .mockResolvedValueOnce({
        status: "confirmation_required",
        confirmation: "sudo delete database shop-db",
      })
      .mockResolvedValueOnce({ status: "success" });
    const onDeleted = vi.fn();
    const user = userEvent.setup();
    render(<DatabaseRowActions database={DB} onDeleted={onDeleted} />);

    await user.click(screen.getByRole("button", { name: /^Actions for / }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    const localDialog = await screen.findByRole("alertdialog");
    await user.type(
      within(localDialog).getByRole("textbox"),
      "sudo delete postgres shop-db",
    );
    await user.click(
      within(localDialog).getByRole("button", { name: "Delete database" }),
    );

    const protectedDialog = await screen.findByRole("dialog");
    expect(
      within(protectedDialog).getByText("sudo delete database shop-db"),
    ).toBeInTheDocument();
    const retry = within(protectedDialog).getByRole("button", {
      name: "Delete database",
    });
    expect(retry).toBeDisabled();
    await user.type(
      within(protectedDialog).getByRole("textbox"),
      "sudo delete database shop-db",
    );
    await user.click(retry);

    expect(remove).toHaveBeenLastCalledWith(
      "shop-db",
      "shop-db",
      "sudo delete database shop-db",
    );
    expect(onDeleted).toHaveBeenCalledWith("shop-db");
  });
});
