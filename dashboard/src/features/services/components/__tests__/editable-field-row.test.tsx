import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
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

  // w4/219: the title quotes the whole new value, so a long (valid) command
  // must scroll inside a viewport-capped dialog instead of pushing Save and
  // Cancel off a mobile screen. jsdom can't measure layout; this pins the cap
  // on the confirmation path, and the live 390x667 check covers geometry.
  it("caps the confirmation to the viewport and scrolls it internally", async () => {
    const user = userEvent.setup();
    const longCommand = `exec httpd ${"-f -p 3000 ".repeat(80)}`.trim();
    render(
      <EditableFieldRow
        label="Docker Command"
        value="old"
        editLabel="Edit Docker Command"
        confirm={{
          title: (v) => `Change Docker Command to ${v}?`,
          body: "This redeploys.",
        }}
        onSave={ok()}
      />,
    );
    await user.click(
      screen.getByRole("button", { name: "Edit Docker Command" }),
    );
    const input = screen.getByRole("textbox", { name: "Docker Command" });
    await user.clear(input);
    await user.click(input);
    await user.paste(longCommand);
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    const dialog = screen.getByRole("alertdialog");
    expect(dialog).toHaveClass("max-h-[85dvh]", "overflow-y-auto");
    // The full value is still shown, not truncated.
    expect(
      within(dialog).getByText(`Change Docker Command to ${longCommand}?`),
    ).toBeInTheDocument();
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

  describe("select variant Escape (w4/250)", () => {
    const options = [
      { value: "default", label: "Use workspace default" },
      { value: "none", label: "No notifications" },
    ];

    function renderSelect(onSave = ok()) {
      return render(
        <EditableFieldRow
          label="Notifications"
          value="default"
          editLabel="Edit notifications"
          options={options}
          onSave={onSave}
        />,
      );
    }

    it("Escape on the closed trigger discards the picked draft without saving", async () => {
      const user = userEvent.setup();
      const onSave = ok();
      renderSelect(onSave);

      await user.click(
        screen.getByRole("button", { name: "Edit notifications" }),
      );
      await user.click(screen.getByRole("combobox", { name: "Notifications" }));
      await user.click(
        screen.getByRole("option", { name: "No notifications" }),
      );
      const trigger = screen.getByRole("combobox", { name: "Notifications" });
      expect(trigger).toHaveTextContent("No notifications");
      expect(trigger).toHaveFocus();

      await user.keyboard("{Escape}");

      expect(trigger).toHaveTextContent("Use workspace default");
      expect(trigger).toBeDisabled();
      expect(
        screen.getByRole("button", { name: "Edit notifications" }),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Save changes" }),
      ).not.toBeInTheDocument();
      expect(onSave).not.toHaveBeenCalled();
    });

    it("first Escape closes an open popup and keeps editing; the next cancels", async () => {
      const user = userEvent.setup();
      const onSave = ok();
      renderSelect(onSave);

      await user.click(
        screen.getByRole("button", { name: "Edit notifications" }),
      );
      const trigger = screen.getByRole("combobox", { name: "Notifications" });
      await user.click(trigger);
      expect(screen.getByRole("listbox")).toBeInTheDocument();

      await user.keyboard("{Escape}");

      expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
      expect(trigger).toBeEnabled();
      expect(
        screen.getByRole("button", { name: "Save changes" }),
      ).toBeInTheDocument();

      trigger.focus();
      await user.keyboard("{Escape}");

      expect(trigger).toBeDisabled();
      expect(
        screen.getByRole("button", { name: "Edit notifications" }),
      ).toBeInTheDocument();
      expect(onSave).not.toHaveBeenCalled();
    });

    it("Escape does not cancel while a save is in flight", async () => {
      const user = userEvent.setup();
      const { rerender } = renderSelect();

      await user.click(
        screen.getByRole("button", { name: "Edit notifications" }),
      );
      rerender(
        <EditableFieldRow
          label="Notifications"
          value="default"
          editLabel="Edit notifications"
          options={options}
          busy
          onSave={ok()}
        />,
      );
      fireEvent.keyDown(
        screen.getByRole("combobox", { name: "Notifications" }),
        { key: "Escape" },
      );

      expect(
        screen.getByRole("button", { name: "Save changes" }),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Edit notifications" }),
      ).not.toBeInTheDocument();
    });
  });

  // Geometry itself is verified in a real browser (jsdom has no layout); this
  // pins the classes that let a long label wrap instead of pushing the pencil
  // and Cancel/Save off a 360–390px screen (w4/251).
  it("select variant lets a long label shrink and wrap, and drops actions below it while editing on mobile", async () => {
    const user = userEvent.setup();
    render(
      <EditableFieldRow
        label="Notifications"
        value="default"
        editLabel="Edit notifications"
        options={[
          {
            value: "default",
            label: "Use workspace default (Only failure notifications)",
          },
        ]}
        onSave={ok()}
      />,
    );

    const trigger = screen.getByRole("combobox", { name: "Notifications" });
    expect(trigger).toHaveClass(
      "min-w-0",
      "whitespace-normal",
      "data-[size=sm]:h-auto",
    );
    expect(trigger).not.toHaveClass("whitespace-nowrap");
    expect(trigger.parentElement).toHaveClass("flex-wrap", "sm:flex-nowrap");
    expect(trigger).not.toHaveClass("basis-full");

    await user.click(
      screen.getByRole("button", { name: "Edit notifications" }),
    );
    expect(trigger).toHaveClass("basis-full", "sm:basis-0");
  });

  describe("cancel returns keyboard focus to the pencil (w4/293)", () => {
    it("Escape on a text field focuses Edit, and Enter reopens the saved value", async () => {
      const user = userEvent.setup();
      const onSave = ok();
      render(
        <EditableFieldRow
          label="Health Check Path"
          value="/healthz"
          editLabel="Edit health check path"
          onSave={onSave}
        />,
      );

      screen.getByRole("button", { name: "Edit health check path" }).focus();
      await user.keyboard("{Enter}");
      const input = screen.getByRole("textbox", { name: "Health Check Path" });
      expect(input).toHaveFocus();
      await user.type(input, "/draft");
      await user.keyboard("{Escape}");

      expect(input).toHaveValue("/healthz");
      expect(input).toBeDisabled();
      const pencil = screen.getByRole("button", {
        name: "Edit health check path",
      });
      expect(pencil).toHaveFocus();

      await user.keyboard("{Enter}");
      expect(input).toBeEnabled();
      expect(input).toHaveValue("/healthz");
      expect(input).toHaveFocus();
      expect(onSave).not.toHaveBeenCalled();
    });

    it("Escape on a number field focuses Edit, and Enter reopens it", async () => {
      const user = userEvent.setup();
      render(
        <EditableFieldRow
          label="Port"
          value="8846"
          editLabel="Edit port"
          type="number"
          onSave={ok()}
        />,
      );

      screen.getByRole("button", { name: "Edit port" }).focus();
      await user.keyboard("{Enter}");
      const input = screen.getByRole("spinbutton", { name: "Port" });
      await user.clear(input);
      await user.type(input, "8847");
      await user.keyboard("{Escape}");

      expect(input).toHaveValue(8846);
      expect(screen.getByRole("button", { name: "Edit port" })).toHaveFocus();
      await user.keyboard("{Enter}");
      expect(input).toBeEnabled();
      expect(input).toHaveFocus();
    });

    it("keyboard activation of Cancel focuses Edit", async () => {
      const user = userEvent.setup();
      render(
        <EditableFieldRow
          label="Health Check Path"
          value="/healthz"
          editLabel="Edit health check path"
          onSave={ok()}
        />,
      );

      await user.click(
        screen.getByRole("button", { name: "Edit health check path" }),
      );
      await user.type(
        screen.getByRole("textbox", { name: "Health Check Path" }),
        "/draft",
      );
      await user.tab();
      expect(screen.getByRole("button", { name: "Cancel" })).toHaveFocus();
      await user.keyboard("{Enter}");

      expect(
        screen.getByRole("button", { name: "Edit health check path" }),
      ).toHaveFocus();
    });

    it("select: popup Escape keeps editing; explicit Cancel focuses Edit", async () => {
      const user = userEvent.setup();
      render(
        <EditableFieldRow
          label="Notifications"
          value="default"
          editLabel="Edit notifications"
          options={[
            { value: "default", label: "Use workspace default" },
            { value: "none", label: "No notifications" },
          ]}
          onSave={ok()}
        />,
      );

      await user.click(
        screen.getByRole("button", { name: "Edit notifications" }),
      );
      const trigger = screen.getByRole("combobox", { name: "Notifications" });
      await user.click(trigger);
      await user.keyboard("{Escape}");
      expect(trigger).toBeEnabled();
      expect(
        screen.queryByRole("button", { name: "Edit notifications" }),
      ).not.toBeInTheDocument();

      screen.getByRole("button", { name: "Cancel" }).focus();
      await user.keyboard("{Enter}");
      expect(trigger).toBeDisabled();
      expect(
        screen.getByRole("button", { name: "Edit notifications" }),
      ).toHaveFocus();
    });

    it("combobox: explicit Cancel focuses Edit", async () => {
      const user = userEvent.setup();
      render(
        <EditableFieldRow
          label="Branch"
          value="main"
          editLabel="Edit branch"
          comboboxOptions={[{ value: "main", label: "main" }]}
          onSave={ok()}
        />,
      );

      await user.click(screen.getByRole("button", { name: "Edit branch" }));
      await user.click(screen.getByRole("button", { name: "Cancel" }));

      expect(screen.getByRole("combobox", { name: "Branch" })).toHaveValue(
        "main",
      );
      expect(screen.getByRole("button", { name: "Edit branch" })).toHaveFocus();
    });

    it("never moves focus on mount, on save, or into a row that became disabled", async () => {
      const user = userEvent.setup();
      const props = {
        label: "Health Check Path",
        value: "/healthz",
        editLabel: "Edit health check path",
        onSave: ok(),
      };
      const { rerender } = render(
        <>
          <button type="button">Elsewhere</button>
          <EditableFieldRow {...props} />
        </>,
      );
      expect(
        screen.getByRole("button", { name: "Edit health check path" }),
      ).not.toHaveFocus();

      // A successful save is not a cancellation: no focus hand-off.
      await user.click(
        screen.getByRole("button", { name: "Edit health check path" }),
      );
      await user.type(
        screen.getByRole("textbox", { name: "Health Check Path" }),
        "/v2",
      );
      await user.click(screen.getByRole("button", { name: "Save changes" }));
      expect(props.onSave).toHaveBeenCalledWith("/healthz/v2");
      expect(
        screen.getByRole("button", { name: "Edit health check path" }),
      ).not.toHaveFocus();

      // Cancel on a row that lost edit rights renders no pencil, so nothing
      // is focused, and re-enabling it later does not steal focus.
      await user.click(
        screen.getByRole("button", { name: "Edit health check path" }),
      );
      rerender(
        <>
          <button type="button">Elsewhere</button>
          <EditableFieldRow {...props} disabled />
        </>,
      );
      await user.click(screen.getByRole("button", { name: "Cancel" }));
      expect(
        screen.queryByRole("button", { name: "Edit health check path" }),
      ).not.toBeInTheDocument();
      screen.getByRole("button", { name: "Elsewhere" }).focus();
      rerender(
        <>
          <button type="button">Elsewhere</button>
          <EditableFieldRow {...props} />
        </>,
      );
      expect(screen.getByRole("button", { name: "Elsewhere" })).toHaveFocus();
    });
  });
});
