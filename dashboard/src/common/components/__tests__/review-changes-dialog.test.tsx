import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  ReviewChangesDialog,
  type ReviewChangesDialogProps,
} from "@/common/components/review-changes-dialog";

function renderReview(props: Partial<ReviewChangesDialogProps> = {}) {
  const onConfirm = vi.fn();
  const onClose = vi.fn();
  const onEdit = vi.fn();
  render(
    <ReviewChangesDialog
      open
      description="Review your compute and storage selections."
      lineItems={[
        {
          id: "plan",
          label: "Compute plan",
          values: ["1 CPU", "4 GB RAM"],
          price: "$55.00/month",
          onEdit,
          editLabel: "Edit compute plan",
        },
        {
          id: "replicas",
          label: "Read replicas",
          values: ["1 replica"],
          price: "$55.00/month",
        },
      ]}
      total="$110.00/month"
      confirmLabel="Update compute"
      onConfirm={onConfirm}
      onClose={onClose}
      {...props}
    />,
  );
  return {
    dialog: screen.getByRole("alertdialog"),
    onConfirm,
    onClose,
    onEdit,
  };
}

// w3/m674: Render's one review component (`BN`), on the dashboard's
// ConfirmDialog.
describe("ReviewChangesDialog", () => {
  it("lists each line with its figures and price, then the total", () => {
    const { dialog } = renderReview({
      warning: "Your database will be unavailable for a few minutes.",
    });
    expect(dialog).toHaveAccessibleName("Apply changes?");
    expect(dialog).toHaveAccessibleDescription(
      "Review your compute and storage selections.",
    );
    const rows = [...dialog.querySelectorAll('[data-slot="review-line"]')].map(
      (row) => [...row.children].map((cell) => cell.textContent),
    );
    expect(rows).toEqual([
      ["Compute plan", "1 CPU4 GB RAM", "$55.00/month"],
      ["Read replicas", "1 replica", "$55.00/month"],
    ]);
    expect(dialog).toHaveTextContent("Total$110.00/month");
    expect(dialog).toHaveTextContent(
      "Your database will be unavailable for a few minutes.",
    );
    expect(within(dialog).queryByRole("alert")).toBeNull();
  });

  it("puts Edit only beside a line that has one", async () => {
    const user = userEvent.setup();
    const { dialog, onEdit } = renderReview();
    const edits = within(dialog).getAllByRole("button", { name: /^Edit/ });
    expect(edits).toHaveLength(1);
    await user.click(edits[0]);
    expect(onEdit).toHaveBeenCalledOnce();
  });

  it("confirms with the change's own button, and closes from Cancel", async () => {
    const user = userEvent.setup();
    const { dialog, onConfirm, onClose } = renderReview();
    await user.click(
      within(dialog).getByRole("button", { name: "Update compute" }),
    );
    expect(onConfirm).toHaveBeenCalledOnce();
    // The caller closes it once the change applied.
    expect(onClose).not.toHaveBeenCalled();
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("shows a failed confirm's error in place", () => {
    const { dialog } = renderReview({ error: "Payment method required." });
    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      "Payment method required.",
    );
  });

  it("holds every way out while the change applies", async () => {
    const user = userEvent.setup();
    const { dialog, onClose } = renderReview({ pending: true });
    expect(
      within(dialog).getByRole("button", { name: "Update compute" }),
    ).toBeDisabled();
    expect(
      within(dialog).getByRole("button", { name: "Cancel" }),
    ).toBeDisabled();
    expect(
      within(dialog).getByRole("button", { name: "Edit compute plan" }),
    ).toBeDisabled();
    await user.keyboard("{Escape}");
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
  });

  it("waits for the typed phrase when the change deletes data", async () => {
    const user = userEvent.setup();
    const { dialog, onConfirm } = renderReview({
      phrase: "sudo clear key value sessions",
    });
    const confirm = within(dialog).getByRole("button", {
      name: "Update compute",
    });
    expect(confirm).toBeDisabled();
    await user.type(
      within(dialog).getByRole("textbox"),
      "sudo clear key value sessions",
    );
    expect(confirm).toBeEnabled();
    await user.click(confirm);
    expect(onConfirm).toHaveBeenCalledOnce();
  });
});
