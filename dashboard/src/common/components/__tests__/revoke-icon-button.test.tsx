import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RevokeIconButton } from "@/common/components/revoke-icon-button";

describe("RevokeIconButton", () => {
  it("confirming the dialog calls onConfirm", async () => {
    const onConfirm = vi.fn();
    const user = userEvent.setup();
    render(
      <RevokeIconButton
        label="Revoke"
        confirmTitle="Revoke this?"
        confirmBody="This can't be undone."
        cancelLabel="Cancel"
        confirmLabel="Revoke"
        onConfirm={onConfirm}
        pending={false}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Revoke" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText("Revoke this?")).toBeInTheDocument();
    await user.click(
      within(dialog).getAllByRole("button", { name: "Revoke" })[0],
    );

    expect(onConfirm).toHaveBeenCalled();
  });

  it("disables the trigger and shows a spinner while pending", () => {
    render(
      <RevokeIconButton
        label="Revoke"
        confirmTitle="Revoke this?"
        confirmBody="This can't be undone."
        cancelLabel="Cancel"
        confirmLabel="Revoke"
        onConfirm={vi.fn()}
        pending
      />,
    );

    expect(screen.getByRole("button", { name: "Revoke" })).toBeDisabled();
  });
});
