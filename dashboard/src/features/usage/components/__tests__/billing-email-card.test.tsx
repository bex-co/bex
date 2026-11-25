import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { BillingEmailCardView } from "@/features/usage/components/billing-email-card";
import { BillingEmailCardSkeleton } from "@/features/usage/components/billing-email-card-skeleton";
import { useBillingEmail } from "@/features/usage/hooks/use-billing-email";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

vi.mock("@/features/usage/hooks/use-billing-email", () => ({
  useBillingEmail: vi.fn(),
}));

let plan = "pro";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    currentWorkspaceId: "tea-a",
    currentWorkspace: { id: "tea-a", plan },
    workspaces: [],
  }),
}));

const mocked = vi.mocked(useBillingEmail);
const HOBBY_NOTE =
  "Hobby workspaces use the owner's account email. A separate billing email is available on Pro and above.";

function billingEmail(email: string, configurable: boolean, saved = true) {
  const save = vi.fn().mockResolvedValue(saved);
  mocked.mockReturnValue({
    billingEmail: {
      __typename: "WorkspaceBillingEmail",
      workspaceId: "tea-a",
      email,
      configurable,
    },
    loading: false,
    error: undefined,
    saving: false,
    save,
  });
  return save;
}

// w6/m627: as on Render, a Pro or higher workspace sets its own billing
// email, and a Hobby workspace's is its owner's account email.
describe("BillingEmailCard", () => {
  beforeEach(() => {
    mocked.mockReset();
    plan = "pro";
  });

  it("lets a Pro workspace's billing manager set an address of its own", async () => {
    const save = billingEmail("owner@example.com", true);
    // Inside the Billing page's own #billing-email section, as it is mounted,
    // so the input's label cannot point at the section instead.
    render(
      <section id="billing-email">
        <BillingEmailCardView workspaceId="tea-a" />
      </section>,
    );
    expect(screen.getByText("owner@example.com")).toBeInTheDocument();
    expect(screen.queryByText(HOBBY_NOTE)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    const input = screen.getByLabelText("Email address");
    expect(input).toHaveValue("owner@example.com");
    fireEvent.change(input, { target: { value: "finance@" } });
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    fireEvent.change(input, { target: { value: " finance@example.com " } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(save).toHaveBeenCalledWith("finance@example.com");
    await waitFor(() =>
      expect(screen.queryByLabelText("Email address")).not.toBeInTheDocument(),
    );
  });

  it("keeps the dialog open with the address when the write is refused", async () => {
    const save = billingEmail("owner@example.com", true, false);
    render(<BillingEmailCardView workspaceId="tea-a" />);
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    const input = screen.getByLabelText("Email address");
    fireEvent.change(input, { target: { value: "finance@example.com" } });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(screen.getByLabelText("Email address")).toHaveValue(
      "finance@example.com",
    );
  });

  it("shows a Hobby workspace's owner email read-only, with the plan that unlocks it", () => {
    plan = "hobby";
    const save = billingEmail("owner@example.com", false);
    render(<BillingEmailCardView workspaceId="tea-a" />);
    expect(screen.getByText("owner@example.com")).toBeInTheDocument();
    expect(screen.getByText(HOBBY_NOTE)).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(save).not.toHaveBeenCalled();
  });

  it("does not pass a failed read off as an address", () => {
    mocked.mockReturnValue({
      billingEmail: null,
      loading: false,
      error: new Error("network down"),
      saving: false,
      save: vi.fn(),
    });
    render(<BillingEmailCardView workspaceId="tea-a" />);
    expect(
      screen.getByText("The billing email could not be loaded."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("shows its skeleton while the first read is in flight", () => {
    mocked.mockReturnValue({
      billingEmail: null,
      loading: true,
      error: undefined,
      saving: false,
      save: vi.fn(),
    });
    render(<BillingEmailCardView workspaceId="tea-a" />);
    expect(screen.getByTestId("billing-email-skeleton")).toBeInTheDocument();
  });
});

// w9/m620: a Developer views billing details but does not change them.
describe("BillingEmailCard for a member who views but does not manage billing", () => {
  beforeEach(() => {
    mocked.mockReset();
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManageBilling: false }),
    );
  });
  afterEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  });

  it("shows the address with Edit held", () => {
    const save = billingEmail("finance@example.com", true);
    render(<BillingEmailCardView workspaceId="tea-a" />);
    expect(screen.getByText("finance@example.com")).toBeInTheDocument();
    const edit = screen.getByRole("button", { name: "Edit" });
    expect(edit).toBeDisabled();
    fireEvent.click(edit);
    expect(screen.queryByLabelText("Email address")).not.toBeInTheDocument();
    expect(save).not.toHaveBeenCalled();
  });
});

// The pending card keeps the ready card's header and the plan's own row: the
// note on Hobby, the Edit button above it (dashboard/AGENTS.md).
describe("BillingEmailCard skeleton", () => {
  it("reserves the Hobby note and no button on Hobby", () => {
    plan = "hobby";
    render(<BillingEmailCardSkeleton />);
    expect(screen.getByText("Billing email")).toBeInTheDocument();
    expect(screen.getByText(HOBBY_NOTE)).toBeInTheDocument();
    expect(screen.queryByText("Edit")).not.toBeInTheDocument();
  });

  it("reserves the Edit button and no note on Pro", () => {
    plan = "pro";
    render(<BillingEmailCardSkeleton />);
    expect(screen.getByText("Edit")).toBeInTheDocument();
    expect(screen.queryByText(HOBBY_NOTE)).not.toBeInTheDocument();
  });

  it("matches the ready card's structure on each plan", () => {
    for (const [p, configurable] of [
      ["hobby", false],
      ["pro", true],
    ] as const) {
      plan = p;
      billingEmail("owner@example.com", configurable);
      const ready = render(<BillingEmailCardView workspaceId="tea-a" />);
      const pending = render(<BillingEmailCardSkeleton />);
      // The card's own regions, less the bars that stand in for data and the
      // button a reserved bar of its own box stands in for.
      const shape = (root: HTMLElement) =>
        Array.from(root.querySelectorAll("[data-slot]"))
          .map((el) => el.getAttribute("data-slot"))
          .filter((slot) => slot !== "skeleton" && slot !== "button");
      expect(shape(pending.container)).toEqual(shape(ready.container));
      // Both keep the plan's own row: the note, or the Edit label.
      const row = (root: HTMLElement) =>
        root.querySelector('[data-slot="card-content"]')!.textContent;
      expect(row(pending.container)).toContain(
        configurable ? "Edit" : HOBBY_NOTE,
      );
      expect(row(ready.container)).toContain(
        configurable ? "Edit" : HOBBY_NOTE,
      );
      ready.unmount();
      pending.unmount();
    }
  });
});
