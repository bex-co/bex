import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeleteWorkspaceCard } from "@/features/workspaces/components/delete-workspace-card";
import type { WorkspaceView } from "@/features/workspaces/types";
import type { BillingBalanceDue } from "@/common/lib/graphql-error";

const mockNavigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

const remove = vi.fn();
const deleteState: { balanceDue: BillingBalanceDue | null } = {
  balanceDue: null,
};
vi.mock("@/features/workspaces/hooks/use-delete-workspace", () => ({
  useDeleteWorkspace: () => ({
    remove,
    busy: false,
    error: null,
    balanceDue: deleteState.balanceDue,
  }),
}));

const setCurrentWorkspaceId = vi.fn();
const refetch = vi.fn();
const workspaceState: { workspaces: WorkspaceView[] } = { workspaces: [] };
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ ...workspaceState, setCurrentWorkspaceId, refetch }),
}));

const WORKSPACE: WorkspaceView = {
  id: "tea-1",
  name: "acme-hq",
  slug: "acme-hq",
  plan: "hobby",
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
};
const OTHER: WorkspaceView = {
  id: "tea-2",
  name: "acme-staging",
  slug: "acme-staging",
  plan: "pro",
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
};

beforeEach(() => {
  mockNavigate.mockReset();
  remove.mockReset();
  remove.mockResolvedValue(true);
  setCurrentWorkspaceId.mockReset();
  refetch.mockReset();
  workspaceState.workspaces = [WORKSPACE, OTHER];
  deleteState.balanceDue = null;
});

// Render's live guard: the full "sudo delete workspace <name>" phrase, not the
// bare name (w6/m5/t002, docs/render-artifacts/workspace-lifecycle.md).
const PHRASE = "sudo delete workspace acme-hq";

describe("DeleteWorkspaceCard — sudo-phrase confirm guard (w6/m3/t004, w6/m5/t002)", () => {
  it("keeps delete disabled until the full sudo phrase is typed (bare name is not enough)", async () => {
    const user = userEvent.setup();
    render(<DeleteWorkspaceCard workspace={WORKSPACE} />);

    // The exact phrase is body copy; the input itself is labeled "Sudo Command".
    expect(screen.getByText(PHRASE)).toBeInTheDocument();
    const input = screen.getByLabelText("Sudo Command");
    const button = screen.getByRole("button", { name: "Delete Workspace" });
    expect(button).toBeDisabled();

    // The bare workspace name is deliberately insufficient.
    await user.type(input, "acme-hq");
    expect(button).toBeDisabled();
    expect(remove).not.toHaveBeenCalled();

    await user.clear(input);
    await user.type(input, PHRASE);
    expect(button).toBeEnabled();
  });

  it("a typo is a no-op, not a destroyed workspace", async () => {
    const user = userEvent.setup();
    render(<DeleteWorkspaceCard workspace={WORKSPACE} />);

    await user.type(screen.getByLabelText("Sudo Command"), `${PHRASE} `);
    const button = screen.getByRole("button", { name: "Delete Workspace" });
    expect(button).toBeDisabled();

    await user.click(button);
    expect(remove).not.toHaveBeenCalled();
  });

  it("on success, switches to a remaining workspace and navigates home", async () => {
    const user = userEvent.setup();
    render(<DeleteWorkspaceCard workspace={WORKSPACE} />);

    await user.type(screen.getByLabelText("Sudo Command"), PHRASE);
    await user.click(screen.getByRole("button", { name: "Delete Workspace" }));

    expect(remove).toHaveBeenCalledWith("tea-1", "acme-hq", PHRASE);
    expect(setCurrentWorkspaceId).toHaveBeenCalledWith("tea-2");
    expect(mockNavigate).toHaveBeenCalledWith({ to: "/", replace: true });
  });

  it("routes to /new/workspace when no workspace remains", async () => {
    workspaceState.workspaces = [WORKSPACE];
    const user = userEvent.setup();
    render(<DeleteWorkspaceCard workspace={WORKSPACE} />);

    await user.type(screen.getByLabelText("Sudo Command"), PHRASE);
    await user.click(screen.getByRole("button", { name: "Delete Workspace" }));

    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
    expect(mockNavigate).toHaveBeenCalledWith({
      to: "/new/workspace",
      replace: true,
      search: { attempt: undefined },
    });
  });
});

describe("DeleteWorkspaceCard — unpaid balance (ADR091 D10.4)", () => {
  it("shows the balance and a Pay invoice link instead of a generic error", () => {
    deleteState.balanceDue = {
      workspaceId: "tea-1",
      amountDueCents: 300,
      currency: "usd",
      hostedInvoiceUrl: "https://invoice.stripe.com/i/acct_1/in_1",
    };
    render(<DeleteWorkspaceCard workspace={WORKSPACE} />);

    expect(screen.getByText("Unpaid balance")).toBeInTheDocument();
    expect(
      screen.getByText(
        "This workspace has an unpaid balance of $3.00. Pay the invoice, then delete it again.",
      ),
    ).toBeInTheDocument();
    const pay = screen.getByRole("link", { name: /Pay invoice/ });
    expect(pay).toHaveAttribute(
      "href",
      "https://invoice.stripe.com/i/acct_1/in_1",
    );
    expect(pay).toHaveAttribute("target", "_blank");
    expect(pay.getAttribute("rel")).toContain("noopener");
    expect(screen.queryByText("Couldn't delete workspace")).toBeNull();
  });

  it("words the balance without an amount and omits the link without a URL", () => {
    deleteState.balanceDue = {
      workspaceId: "tea-1",
      amountDueCents: null,
      currency: "usd",
      hostedInvoiceUrl: null,
    };
    render(<DeleteWorkspaceCard workspace={WORKSPACE} />);

    expect(
      screen.getByText(
        "This workspace has an unpaid balance. Pay the invoice, then delete it again.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Pay invoice/ })).toBeNull();
  });
});
