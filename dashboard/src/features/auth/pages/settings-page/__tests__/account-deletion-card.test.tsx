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
// The account's workspaces: the loading frame draws one row per workspace.
let workspaces: { id: string; name: string }[] = [];
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ workspaces }),
}));
vi.mock("@/common/lib/ory/logout", () => ({
  endBrowserSession: vi.fn(),
  clearBrowserAccountState,
}));

import { useMutation, useQuery } from "@apollo/client/react";
import { endBrowserSession } from "@/common/lib/ory/logout";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { hydrateAcrossBoundary } from "@/test/hydration";
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
    workspaces = [];
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

  // w4/686: the preview's loading frame draws the ready card's header, prompt,
  // field and button with the same copy, and one row per workspace.
  it("frames the ready card while the preview loads (w4/686)", () => {
    workspaces = [
      { id: "tea-shared", name: "shared" },
      { id: "tea-personal", name: "personal" },
      { id: "tea-team", name: "team" },
    ];
    preview({
      delete: [{ id: "tea-personal", name: "personal", action: "delete" }],
      leave: [
        { id: "tea-shared", name: "shared", action: "leave" },
        { id: "tea-team", name: "team", action: "leave" },
      ],
    });
    const ready = render(<AccountDeletionCard />);
    const readyTitle = screen.getByText("Delete account").textContent;
    const readyDescription = ready.container.querySelector(
      "[data-slot='card-description']",
    )!.textContent;
    const readyPrompt = ready.container.querySelector(
      "[data-slot='card-content'] p",
    )!.textContent;
    const readyLists = [
      ...ready.container.querySelectorAll("[data-slot='card-content'] ul"),
    ].map((list) => list.querySelectorAll("li").length);
    ready.unmount();

    vi.mocked(useQuery).mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch,
    } as never);
    const { container } = render(<AccountDeletionCard />);
    const frame = container.querySelector(
      "[data-testid='account-deletion-skeleton']",
    )!;
    const reserved = [...frame.querySelectorAll(".invisible")].map(
      (node) => node.textContent,
    );
    // Each row reserves its workspace's name and id (no separator in text).
    expect(reserved).toEqual([
      readyTitle,
      readyDescription,
      "personaltea-personal",
      "sharedtea-shared",
      "teamtea-team",
      readyPrompt,
      "Sudo Command",
      "Delete my account",
    ]);
    // Two lists, the rows in id order, as many as the ready preview lists.
    const lists = [...frame.querySelectorAll("ul")].map(
      (list) => list.querySelectorAll("li").length,
    );
    expect(lists).toEqual(readyLists);
    expect(frame.querySelector("input")).toBeNull();
    expect(frame.querySelector("button")).toBeNull();
  });

  // w4/686 reopened: the frame's prompt sits in a <p>, and a <div> bar inside
  // it is moved out by the HTML parser, so every direct /settings load threw
  // React #418. The server markup must hydrate as it is.
  it("hydrates the loading frame's server markup without a mismatch (w4/686)", () => {
    workspaces = [
      { id: "tea-shared", name: "shared" },
      { id: "tea-personal", name: "personal" },
    ];
    vi.mocked(useQuery).mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch,
    } as never);
    const now = Date.UTC(2026, 10, 16);
    const probe = hydrateAcrossBoundary(<AccountDeletionCard />, {
      serverNow: now,
      clientNow: now,
    });
    expect(probe.html).toContain("account-deletion-skeleton");
    expect(probe.recovered).toEqual([]);
  });
});
