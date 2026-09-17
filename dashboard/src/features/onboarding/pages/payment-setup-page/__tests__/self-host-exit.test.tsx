import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@apollo/client/react", () => ({
  useMutation: vi.fn(),
}));
vi.mock("@/common/lib/ory/logout", () => ({
  endBrowserSession: vi.fn(),
  clearBrowserAccountState: vi.fn(),
}));

import { useMutation } from "@apollo/client/react";
import { endBrowserSession } from "@/common/lib/ory/logout";
import { SelfHostExit } from "../self-host-exit";

const SELF_HOST_URL = "https://github.com/bex-co/bex";
const PHRASE = "delete my account";

const assign = vi.fn();
const remove = vi.fn();

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useMutation).mockReturnValue([remove, { loading: false }] as never);
  vi.mocked(endBrowserSession).mockResolvedValue(undefined as never);
  remove.mockResolvedValue({});
  Object.defineProperty(window, "location", {
    configurable: true,
    value: { assign },
  });
});

function openDialog() {
  return userEvent.setup();
}

describe("SelfHostExit", () => {
  it("never deletes on a single click — the wall's button only opens the confirmation", async () => {
    const user = openDialog();
    render(<SelfHostExit />);

    await user.click(
      screen.getByRole("button", { name: "Delete this account and self-host" }),
    );

    expect(remove).not.toHaveBeenCalled();
    expect(assign).not.toHaveBeenCalled();
    expect(
      screen.getByRole("heading", {
        name: "Delete this account and self-host?",
      }),
    ).toBeInTheDocument();
  });

  it("keeps the confirm disabled until the phrase matches exactly", async () => {
    const user = openDialog();
    render(<SelfHostExit />);
    await user.click(
      screen.getByRole("button", { name: "Delete this account and self-host" }),
    );

    const confirm = screen.getByRole("button", {
      name: "Delete account and continue",
    });
    expect(confirm).toBeDisabled();

    await user.type(screen.getByRole("textbox"), "delete my accoun");
    expect(confirm).toBeDisabled();

    await user.type(screen.getByRole("textbox"), "t");
    expect(confirm).toBeEnabled();
  });

  it("deletes once, ends the session, then forwards to the self-hosting guide", async () => {
    const user = openDialog();
    render(<SelfHostExit />);
    await user.click(
      screen.getByRole("button", { name: "Delete this account and self-host" }),
    );
    await user.type(screen.getByRole("textbox"), PHRASE);

    await user.click(
      screen.getByRole("button", { name: "Delete account and continue" }),
    );

    await waitFor(() => expect(assign).toHaveBeenCalledWith(SELF_HOST_URL));
    expect(remove).toHaveBeenCalledOnce();
    expect(remove).toHaveBeenCalledWith({ variables: { confirmation: PHRASE } });
    expect(endBrowserSession).toHaveBeenCalledOnce();
  });

  it("stays put and reports the failure when deletion fails — never forwards as if it worked", async () => {
    remove.mockRejectedValue(new Error("nope"));
    const user = openDialog();
    render(<SelfHostExit />);
    await user.click(
      screen.getByRole("button", { name: "Delete this account and self-host" }),
    );
    await user.type(screen.getByRole("textbox"), PHRASE);

    await user.click(
      screen.getByRole("button", { name: "Delete account and continue" }),
    );

    await waitFor(() =>
      expect(screen.getByRole("alert")).toBeInTheDocument(),
    );
    expect(assign).not.toHaveBeenCalled();
    expect(endBrowserSession).not.toHaveBeenCalled();
    // Still retryable: the dialog stayed open with the phrase intact.
    expect(
      screen.getByRole("button", { name: "Delete account and continue" }),
    ).toBeEnabled();
  });

  it("leaves the account alone when dismissed", async () => {
    const user = openDialog();
    render(<SelfHostExit />);
    await user.click(
      screen.getByRole("button", { name: "Delete this account and self-host" }),
    );
    await user.type(screen.getByRole("textbox"), PHRASE);

    await user.click(screen.getByRole("button", { name: "Keep my account" }));

    expect(remove).not.toHaveBeenCalled();
    expect(assign).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(
        screen.queryByRole("heading", {
          name: "Delete this account and self-host?",
        }),
      ).not.toBeInTheDocument(),
    );
  });
});
