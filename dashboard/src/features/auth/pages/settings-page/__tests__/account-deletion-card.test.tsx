import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { clearBrowserAccountState } = vi.hoisted(() => ({
  clearBrowserAccountState: vi.fn(),
}));

vi.mock("@apollo/client/react", () => ({
  useQuery: vi.fn(),
  useMutation: vi.fn(),
}));
vi.mock("@/common/lib/ory/logout", () => ({
  endBrowserSession: vi.fn(),
  clearBrowserAccountState,
}));

import { useMutation, useQuery } from "@apollo/client/react";
import { endBrowserSession } from "@/common/lib/ory/logout";
import { codedGraphQLError } from "@/test/mocks/apollo";
import {
  AccountDeletionCard,
  accountDeletionConfirmation,
} from "../account-deletion-card";

const mutate = vi.fn();
const refetch = vi.fn();

function preview(overrides?: {
  delete?: { id: string; name: string; action: string }[];
  leave?: { id: string; name: string; action: string }[];
  blocked?: { id: string; name: string; action: string }[];
}) {
  vi.mocked(useQuery).mockReturnValue({
    data: {
      accountDeletionPreview: {
        __typename: "AccountDeletionPreview",
        delete: overrides?.delete ?? [],
        leave: overrides?.leave ?? [],
        blocked: overrides?.blocked ?? [],
      },
    },
    loading: false,
    error: undefined,
    refetch,
  } as never);
}

describe("AccountDeletionCard", () => {
  beforeEach(() => {
    mutate.mockReset();
    refetch.mockReset();
    vi.mocked(useMutation).mockReturnValue([
      mutate,
      { loading: false },
    ] as never);
    clearBrowserAccountState.mockReset().mockResolvedValue(undefined);
    vi.mocked(endBrowserSession).mockReset();
    vi.stubGlobal("location", { assign: vi.fn() });
  });

  it("names delete and leave workspaces and requires the exact phrase", async () => {
    preview({
      delete: [{ id: "tea-personal", name: "personal", action: "delete" }],
      leave: [{ id: "tea-shared", name: "shared", action: "leave" }],
    });
    render(<AccountDeletionCard />);

    expect(screen.getByText("personal")).toBeInTheDocument();
    expect(screen.getByText("shared")).toBeInTheDocument();
    const submit = screen.getByRole("button", { name: "Delete my account" });
    expect(submit).toBeDisabled();

    await userEvent.type(
      screen.getByLabelText("Sudo Command"),
      "delete account",
    );
    expect(submit).toBeDisabled();
    await userEvent.clear(screen.getByLabelText("Sudo Command"));
    await userEvent.type(
      screen.getByLabelText("Sudo Command"),
      accountDeletionConfirmation,
    );
    expect(submit).toBeEnabled();
  });

  it("keeps deletion disabled and gives actionable blocker details", async () => {
    preview({
      blocked: [{ id: "tea-blocked", name: "orphan-risk", action: "blocked" }],
    });
    render(<AccountDeletionCard />);

    expect(
      screen.getByText("Resolve workspace ownership first"),
    ).toBeInTheDocument();
    expect(screen.getByText("orphan-risk")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open settings" })).toHaveAttribute(
      "href",
      "/w/tea-blocked/settings",
    );
    await userEvent.type(
      screen.getByLabelText("Sudo Command"),
      accountDeletionConfirmation,
    );
    expect(
      screen.getByRole("button", { name: "Delete my account" }),
    ).toBeDisabled();
  });

  it("clears local account state and reaches the terminal page after acceptance", async () => {
    preview({
      delete: [{ id: "tea-personal", name: "personal", action: "delete" }],
    });
    mutate.mockResolvedValue({ data: { deleteAccount: { state: "pending" } } });
    vi.mocked(endBrowserSession).mockRejectedValue(
      new Error("Kratos already unavailable"),
    );
    render(<AccountDeletionCard />);

    await userEvent.type(
      screen.getByLabelText("Sudo Command"),
      accountDeletionConfirmation,
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Delete my account" }),
    );

    expect(mutate).toHaveBeenCalledTimes(1);
    expect(clearBrowserAccountState).toHaveBeenCalledTimes(1);
    expect(window.location.assign).toHaveBeenCalledWith(
      "/auth/account-deleted",
    );
  });

  it("admits only one mutation while an accepted click is in flight", async () => {
    preview({
      delete: [{ id: "tea-personal", name: "personal", action: "delete" }],
    });
    let resolveMutation!: (value: {
      data: { deleteAccount: { state: string } };
    }) => void;
    mutate.mockReturnValue(
      new Promise((resolve) => {
        resolveMutation = resolve;
      }),
    );
    vi.mocked(endBrowserSession).mockResolvedValue(undefined);
    render(<AccountDeletionCard />);

    await userEvent.type(
      screen.getByLabelText("Sudo Command"),
      accountDeletionConfirmation,
    );
    const submit = screen.getByRole("button", { name: "Delete my account" });
    fireEvent.click(submit);
    fireEvent.click(submit);

    expect(mutate).toHaveBeenCalledTimes(1);
    expect(submit).toBeDisabled();
    resolveMutation({ data: { deleteAccount: { state: "pending" } } });
    await waitFor(() =>
      expect(window.location.assign).toHaveBeenCalledWith(
        "/auth/account-deleted",
      ),
    );
  });

  it("surfaces a safe error and permits a retry when the mutation fails", async () => {
    preview({
      delete: [{ id: "tea-personal", name: "personal", action: "delete" }],
    });
    mutate.mockRejectedValue(new Error("dependency unavailable"));
    render(<AccountDeletionCard />);

    await userEvent.type(
      screen.getByLabelText("Sudo Command"),
      accountDeletionConfirmation,
    );
    const submit = screen.getByRole("button", { name: "Delete my account" });
    await userEvent.click(submit);

    expect(
      screen.getByText("Account deletion could not start"),
    ).toBeInTheDocument();
    expect(submit).toBeEnabled();
    await userEvent.click(submit);
    expect(mutate).toHaveBeenCalledTimes(2);
  });

  it("names the workspace with an unpaid balance and links its invoice", async () => {
    preview({
      delete: [{ id: "tea-personal", name: "personal", action: "delete" }],
    });
    mutate.mockRejectedValue(
      codedGraphQLError("BILLING_BALANCE_DUE", {
        workspaceId: "tea-personal",
        amountDueCents: 1250,
        currency: "usd",
        hostedInvoiceUrl: "https://invoice.stripe.com/i/acct_1/in_2",
      }),
    );
    render(<AccountDeletionCard />);

    await userEvent.type(
      screen.getByLabelText("Sudo Command"),
      accountDeletionConfirmation,
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Delete my account" }),
    );

    expect(
      screen.getByText(
        "The workspace personal has an unpaid balance of $12.50. Pay the invoice, then delete your account again.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Pay invoice/ })).toHaveAttribute(
      "href",
      "https://invoice.stripe.com/i/acct_1/in_2",
    );
    expect(screen.queryByText("Account deletion could not start")).toBeNull();
    expect(window.location.assign).not.toHaveBeenCalled();
    expect(
      screen.getByRole("button", { name: "Delete my account" }),
    ).toBeEnabled();
  });

  // ADR091 D10.4: Stripe can take a minute to aggregate the final usage, so a
  // BILLING_SETTLING refusal shows "Finalizing final usage…" and retries every
  // 15 s for up to 3 minutes before it is shown as an error.
  it("retries BILLING_SETTLING while finalizing final usage", async () => {
    preview({
      delete: [{ id: "tea-personal", name: "personal", action: "delete" }],
    });
    vi.mocked(endBrowserSession).mockResolvedValue(undefined);
    mutate
      .mockRejectedValueOnce(codedGraphQLError("BILLING_SETTLING"))
      .mockResolvedValue({ data: { deleteAccount: { state: "PENDING" } } });
    render(<AccountDeletionCard />);
    fireEvent.change(screen.getByLabelText("Sudo Command"), {
      target: { value: accountDeletionConfirmation },
    });

    vi.useFakeTimers();
    try {
      await act(async () => {
        fireEvent.click(
          screen.getByRole("button", { name: "Delete my account" }),
        );
      });
      expect(screen.getByText("Finalizing final usage…")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Delete my account" }),
      ).toBeDisabled();
      await act(async () => {
        await vi.advanceTimersByTimeAsync(15_000);
      });
      expect(mutate).toHaveBeenCalledTimes(2);
      expect(window.location.assign).toHaveBeenCalledWith(
        "/auth/account-deleted",
      );
    } finally {
      vi.useRealTimers();
    }
  });

  it("shows the retry message after 3 minutes of BILLING_SETTLING", async () => {
    preview({
      delete: [{ id: "tea-personal", name: "personal", action: "delete" }],
    });
    mutate.mockRejectedValue(codedGraphQLError("BILLING_SETTLING"));
    render(<AccountDeletionCard />);
    fireEvent.change(screen.getByLabelText("Sudo Command"), {
      target: { value: accountDeletionConfirmation },
    });

    vi.useFakeTimers();
    try {
      await act(async () => {
        fireEvent.click(
          screen.getByRole("button", { name: "Delete my account" }),
        );
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(180_000);
      });
      expect(mutate).toHaveBeenCalledTimes(13);
      expect(screen.queryByText("Finalizing final usage…")).toBeNull();
      expect(
        screen.getByText(
          "The final usage is still being processed. Try deleting again in a few minutes.",
        ),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Delete my account" }),
      ).toBeEnabled();
      expect(window.location.assign).not.toHaveBeenCalled();
    } finally {
      vi.useRealTimers();
    }
  });
});
