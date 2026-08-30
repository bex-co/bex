import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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
});
