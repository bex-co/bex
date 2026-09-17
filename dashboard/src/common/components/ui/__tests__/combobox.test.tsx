import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Combobox } from "../combobox";

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
