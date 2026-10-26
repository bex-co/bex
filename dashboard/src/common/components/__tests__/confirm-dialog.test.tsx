import { describe, it, expect, vi } from "vitest";
import { useState } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { ConfirmDialog } from "@/common/components/confirm-dialog";

// Each behavior is asserted once, here at the primitive. The migrated call
// sites keep their own tests for their own logic; re-asserting the dialog's
// mechanics 27 times would be the coverage-gaming the board forbids.
describe("ConfirmDialog", () => {
  it("opens from its trigger and confirms (uncontrolled)", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(
      <ConfirmDialog
        trigger={<button>Delete thing</button>}
        title="Delete thing?"
        description="This cannot be undone."
        confirmLabel="Delete"
        onConfirm={onConfirm}
      />,
    );

    // The dialog's contents must not exist before it is opened.
    expect(screen.queryByText("This cannot be undone.")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Delete thing" }));
    expect(await screen.findByText("This cannot be undone.")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Delete" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("renders open and reports dismissal (controlled)", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    render(
      <ConfirmDialog
        open
        onOpenChange={onOpenChange}
        title="Restore?"
        description="Everything after the snapshot is lost."
        confirmLabel="Restore"
        onConfirm={vi.fn()}
      />,
    );

    expect(screen.getByText("Everything after the snapshot is lost.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /cancel/i }));
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
  });

  it("keeps confirm disabled until the phrase matches exactly", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(
      <ConfirmDialog
        open
        title="Delete disk?"
        description="All data is lost."
        confirmLabel="Delete"
        phrase="sudo delete disk /var/data"
        onConfirm={onConfirm}
      />,
    );

    const confirm = screen.getByRole("button", { name: "Delete" });
    expect(confirm).toBeDisabled();

    // A near-miss must not arm it — one character short of the phrase.
    await user.type(screen.getByLabelText(/type/i), "sudo delete disk /var/dat");
    expect(confirm).toBeDisabled();
    expect(onConfirm).not.toHaveBeenCalled();

    await user.type(screen.getByLabelText(/type/i), "a");
    await waitFor(() => expect(confirm).toBeEnabled());
    await user.click(confirm);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("disables confirm while an action is in flight", () => {
    render(
      <ConfirmDialog
        open
        title="Suspend?"
        description="The service stops serving."
        confirmLabel="Suspend"
        pending
        onConfirm={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "Suspend" })).toBeDisabled();
  });

  it("renders extra content between the description and the footer", () => {
    render(
      <ConfirmDialog
        open
        title="Scale down?"
        description="Instances will be removed."
        confirmLabel="Scale"
        onConfirm={vi.fn()}
      >
        <p>Currently running 3 instances.</p>
      </ConfirmDialog>,
    );

    expect(screen.getByText("Currently running 3 instances.")).toBeInTheDocument();
  });
});

// w4/408: a controlled dialog opened from the caller's own button (no Radix
// trigger) returns focus to that button on close, instead of dropping it to
// the page top.
describe("ConfirmDialog focus return", () => {
  function Harness() {
    const [open, setOpen] = useState(false);
    return (
      <>
        <button onClick={() => setOpen(true)}>Suspend service</button>
        <ConfirmDialog
          open={open}
          onOpenChange={setOpen}
          title="Suspend service?"
          description="It stops serving."
          confirmLabel="Suspend"
          onConfirm={() => setOpen(false)}
        />
      </>
    );
  }

  it("returns focus to the opener on Escape", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const opener = screen.getByRole("button", { name: "Suspend service" });
    opener.focus();
    await user.keyboard("{Enter}");
    expect(await screen.findByRole("alertdialog")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    expect(document.activeElement).toBe(opener);
  });

  it("returns focus to the opener on Cancel", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const opener = screen.getByRole("button", { name: "Suspend service" });
    opener.focus();
    await user.keyboard("{Enter}");
    expect(await screen.findByRole("alertdialog")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /cancel/i }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    expect(document.activeElement).toBe(opener);
  });
});
