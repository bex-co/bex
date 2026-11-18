import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { shape } from "@/test/card-shape";
import { WorkspacePrivacyCard } from "@/features/workspaces/components/workspace-privacy-card";
import { WorkspacePrivacyCardSkeleton } from "@/features/workspaces/components/workspace-privacy-card-skeleton";
import type { WorkspaceView } from "@/features/workspaces/types";

const mutate = vi.hoisted(() => vi.fn());
const mutationState = vi.hoisted(() => ({ loading: false }));
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate, mutationState],
}));

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("sonner", () => ({ toast }));

const WORKSPACE: WorkspaceView = {
  id: "tea-staging",
  name: "staging",
  slug: "staging",
  plan: "scale",
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
  requireGoogleLogin: false,
  organization: {
    id: "org-1",
    name: "Acme",
    primaryWorkspaceId: "tea-hq",
    requireTwoFactor: false,
    requireGoogleLogin: false,
  },
  primary: false,
  privacy: "invite_only",
};

beforeEach(() => {
  mutate.mockReset();
  mutate.mockResolvedValue({ data: {} });
  toast.success.mockReset();
  toast.error.mockReset();
  mutationState.loading = false;
});

describe("WorkspacePrivacyCard (w7/m464)", () => {
  it("shows the saved setting and offers Render's two", () => {
    render(<WorkspacePrivacyCard workspace={WORKSPACE} />);

    const group = screen.getByRole("radiogroup", { name: "Workspace privacy" });
    expect(group).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Invite-only/ })).toBeChecked();
    expect(screen.getByRole("radio", { name: /Public/ })).not.toBeChecked();
    expect(screen.getByRole("radio", { name: /Public/ })).toHaveTextContent(
      "Any member of your organization can add themselves to this workspace.",
    );
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  // No refetch: the mutation answers the Workspace's privacy, which the cache
  // merges into the workspaces list.
  it("sets the workspace's privacy", async () => {
    const user = userEvent.setup();
    render(<WorkspacePrivacyCard workspace={WORKSPACE} />);

    await user.click(screen.getByRole("radio", { name: /Public/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(mutate).toHaveBeenCalledWith({
      variables: { workspaceId: "tea-staging", privacy: "public" },
    });
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith("Workspace privacy saved"),
    );
  });

  it("shows the server's refusal and keeps the draft", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError(
        "ORGANIZATION_WORKSPACE_REQUIRED",
        {},
        "bad request: this workspace is not in an organization",
      ),
    );
    const user = userEvent.setup();
    render(<WorkspacePrivacyCard workspace={WORKSPACE} />);

    await user.click(screen.getByRole("radio", { name: /Public/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "This workspace is not in an organization",
      ),
    );
    expect(screen.getByRole("radio", { name: /Public/ })).toBeChecked();
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });
});

// AGENTS.md "Shape parity is mandatory": the preview is the ready card's own
// header, both options at their own box, and the Save button's box.
describe("WorkspacePrivacyCardSkeleton matches the card (w7/m464)", () => {
  it("has the same card, header, options and action slot", () => {
    const { container: pendingRoot } = render(<WorkspacePrivacyCardSkeleton />);
    const { container: readyRoot } = render(
      <WorkspacePrivacyCard workspace={WORKSPACE} />,
    );
    const pending =
      pendingRoot.querySelector<HTMLElement>('[data-slot="card"]')!;
    const ready = readyRoot.querySelector<HTMLElement>('[data-slot="card"]')!;

    expect(pending.className).toBe(ready.className);
    expect(shape(pending.querySelector('[data-slot="card-header"]')!)).toBe(
      shape(ready.querySelector('[data-slot="card-header"]')!),
    );
    const pendingContent = pending.querySelector('[data-slot="card-content"]')!;
    const readyContent = ready.querySelector('[data-slot="card-content"]')!;
    expect(pendingContent.className).toBe(readyContent.className);
    expect(pendingContent.children).toHaveLength(readyContent.children.length);

    // The options: the same grid, the same buttons with the same copy.
    const pendingOptions = pendingContent.firstElementChild!;
    const readyOptions = readyContent.firstElementChild!;
    expect(pendingOptions.className).toBe(readyOptions.className);
    const pendingButtons = [...pendingOptions.querySelectorAll("button")];
    const readyButtons = [...readyOptions.querySelectorAll("button")];
    expect(pendingButtons.map((b) => b.textContent)).toEqual(
      readyButtons.map((b) => b.textContent),
    );
    for (const button of pendingButtons) {
      expect(button).toBeDisabled();
      expect(button).toHaveClass("flex", "items-start", "rounded-lg", "p-3");
    }

    // The Save button's own box (size sm), reserved with its label.
    const reserved = pendingContent.lastElementChild!;
    const save = readyContent.lastElementChild!;
    expect(reserved.textContent).toBe(save.textContent);
    expect(reserved.firstElementChild).toHaveClass("h-8", "invisible");
    expect(save).toHaveClass("h-8");
  });

  it("offers no control before its data exists", () => {
    const { container } = render(<WorkspacePrivacyCardSkeleton />);
    for (const button of container.querySelectorAll("button")) {
      expect(button).toBeDisabled();
      expect(button).toHaveAttribute("tabindex", "-1");
    }
  });
});
