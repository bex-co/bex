import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { IPAllowListEditor } from "@/common/components/ip-allow-list-editor";
import type { IPAllowListEntryDraft } from "@/common/lib/ip-allow-list";

const labels = {
  hint: "Only these CIDRs may reach the endpoint.",
  open: "Open to all source IPs.",
  descriptionPlaceholder: "Description (optional)",
  cidr: "New CIDR block",
  cidrRule: (number: number) => `CIDR block for rule ${number}`,
  description: "New rule description",
  descriptionRule: (number: number) => `Description for rule ${number}`,
  add: "Add",
  save: "Save allowlist",
  invalid: "Enter a valid CIDR.",
  remove: (cidr: string) => `Remove ${cidr}`,
  moveUp: (cidr: string) => `Move up ${cidr}`,
  moveDown: (cidr: string) => `Move down ${cidr}`,
};

describe("IPAllowListEditor", () => {
  it("names add-row and existing-row inputs (not by placeholder example) (m101/t010)", () => {
    render(
      <IPAllowListEditor
        entries={[{ cidrBlock: "10.0.0.0/8", description: "corp" }]}
        labels={labels}
        saving={false}
        onSave={vi.fn(async () => true)}
      />,
    );

    expect(
      screen.getByRole("textbox", { name: "New CIDR block" }),
    ).toHaveAttribute("placeholder", "203.0.113.0/24");
    expect(
      screen.getByRole("textbox", { name: "New rule description" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", { name: "CIDR block for rule 1" }),
    ).toHaveValue("10.0.0.0/8");
    expect(
      screen.getByRole("textbox", { name: "Description for rule 1" }),
    ).toHaveValue("corp");
    // Placeholder text alone must not be the accessible name.
    expect(
      screen.queryByRole("textbox", { name: "203.0.113.0/24" }),
    ).not.toBeInTheDocument();
  });
});

// w4/135: the row key was `${index}-${entry.cidrBlock}` — the editable value
// itself — so the first CIDR keystroke changed the row's identity, React
// deleted the row's DOM subtree, and the focused input went with it. Live,
// Backspace left `203.0.113.0/2` with document.activeElement on BODY, and the
// following `4` never arrived. Description edits never touched the key, which
// is why they worked and made the failure look field-specific.
//
// These use SEQUENTIAL keyboard input, not a one-shot fill: a single
// change event cannot observe focus loss, which is why the existing coverage
// missed this.
describe("IPAllowListEditor row identity", () => {
  function renderEditor(
    entries: IPAllowListEntryDraft[],
    onSave = vi.fn(async (_entries: IPAllowListEntryDraft[]) => true),
  ) {
    render(
      <IPAllowListEditor
        entries={entries}
        labels={labels}
        saving={false}
        onSave={onSave}
      />,
    );
    return onSave;
  }

  it("keeps focus and accepts continuous typing in a server-seeded row's CIDR", async () => {
    const user = userEvent.setup();
    renderEditor([{ cidrBlock: "203.0.113.0/24", description: "office" }]);

    const input = screen.getByRole("textbox", {
      name: "CIDR block for rule 1",
    });
    await user.click(input);
    await user.keyboard("{End}{Backspace}");

    // The exact live observation: mid-edit value, and the SAME element still
    // focused — it used to be replaced, leaving activeElement on BODY.
    expect(input).toHaveValue("203.0.113.0/2");
    expect(input).toHaveFocus();
    expect(document.activeElement).toBe(input);

    await user.keyboard("4");
    expect(input).toHaveValue("203.0.113.0/24");
    expect(input).toHaveFocus();
  });

  it("keeps focus through a whole retyped CIDR, one keystroke at a time", async () => {
    const user = userEvent.setup();
    renderEditor([{ cidrBlock: "10.0.0.0/8", description: "corp" }]);

    const input = screen.getByRole("textbox", {
      name: "CIDR block for rule 1",
    });
    await user.clear(input);
    await user.type(input, "192.168.0.0/16");

    expect(input).toHaveValue("192.168.0.0/16");
    expect(input).toHaveFocus();
    // The pairing survives: the description belongs to the same row.
    expect(
      screen.getByRole("textbox", { name: "Description for rule 1" }),
    ).toHaveValue("corp");
  });

  it("keeps focus in a freshly added row, not only a seeded one", async () => {
    const user = userEvent.setup();
    renderEditor([]);

    await user.type(
      screen.getByRole("textbox", { name: "New CIDR block" }),
      "203.0.113.0/24",
    );
    await user.click(screen.getByRole("button", { name: "Add" }));

    const input = screen.getByRole("textbox", {
      name: "CIDR block for rule 1",
    });
    await user.click(input);
    await user.keyboard("{End}{Backspace}4");
    expect(input).toHaveValue("203.0.113.0/24");
    expect(input).toHaveFocus();
  });

  it("stays editable and focused while an invalid or duplicate CIDR blocks Save", async () => {
    const user = userEvent.setup();
    renderEditor([
      { cidrBlock: "10.0.0.0/8", description: "a" },
      { cidrBlock: "192.168.0.0/16", description: "b" },
    ]);

    const second = screen.getByRole("textbox", {
      name: "CIDR block for rule 2",
    });
    await user.clear(second);
    await user.type(second, "10.0.0.0/8");

    // Duplicate: Save is blocked, the row keeps the caret so it can be fixed.
    expect(second).toHaveValue("10.0.0.0/8");
    expect(second).toHaveFocus();
    expect(
      screen.getByRole("button", { name: "Save allowlist" }),
    ).toBeDisabled();
    expect(screen.getByRole("alert")).toBeInTheDocument();

    await user.type(second, "0");
    expect(second).toHaveValue("10.0.0.0/80");
    expect(second).toHaveFocus();
  });

  it("keeps each row's identity across reorder and neighbour removal", async () => {
    const user = userEvent.setup();
    const onSave = renderEditor([
      { cidrBlock: "10.0.0.0/8", description: "first" },
      { cidrBlock: "172.16.0.0/12", description: "second" },
      { cidrBlock: "192.168.0.0/16", description: "third" },
    ]);

    await user.click(
      screen.getByRole("button", { name: "Move down 10.0.0.0/8" }),
    );
    await user.click(
      screen.getByRole("button", { name: "Remove 192.168.0.0/16" }),
    );

    // Edit the moved row after the reorder — the case a positional key breaks.
    const moved = screen.getByRole("textbox", {
      name: "CIDR block for rule 2",
    });
    await user.clear(moved);
    await user.type(moved, "10.1.0.0/16");
    expect(moved).toHaveFocus();

    await user.click(screen.getByRole("button", { name: "Save allowlist" }));

    // Order preserved, pairings intact, and the payload is the plain public
    // shape — no editor-local identity leaks into the GraphQL variables.
    expect(onSave).toHaveBeenCalledWith([
      { cidrBlock: "172.16.0.0/12", description: "second" },
      { cidrBlock: "10.1.0.0/16", description: "first" },
    ]);
    const saved = onSave.mock.lastCall?.[0] ?? [];
    for (const entry of saved) {
      expect(Object.keys(entry).sort()).toEqual(["cidrBlock", "description"]);
    }
  });

  it("submits [] for an intentionally emptied list", async () => {
    const user = userEvent.setup();
    const onSave = renderEditor([
      { cidrBlock: "10.0.0.0/8", description: "x" },
    ]);

    await user.click(screen.getByRole("button", { name: "Remove 10.0.0.0/8" }));
    await user.click(screen.getByRole("button", { name: "Save allowlist" }));

    expect(onSave).toHaveBeenCalledWith([]);
  });

  it("is clean initially and again once an edit is reverted", async () => {
    const user = userEvent.setup();
    renderEditor([{ cidrBlock: "10.0.0.0/8", description: "corp" }]);

    const save = screen.getByRole("button", { name: "Save allowlist" });
    expect(save).toBeDisabled();

    const input = screen.getByRole("textbox", {
      name: "CIDR block for rule 1",
    });
    await user.click(input);
    await user.keyboard("{End}{Backspace}");
    // Mid-edit the value is "10.0.0.0/" — dirty, but not a valid CIDR, so Save
    // stays disabled by validation rather than by cleanliness. The row is still
    // editable and focused, which is the property that matters here.
    expect(save).toBeDisabled();
    expect(input).toHaveFocus();

    await user.keyboard("9");
    expect(input).toHaveValue("10.0.0.0/9");
    expect(save).toBeEnabled();

    await user.keyboard("{Backspace}8");
    expect(input).toHaveValue("10.0.0.0/8");
    expect(save).toBeDisabled();
  });

  it("keeps the draft after a failed Save", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn(async (_entries: IPAllowListEntryDraft[]) => false);
    renderEditor([{ cidrBlock: "10.0.0.0/8", description: "corp" }], onSave);

    const input = screen.getByRole("textbox", {
      name: "CIDR block for rule 1",
    });
    await user.clear(input);
    await user.type(input, "10.1.0.0/16");
    await user.click(screen.getByRole("button", { name: "Save allowlist" }));

    expect(onSave).toHaveBeenCalledTimes(1);
    expect(
      screen.getByRole("textbox", { name: "CIDR block for rule 1" }),
    ).toHaveValue("10.1.0.0/16");
  });

  it("does not call onSave before the Save button is pressed", async () => {
    const user = userEvent.setup();
    const onSave = renderEditor([
      { cidrBlock: "10.0.0.0/8", description: "corp" },
    ]);

    await user.type(
      screen.getByRole("textbox", { name: "Description for rule 1" }),
      " abc",
    );
    expect(onSave).not.toHaveBeenCalled();
  });

  it("keeps the description control working, the live control that passed", async () => {
    const user = userEvent.setup();
    renderEditor([{ cidrBlock: "10.0.0.0/8", description: "corp" }]);

    const description = screen.getByRole("textbox", {
      name: "Description for rule 1",
    });
    await user.click(description);
    await user.keyboard(" abc");
    expect(description).toHaveValue("corp abc");
    expect(description).toHaveFocus();
  });
});
