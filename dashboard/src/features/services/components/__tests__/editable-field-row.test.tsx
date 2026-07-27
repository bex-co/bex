import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";

/** Minimal always-successful save handler. */
function ok() {
  return vi.fn(async () => true);
}

describe("EditableFieldRow", () => {
  it("renders the value in a disabled input with a pencil, no Cancel/Save", () => {
    render(
      <EditableFieldRow
        label="Service Name"
        value="docs-site"
        editLabel="Edit service name"
        onSave={ok()}
      />,
    );

    const input = screen.getByRole("textbox", { name: "Service Name" });
    expect(input).toHaveValue("docs-site");
    expect(input).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Edit service name" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Save changes" }),
    ).not.toBeInTheDocument();
  });

  it("pencil enables + focuses the same input and swaps in Cancel/Save (disabled until dirty)", async () => {
    const user = userEvent.setup();
    render(
      <EditableFieldRow
        label="Service Name"
        value="docs-site"
        editLabel="Edit service name"
        onSave={ok()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit service name" }));

    const input = screen.getByRole("textbox", { name: "Service Name" });
    expect(input).toBeEnabled();
    expect(input).toHaveFocus();
    expect(
      screen.queryByRole("button", { name: "Edit service name" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
    // Save is disabled until the draft differs from the current value.
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();

    await user.type(input, "-v2");
    expect(screen.getByRole("button", { name: "Save changes" })).toBeEnabled();
  });

  it("saves the trimmed draft, then returns to the disabled state", async () => {
    const user = userEvent.setup();
    const onSave = ok();
    render(
      <EditableFieldRow
        label="Service Name"
        value="docs-site"
        editLabel="Edit service name"
        onSave={onSave}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit service name" }));
    const input = screen.getByRole("textbox", { name: "Service Name" });
    await user.clear(input);
    await user.type(input, "  Customer API  ");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(onSave).toHaveBeenCalledWith("Customer API");
    // Back to view mode.
    expect(
      screen.getByRole("button", { name: "Edit service name" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", { name: "Service Name" }),
    ).toBeDisabled();
  });

  it("Cancel restores the original value and disabled state without saving", async () => {
    const user = userEvent.setup();
    const onSave = ok();
    render(
      <EditableFieldRow
        label="Service Name"
        value="docs-site"
        editLabel="Edit service name"
        onSave={onSave}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit service name" }));
    const input = screen.getByRole("textbox", { name: "Service Name" });
    await user.clear(input);
    await user.type(input, "throwaway");
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onSave).not.toHaveBeenCalled();
    const restored = screen.getByRole("textbox", { name: "Service Name" });
    expect(restored).toHaveValue("docs-site");
    expect(restored).toBeDisabled();
  });

  it("Escape cancels the edit like the Cancel button", async () => {
    const user = userEvent.setup();
    const onSave = ok();
    render(
      <EditableFieldRow
        label="Service Name"
        value="docs-site"
        editLabel="Edit service name"
        onSave={onSave}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit service name" }));
    const input = screen.getByRole("textbox", { name: "Service Name" });
    await user.type(input, "-x");
    await user.keyboard("{Escape}");

    expect(onSave).not.toHaveBeenCalled();
    expect(screen.getByRole("textbox", { name: "Service Name" })).toHaveValue(
      "docs-site",
    );
    expect(
      screen.getByRole("button", { name: "Edit service name" }),
    ).toBeInTheDocument();
  });

  it("runs the confirm dialog before onSave and aborts when it is dismissed", async () => {
    const user = userEvent.setup();
    const onSave = ok();
    render(
      <EditableFieldRow
        label="Root Directory"
        value="backend"
        editLabel="Edit Root Directory"
        confirm={{
          title: (v) => `Change Root Directory to ${v}?`,
          body: "This triggers a rebuild.",
        }}
        onSave={onSave}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Edit Root Directory" }),
    );
    const input = screen.getByRole("textbox", { name: "Root Directory" });
    await user.clear(input);
    await user.type(input, "web");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    // Save has NOT fired yet — the confirm dialog is up.
    expect(onSave).not.toHaveBeenCalled();
    const dialog = screen.getByRole("alertdialog");
    expect(
      within(dialog).getByText("Change Root Directory to web?"),
    ).toBeInTheDocument();

    // Dismissing the dialog aborts the save entirely.
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(onSave).not.toHaveBeenCalled();

    // Re-open and confirm this time.
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await user.click(
      within(screen.getByRole("alertdialog")).getByRole("button", {
        name: "Save changes",
      }),
    );
    expect(onSave).toHaveBeenCalledWith("web");
  });

  it("blocks save and shows the error while validation fails", async () => {
    const user = userEvent.setup();
    const onSave = ok();
    render(
      <EditableFieldRow
        label="Schedule"
        value="0 0 * * *"
        editLabel="Edit schedule"
        validate={(d) => (d.trim() === "" ? "Schedule is required." : null)}
        onSave={onSave}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    const input = screen.getByRole("textbox", { name: "Schedule" });
    await user.clear(input);

    expect(screen.getByText("Schedule is required.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(onSave).not.toHaveBeenCalled();
  });

  it("stays non-editable with no pencil when `disabled`", () => {
    render(
      <EditableFieldRow
        label="Custom URL"
        value="https://status.example.com"
        editLabel="Edit custom URL"
        disabled
        onSave={ok()}
      />,
    );

    expect(screen.getByRole("textbox", { name: "Custom URL" })).toBeDisabled();
    expect(
      screen.queryByRole("button", { name: "Edit custom URL" }),
    ).not.toBeInTheDocument();
  });

  it("combobox variant: disabled combobox, then edit → free-text → save (w5/m54)", async () => {
    const user = userEvent.setup();
    const onSave = ok();
    render(
      <EditableFieldRow
        label="Branch"
        value="main"
        editLabel="Edit branch"
        comboboxOptions={[
          { value: "main", label: "main" },
          { value: "dev", label: "dev" },
        ]}
        onSave={onSave}
      />,
    );

    const combo = screen.getByRole("combobox", { name: "Branch" });
    expect(combo).toBeDisabled();
    expect(combo).toHaveValue("main");

    await user.click(screen.getByRole("button", { name: "Edit branch" }));
    // allowCustom: a typed value that isn't in the option list still saves.
    await user.clear(combo);
    await user.type(combo, "feature-x");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(onSave).toHaveBeenCalledWith("feature-x");
  });

  it("select variant: disabled select, then edit → pick → save", async () => {
    const user = userEvent.setup();
    const onSave = ok();
    render(
      <EditableFieldRow
        label="Notifications"
        value="default"
        editLabel="Edit notifications"
        options={[
          { value: "default", label: "Use workspace default" },
          { value: "none", label: "No notifications" },
        ]}
        onSave={onSave}
      />,
    );

    const trigger = screen.getByRole("combobox", { name: "Notifications" });
    expect(trigger).toBeDisabled();
    expect(trigger).toHaveTextContent("Use workspace default");

    await user.click(
      screen.getByRole("button", { name: "Edit notifications" }),
    );
    await user.click(screen.getByRole("combobox", { name: "Notifications" }));
    await user.click(screen.getByRole("option", { name: "No notifications" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(onSave).toHaveBeenCalledWith("none");
  });
});
