import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Command, CommandInput, CommandItem, CommandList } from "../command";
import { Combobox } from "../combobox";

// The active option of a cmdk list (the global search, a combobox) is marked
// by a 2px outline inset in the focus colour, as Render's list rows are
// (w3/114): focus stays in the search field, so its ring does not move with
// the arrows, and the tint alone measured 1.2:1 on the list. The ring is the
// keyboard's: an option the pointer moved onto keeps the tint alone. jsdom
// applies no stylesheet, so this reads the classes an option renders; the
// measured ratios are in the item's record.

const SELECTED = "data-[selected=true]:";

/** What an option draws: the inset ring with the tint, the tint, or neither. */
function mark(option: HTMLElement): "ring" | "tint" | "none" {
  const selected = option.getAttribute("data-selected") === "true";
  const classes = option.className.split(/\s+/).flatMap((c) => {
    if (c.startsWith(SELECTED))
      return selected ? [c.slice(SELECTED.length)] : [];
    return c.includes(":") ? [] : [c];
  });
  if (!classes.includes("bg-accent")) return "none";
  // Tailwind 4 draws `outline-2` in `var(--tw-outline-style)`, which an
  // unconditional `outline-none` sets to none.
  const voided = option.className
    .split(/\s+/)
    .some((c) => c === "outline-none" || c === "outline-hidden");
  const ring = ["outline-2", "-outline-offset-2", "outline-ring"].every((c) =>
    classes.includes(c),
  );
  return ring && !voided ? "ring" : "tint";
}

const marks = () => screen.getAllByRole("option").map(mark);

function Palette() {
  return (
    <Command label="Search">
      <CommandInput aria-label="Search" />
      <CommandList>
        <CommandItem value="alpha">Alpha</CommandItem>
        <CommandItem value="beta">Beta</CommandItem>
        <CommandItem value="gamma">Gamma</CommandItem>
        <CommandItem value="delta" disabled>
          Delta
        </CommandItem>
      </CommandList>
    </Command>
  );
}

describe("a cmdk list marks its active option with the focus ring (w3/114)", () => {
  it("rings the option the arrows make active, and only that one", () => {
    render(<Palette />);
    // cmdk starts on the first option, which Enter would pick.
    expect(marks()).toEqual(["ring", "none", "none", "none"]);
    fireEvent.keyDown(screen.getByRole("combobox"), { key: "ArrowDown" });
    expect(screen.getByRole("option", { name: "Beta" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(marks()).toEqual(["none", "ring", "none", "none"]);
  });

  it("leaves the option a moving pointer selects with the tint alone, until a key is pressed", () => {
    render(<Palette />);
    fireEvent.pointerMove(screen.getByRole("option", { name: "Gamma" }));
    expect(screen.getByRole("option", { name: "Gamma" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(marks()).toEqual(["none", "none", "tint", "none"]);

    fireEvent.keyDown(screen.getByRole("combobox"), { key: "ArrowUp" });
    expect(marks()).toEqual(["none", "ring", "none", "none"]);
  });

  it("keeps the ring where the keyboard put it when the pointer moves over the field or a disabled option", () => {
    render(<Palette />);
    fireEvent.keyDown(screen.getByRole("combobox"), { key: "ArrowDown" });
    fireEvent.pointerMove(screen.getByRole("combobox"));
    fireEvent.pointerMove(screen.getByRole("option", { name: "Delta" }));
    expect(marks()).toEqual(["none", "ring", "none", "none"]);
  });
});

describe("a combobox marks the option its field made active (w3/114)", () => {
  const options = [
    { value: "main", label: "main" },
    { value: "develop", label: "develop" },
    { value: "release", label: "release" },
  ];

  it("rings the arrows' highlight, tints the pointer's, and marks nothing Enter would not pick", () => {
    const onValueChange = vi.fn();
    render(
      <Combobox options={options} value="" onValueChange={onValueChange} />,
    );
    const input = screen.getByRole("combobox");
    fireEvent.click(input);
    // cmdk selects its first option on its own; Enter here keeps the typed
    // text, so no option may look active.
    expect(marks()).toEqual(["none", "none", "none"]);

    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "ArrowDown" });
    expect(marks()).toEqual(["none", "ring", "none"]);

    fireEvent.mouseMove(screen.getByRole("option", { name: "release" }));
    expect(marks()).toEqual(["none", "none", "tint"]);

    fireEvent.keyDown(input, { key: "ArrowUp" });
    expect(marks()).toEqual(["none", "ring", "none"]);
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onValueChange).toHaveBeenCalledWith("develop");
  });

  it("does not move the highlight to an option the list scrolls under a still pointer", () => {
    render(<Combobox options={options} value="" onValueChange={vi.fn()} />);
    const input = screen.getByRole("combobox");
    fireEvent.click(input);
    fireEvent.keyDown(input, { key: "ArrowDown" });
    // What the browser sends when an option slides under the pointer.
    fireEvent.mouseEnter(screen.getByRole("option", { name: "release" }));
    expect(marks()).toEqual(["ring", "none", "none"]);
  });
});
