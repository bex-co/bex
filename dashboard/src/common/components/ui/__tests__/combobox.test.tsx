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

describe("Combobox state that follows its inputs", () => {
  it("shows a value the caller changes from outside", () => {
    const { rerender } = render(
      <Combobox options={options} value="alpha" onValueChange={vi.fn()} />,
    );
    const input = screen.getByRole("combobox");
    expect(input).toHaveValue("alpha");
    rerender(
      <Combobox options={options} value="beta" onValueChange={vi.fn()} />,
    );
    expect(input).toHaveValue("beta");
  });

  it("drops the keyboard highlight when typing changes the list", () => {
    const onValueChange = vi.fn();
    render(
      <Combobox options={options} value="" onValueChange={onValueChange} />,
    );
    const input = screen.getByRole("combobox");
    fireEvent.focus(input);
    fireEvent.keyDown(input, { key: "ArrowDown" });
    expect(screen.getByRole("option", { name: "Alpha" })).toHaveClass(
      "bg-accent",
    );

    fireEvent.change(input, { target: { value: "a" } });
    expect(screen.getByRole("option", { name: "Alpha" })).not.toHaveClass(
      "bg-accent",
    );
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onValueChange).toHaveBeenLastCalledWith("a");
    expect(onValueChange).not.toHaveBeenCalledWith("alpha");
  });

  it("reopens without the highlight it closed with", () => {
    render(
      <Combobox options={options} value="alpha" onValueChange={vi.fn()} />,
    );
    const input = screen.getByRole("combobox");
    fireEvent.focus(input);
    fireEvent.keyDown(input, { key: "ArrowDown" });
    // Re-selecting the option already typed closes the list without
    // changing what it shows.
    fireEvent.keyDown(input, { key: "Enter" });
    expect(input).toHaveAttribute("aria-expanded", "false");

    fireEvent.focus(input);
    expect(input).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("option", { name: "Alpha" })).not.toHaveClass(
      "bg-accent",
    );
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

describe("application-authored copy follows the selected language (w4/313)", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  const typeMissing = (value: string) => {
    const input = screen.getByRole("combobox");
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value } });
    return input;
  };

  it("translates the empty guidance, custom-value hint and clear name", async () => {
    await i18n.changeLanguage("zh");
    render(<Combobox options={options} value="" onValueChange={vi.fn()} />);
    const input = typeMissing("qa-missing-branch");
    expect(input).toHaveAttribute("placeholder", "选择一个选项");
    expect(screen.getByText("未找到结果")).toBeInTheDocument();
    expect(
      screen.getByText("按 Enter 使用“qa-missing-branch”"),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "清除输入" }));
    expect(input).toHaveValue("");
  });

  it("keeps English copy and the literal typed value", () => {
    render(<Combobox options={options} value="" onValueChange={vi.fn()} />);
    typeMissing('a"<b>{x}');
    expect(screen.getByText("No results found")).toBeInTheDocument();
    expect(
      screen.getByText('Press Enter to use "a"<b>{x}"'),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Clear input" }),
    ).toBeInTheDocument();
  });

  it("keeps caller placeholder and empty text, and omits the hint without custom values", async () => {
    await i18n.changeLanguage("zh");
    render(
      <Combobox
        options={options}
        value=""
        onValueChange={vi.fn()}
        placeholder="选择分支"
        emptyText="没有匹配的分支"
        allowCustom={false}
      />,
    );
    const input = typeMissing("qa-missing-branch");
    expect(input).toHaveAttribute("placeholder", "选择分支");
    expect(screen.getByText("没有匹配的分支")).toBeInTheDocument();
    expect(screen.queryByText(/按 Enter/)).toBeNull();
  });
});
