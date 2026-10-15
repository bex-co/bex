import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n/init";
import { Combobox } from "../combobox";
import { Command, CommandItem, CommandList } from "../command";

const options = [
  { value: "alpha", label: "Alpha" },
  { value: "beta", label: "Beta" },
];

describe("Combobox keyboard selection", () => {
  it.each([
    ["ArrowDown", "alpha"],
    ["ArrowUp", "beta"],
  ])(
    "opens a closed list with %s and selects its boundary option",
    (key, value) => {
      const onValueChange = vi.fn();
      render(
        <Combobox options={options} value="" onValueChange={onValueChange} />,
      );
      const input = screen.getByRole("combobox");
      fireEvent.keyDown(input, { key });
      expect(input).toHaveAttribute("aria-expanded", "true");
      fireEvent.keyDown(input, { key: "Enter" });
      expect(onValueChange).toHaveBeenCalledWith(value);
      expect(input).toHaveValue(value);
      expect(input).toHaveAttribute("aria-expanded", "false");
    },
  );

  it("clamps arrow navigation at both ends of an open list", () => {
    const onValueChange = vi.fn();
    render(
      <Combobox options={options} value="" onValueChange={onValueChange} />,
    );
    const input = screen.getByRole("combobox");
    fireEvent.focus(input);
    for (let i = 0; i < 3; i++) fireEvent.keyDown(input, { key: "ArrowDown" });
    expect(screen.getByRole("option", { name: "Beta" })).toHaveClass(
      "bg-accent",
    );
    for (let i = 0; i < 3; i++) fireEvent.keyDown(input, { key: "ArrowUp" });
    expect(screen.getByRole("option", { name: "Alpha" })).toHaveClass(
      "bg-accent",
    );
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onValueChange).toHaveBeenCalledWith("alpha");
  });

  it.each([true, false])(
    "keeps custom Enter behavior with allowCustom=%s",
    (allowCustom) => {
      const onValueChange = vi.fn();
      render(
        <Combobox
          options={options}
          value="custom"
          allowCustom={allowCustom}
          onValueChange={onValueChange}
        />,
      );
      const input = screen.getByRole("combobox");
      fireEvent.focus(input);
      fireEvent.keyDown(input, { key: "ArrowDown" });
      fireEvent.keyDown(input, { key: "Enter" });
      expect(onValueChange.mock.calls).toEqual(allowCustom ? [["custom"]] : []);
      expect(input).toHaveAttribute("aria-expanded", String(!allowCustom));
    },
  );

  it("Escape closes and clears the highlight without selecting an option", () => {
    const onValueChange = vi.fn();
    render(
      <Combobox options={options} value="" onValueChange={onValueChange} />,
    );
    const input = screen.getByRole("combobox");
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "Escape" });
    expect(input).toHaveAttribute("aria-expanded", "false");
    expect(onValueChange).not.toHaveBeenCalled();
    fireEvent.focus(input);
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onValueChange).toHaveBeenCalledWith("");
  });
});

describe("results list name (w4/312)", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("names a Chinese picker's listbox in Chinese", async () => {
    await i18n.changeLanguage("zh");
    render(<Combobox options={options} value="" onValueChange={vi.fn()} />);
    fireEvent.keyDown(screen.getByRole("combobox"), { key: "ArrowDown" });
    expect(screen.getByRole("listbox", { name: "建议" })).toBeInTheDocument();
    expect(screen.queryByRole("listbox", { name: "Suggestions" })).toBeNull();
  });

  it("keeps a caller's explicit list label", async () => {
    await i18n.changeLanguage("zh");
    render(
      <Command>
        <CommandList label="Branches">
          <CommandItem>main</CommandItem>
        </CommandList>
      </Command>,
    );
    expect(
      screen.getByRole("listbox", { name: "Branches" }),
    ).toBeInTheDocument();
  });
});
