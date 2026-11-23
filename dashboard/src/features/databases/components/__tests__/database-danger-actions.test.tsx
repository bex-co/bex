import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatabaseDangerActions } from "@/features/databases/components/database-danger-actions";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useAdminOnlyActions } from "@/features/capabilities/hooks/use-admin-only-actions";
import { mockCapabilities } from "@/test/mocks/capabilities";
import {
  ADMIN_ONLY_REASON,
  ROLE_REASON,
  adminOnlyAs,
} from "@/test/mocks/admin-only";
import { lifecycleRows, lifecycleState } from "@/test/mocks/resource-actions";
import type { UseDatabaseLifecycleResult } from "@/features/databases/hooks/use-database-lifecycle";
import type { DatabaseView } from "@/features/databases/types";

const remove = vi.fn();
vi.mock("@/features/databases/hooks/use-delete-database", () => ({
  useDeleteDatabase: () => ({ remove, deleting: null }),
}));

// Suspend and Resume read the database's own projection, and a fresh read of
// it right before dispatch (w4/m578).
const projection = vi.hoisted(() => ({ state: null as unknown }));
vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useDatastoreActions: () => projection.state,
}));
const apolloQuery = vi.hoisted(() => vi.fn());
vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ query: apolloQuery }),
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-test" }),
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

function lifecycleStub(
  run = vi.fn().mockResolvedValue({ status: "success" }),
): UseDatabaseLifecycleResult {
  return { pending: null, run };
}

beforeEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  adminOnlyAs("admin");
  remove.mockReset();
  remove.mockResolvedValue({ status: "success" });
  projection.state = lifecycleState(DB.id, "allowed");
  apolloQuery.mockReset();
  apolloQuery.mockResolvedValue({
    data: { databaseActions: lifecycleRows("allowed") },
  });
});

describe("DatabaseDangerActions — detail-page bottom action row", () => {
  it("keeps can_operate lifecycle actions but disables can_create delete for a contributor", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    // Delete opens with can_create, so its decision is the role's refusal.
    adminOnlyAs("viewer");
    const lifecycle = lifecycleStub();
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycle}
      />,
    );

    const removeButton = screen.getByRole("button", {
      name: "Delete Database",
    });
    expect(removeButton).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Suspend Database" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Restart Database" }),
    ).toBeEnabled();

    await user.hover(removeButton.parentElement!);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(ROLE_REASON);
    await user.click(removeButton);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(remove).not.toHaveBeenCalled();
  });

  // w4/724: Render lets only an Admin delete a protected environment's
  // database. A Developer used to be offered Delete, type the phrase and read
  // "Forbidden"; now the button says why before the dialog opens.
  it("disables Delete for a Developer in a protected environment with Render's reason", async () => {
    adminOnlyAs("developer");
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycleStub()}
      />,
    );
    expect(useAdminOnlyActions).toHaveBeenCalledWith("database", DB.id);

    const removeButton = screen.getByRole("button", {
      name: "Delete Database",
    });
    expect(removeButton).toBeDisabled();
    await user.hover(removeButton.parentElement!);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      ADMIN_ONLY_REASON,
    );
    await user.click(removeButton);
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(remove).not.toHaveBeenCalled();
    // Restart stays theirs: Render leaves it open in a protected environment.
    expect(
      screen.getByRole("button", { name: "Restart Database" }),
    ).toBeEnabled();
  });

  it("disables lifecycle actions with the role's reason for a viewer", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        role: "VIEWER",
        canCreate: false,
        canOperate: false,
      }),
    );
    projection.state = lifecycleState(DB.id, "viewer");
    const lifecycle = lifecycleStub();
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycle}
      />,
    );

    const suspend = screen.getByRole("button", { name: "Suspend Database" });
    expect(suspend).toBeDisabled();
    await user.hover(suspend.parentElement!);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Your role can’t perform this action.",
    );
    await user.click(suspend);
    expect(
      screen.getByRole("button", { name: "Restart Database" }),
    ).toBeDisabled();
    expect(lifecycle.run).not.toHaveBeenCalled();
  });

  // w4/m578: in a protected environment only an Admin may suspend or resume,
  // so a Developer reads Render's reason before the click and sends nothing.
  // Restart, which Render leaves open there, stays on can_operate.
  it.each([
    ["Suspend Database", DB],
    ["Resume Database", { ...DB, suspended: "suspended" }],
  ])(
    "disables %s with Render's admin-only reason in a protected environment",
    async (name, database) => {
      projection.state = lifecycleState(DB.id, "adminOnly");
      const lifecycle = lifecycleStub();
      const user = userEvent.setup();
      render(
        <DatabaseDangerActions
          database={database}
          onDeleted={vi.fn()}
          lifecycle={lifecycle}
        />,
      );

      const control = screen.getByRole("button", { name });
      expect(control).toBeDisabled();
      await user.hover(control.parentElement!);
      expect(await screen.findByRole("tooltip")).toHaveTextContent(
        "This action is limited to admins.",
      );
      await user.click(control);
      expect(screen.queryByRole("dialog")).toBeNull();
      expect(lifecycle.run).not.toHaveBeenCalled();
      expect(apolloQuery).not.toHaveBeenCalled();
      if (database === DB) {
        expect(
          screen.getByRole("button", { name: "Restart Database" }),
        ).toBeEnabled();
      }
    },
  );

  it("disables Suspend while its decision is still being checked", async () => {
    projection.state = { status: "checking", refresh: async () => undefined };
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycleStub()}
      />,
    );

    const suspend = screen.getByRole("button", { name: "Suspend Database" });
    expect(suspend).toBeDisabled();
    await user.hover(suspend.parentElement!);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Checking whether you can perform this action…",
    );
  });

  it("sends nothing when the fresh check before dispatch refuses", async () => {
    apolloQuery.mockResolvedValue({
      data: { databaseActions: lifecycleRows("adminOnly") },
    });
    const lifecycle = lifecycleStub();
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycle}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Suspend Database" }));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Suspend" }));
    await waitFor(() => expect(apolloQuery).toHaveBeenCalledTimes(1));
    expect(apolloQuery.mock.calls[0][0]).toMatchObject({
      variables: { id: DB.id, ownerId: "tea-test" },
      fetchPolicy: "no-cache",
    });
    expect(lifecycle.run).not.toHaveBeenCalled();
  });

  it("renders Delete / Restart / Suspend Database buttons", () => {
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycleStub()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Delete Database" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Restart Database" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Suspend Database" }),
    ).toBeEnabled();
  });

  it("gates delete behind the sudo type-to-confirm phrase", async () => {
    const onDeleted = vi.fn();
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={onDeleted}
        lifecycle={lifecycleStub()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Delete Database" }));
    const dialog = await screen.findByRole("alertdialog");
    const confirm = within(dialog).getByRole("button", {
      name: "Delete database",
    });
    expect(confirm).toBeDisabled();

    expect(
      within(dialog).getByText("sudo delete postgres shop-db"),
    ).toBeInTheDocument();
    const input = within(dialog).getByLabelText("Sudo Command");
    // The bare database name is deliberately insufficient.
    await user.type(input, "shop-db");
    expect(confirm).toBeDisabled();
    await user.clear(input);
    await user.type(input, "sudo delete postgres shop-db");
    expect(confirm).toBeEnabled();

    await user.click(confirm);
    expect(remove).toHaveBeenCalledWith("shop-db", "shop-db");
    expect(onDeleted).toHaveBeenCalledWith("shop-db");
  });

  it("confirms suspend and restart through a dialog before running the verb", async () => {
    const lifecycle = lifecycleStub();
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycle}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Suspend Database" }));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Suspend" }));
    await waitFor(() =>
      expect(lifecycle.run).toHaveBeenCalledWith("suspend", DB),
    );

    await user.click(screen.getByRole("button", { name: "Restart Database" }));
    const restartDialog = await screen.findByRole("dialog");
    await user.click(
      within(restartDialog).getByRole("button", { name: "Restart" }),
    );
    expect(lifecycle.run).toHaveBeenCalledWith("restart", DB);
  });

  it("swaps Suspend for Resume (immediate, no confirm) while suspended", async () => {
    const lifecycle = lifecycleStub();
    const suspended = { ...DB, suspended: "suspended" };
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={suspended}
        onDeleted={vi.fn()}
        lifecycle={lifecycle}
      />,
    );

    expect(
      screen.queryByRole("button", { name: "Suspend Database" }),
    ).toBeNull();
    expect(
      screen.getByRole("button", { name: "Restart Database" }),
    ).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Resume Database" }));
    await waitFor(() =>
      expect(lifecycle.run).toHaveBeenCalledWith("resume", suspended),
    );
  });

  it("prompts with the server-issued phrase when a protected suspend is blocked", async () => {
    const run = vi
      .fn()
      .mockResolvedValueOnce({
        status: "confirmation_required",
        confirmation: "sudo suspend database shop-db",
      })
      .mockResolvedValueOnce({ status: "success" });
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycleStub(run)}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Suspend Database" }));
    const firstDialog = await screen.findByRole("dialog");
    await user.click(
      within(firstDialog).getByRole("button", { name: "Suspend" }),
    );

    const protectedDialog = await screen.findByRole("dialog");
    expect(
      within(protectedDialog).getByText("sudo suspend database shop-db"),
    ).toBeInTheDocument();
    await user.type(
      within(protectedDialog).getByRole("textbox"),
      "sudo suspend database shop-db",
    );
    await user.click(
      within(protectedDialog).getByRole("button", { name: "Suspend" }),
    );
    await waitFor(() =>
      expect(run).toHaveBeenLastCalledWith(
        "suspend",
        DB,
        "sudo suspend database shop-db",
      ),
    );
  });

  it("disables Delete for a contributor without can_create", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    adminOnlyAs("viewer");
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycleStub()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Delete Database" }),
    ).toBeDisabled();
    // Restart/suspend are can_operate — a contributor keeps them.
    expect(
      screen.getByRole("button", { name: "Restart Database" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Suspend Database" }),
    ).toBeEnabled();
  });

  it("keeps Delete enabled for an admin", () => {
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycleStub()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Delete Database" }),
    ).toBeEnabled();
  });
});
