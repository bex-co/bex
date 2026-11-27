import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { KeyValueRowActions } from "@/features/keyvalue/components/key-value-row-actions";
import type { KeyValueView } from "@/features/keyvalue/types";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useAdminOnlyActions } from "@/features/capabilities/hooks/use-admin-only-actions";
import { mockCapabilities } from "@/test/mocks/capabilities";
import {
  ADMIN_ONLY_REASON,
  ROLE_REASON,
  adminOnlyAs,
} from "@/test/mocks/admin-only";

// Generate Blueprint opens the Create Blueprint page (w3/m681).
const navigate = vi.hoisted(() => vi.fn());
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useNavigate: () => navigate,
}));

const remove = vi.fn();
vi.mock("@/features/keyvalue/hooks/use-delete-key-value", () => ({
  useDeleteKeyValue: () => ({ remove, deleting: null }),
}));

// KeyValueRowActions renders a "Move to project" submenu via useMoveToProject,
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

const KV: KeyValueView = {
  id: "sessions-cache",
  name: "sessions-cache",
  status: "available",
  plan: "starter",
  version: "8",
  createdAt: null,
  externalHost: null,
  public: false,
  suspended: false,
};

beforeEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  adminOnlyAs("admin");
  remove.mockReset();
  remove.mockResolvedValue({ status: "success" });
});

describe("KeyValueRowActions", () => {
  it("keeps a denied delete focusable with a reason and suppresses selection", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    // Delete opens with can_create, so its decision is the role's refusal.
    adminOnlyAs("viewer");
    const user = userEvent.setup();
    render(<KeyValueRowActions keyValue={KV} onDeleted={vi.fn()} />);

    await user.tab();
    // w4/511: the trigger names its row, so a list of rows has distinct menus.
    expect(
      screen.getByRole("button", { name: "Actions for sessions-cache" }),
    ).toHaveFocus();
    // The menu opens on its first item, Delete; Generate Blueprint follows.
    await user.keyboard("{Enter}");
    const removeItem = await screen.findByRole("menuitem", { name: "Delete" });
    expect(removeItem).toHaveAttribute("aria-disabled", "true");
    expect(removeItem).toHaveFocus();
    expect(await screen.findByRole("tooltip")).toHaveTextContent(ROLE_REASON);

    await user.keyboard("{Enter}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(remove).not.toHaveBeenCalled();
  });

  // w4/724: Render lets only an Admin delete a protected environment's Key
  // Value, from the list's row menu too. A Developer used to type the phrase
  // and read "Forbidden".
  it("disables Delete for a Developer in a protected environment with Render's reason", async () => {
    adminOnlyAs("developer");
    const user = userEvent.setup();
    render(<KeyValueRowActions keyValue={KV} onDeleted={vi.fn()} />);
    expect(useAdminOnlyActions).toHaveBeenCalledWith("keyValue", KV.id);

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
  });

  it("gates delete behind Render's exact sudo confirmation", async () => {
    const onDeleted = vi.fn();
    const user = userEvent.setup();
    render(<KeyValueRowActions keyValue={KV} onDeleted={onDeleted} />);

    await user.click(screen.getByRole("button", { name: /^Actions for / }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));

    const dialog = await screen.findByRole("alertdialog");
    const confirm = within(dialog).getByRole("button", {
      name: "Delete Key Value Instance",
    });
    expect(confirm).toBeDisabled(); // nothing typed yet

    const input = within(dialog).getByRole("textbox");
    await user.type(input, "wrong-name");
    expect(confirm).toBeDisabled();

    await user.clear(input);
    await user.type(input, "sudo delete key value sessions-cache");
    expect(confirm).toBeEnabled();

    await user.click(confirm);
    expect(remove).toHaveBeenCalledWith("sessions-cache", "sessions-cache");
    expect(onDeleted).toHaveBeenCalledWith("sessions-cache");
  });

  it("retries a protected delete only with the server-issued phrase", async () => {
    remove
      .mockResolvedValueOnce({
        status: "confirmation_required",
        confirmation: "sudo delete key value sessions-cache",
      })
      .mockResolvedValueOnce({ status: "success" });
    const onDeleted = vi.fn();
    const user = userEvent.setup();
    render(<KeyValueRowActions keyValue={KV} onDeleted={onDeleted} />);

    await user.click(screen.getByRole("button", { name: /^Actions for / }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    const localDialog = await screen.findByRole("alertdialog");
    await user.type(
      within(localDialog).getByRole("textbox"),
      "sudo delete key value sessions-cache",
    );
    await user.click(
      within(localDialog).getByRole("button", {
        name: "Delete Key Value Instance",
      }),
    );

    const protectedDialog = await screen.findByRole("dialog");
    await user.type(
      within(protectedDialog).getByRole("textbox"),
      "sudo delete key value sessions-cache",
    );
    await user.click(
      within(protectedDialog).getByRole("button", {
        name: "Delete Key Value Instance",
      }),
    );
    expect(remove).toHaveBeenLastCalledWith(
      "sessions-cache",
      "sessions-cache",
      "sudo delete key value sessions-cache",
    );
    expect(onDeleted).toHaveBeenCalledWith("sessions-cache");
  });
});

describe("KeyValueRowActions Generate Blueprint (w3/m681)", () => {
  beforeEach(() => navigate.mockReset());

  it("opens the Create Blueprint page with this Key Value", async () => {
    const user = userEvent.setup();
    render(<KeyValueRowActions keyValue={KV} onDeleted={vi.fn()} />);
    await user.click(
      screen.getByRole("button", { name: "Actions for sessions-cache" }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: "Generate Blueprint" }),
    );
    expect(navigate).toHaveBeenCalledWith({
      to: "/blueprint/generate",
      state: {
        blueprintExport: {
          resources: [
            { kind: "keyvalue", id: "sessions-cache", name: "sessions-cache" },
          ],
          source: "table row menu",
        },
      },
    });
  });

  it("is disabled with the role's reason for a Viewer", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "VIEWER", canCreate: false, canOperate: false }),
    );
    const user = userEvent.setup();
    render(<KeyValueRowActions keyValue={KV} onDeleted={vi.fn()} />);
    await user.click(
      screen.getByRole("button", { name: "Actions for sessions-cache" }),
    );
    const item = await screen.findByRole("menuitem", {
      name: "Generate Blueprint",
    });
    expect(item).toHaveAttribute("aria-disabled", "true");
    await user.click(item);
    expect(navigate).not.toHaveBeenCalled();
  });
});
