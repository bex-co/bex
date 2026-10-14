import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PlanCardGrid } from "../plan-card-grid";

// w4/m207: card radios follow the W3C radio-group keyboard contract.
const tiers = [
  { id: "free", name: "Free", cpu: "0.1", memory: "256Mi" },
  { id: "starter", name: "Starter", cpu: "0.5", memory: "512Mi" },
  { id: "standard", name: "Standard", cpu: "1", memory: "2Gi" },
];

function Harness({
  initial,
  onChange = () => {},
  disabled,
}: {
  initial: string;
  onChange?: (id: string) => void;
  disabled?: boolean;
}) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <PlanCardGrid
        instanceTypes={tiers}
        value={value}
        disabled={disabled}
        ariaLabel="Instance Type"
        onChange={(id) => {
          onChange(id);
          setValue(id);
        }}
      />
      <button type="button">after</button>
    </>
  );
}

const radio = (name: RegExp) => screen.getByRole("radio", { name });

describe("PlanCardGrid keyboard", () => {
  it("is one named group with a single Tab stop on the checked card", async () => {
    const user = userEvent.setup();
    render(<Harness initial="starter" />);
    expect(screen.getByRole("radiogroup", { name: "Instance Type" })).toBeTruthy();
    await user.tab();
    expect(document.activeElement).toBe(radio(/Starter/));
    await user.tab();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "after" }));
  });

  it("moves focus and selection with the arrows, wrapping, and Home/End", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness initial="free" onChange={onChange} />);
    await user.tab();
    await user.keyboard("{ArrowRight}");
    expect(document.activeElement).toBe(radio(/Starter/));
    expect(radio(/Starter/).getAttribute("aria-checked")).toBe("true");
    expect(radio(/Free/).getAttribute("aria-checked")).toBe("false");
    await user.keyboard("{ArrowDown}{ArrowDown}");
    expect(document.activeElement).toBe(radio(/Free/)); // wrapped
    await user.keyboard("{ArrowLeft}");
    expect(document.activeElement).toBe(radio(/Standard/));
    await user.keyboard("{Home}");
    expect(document.activeElement).toBe(radio(/Free/));
    await user.keyboard("{End}{ArrowUp}");
    expect(radio(/Starter/).getAttribute("aria-checked")).toBe("true");
    expect(onChange.mock.calls.map(([id]) => id)).toEqual([
      "starter", "standard", "free", "standard", "free", "standard", "starter",
    ]);
  });

  it("keeps Space and click, and never selects on render or focus", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness initial="" onChange={onChange} />);
    expect(radio(/Free/).tabIndex).toBe(0); // no value: the first card is the stop
    await user.tab();
    expect(onChange).not.toHaveBeenCalled();
    await user.keyboard(" ");
    expect(onChange).toHaveBeenLastCalledWith("free");
    await user.click(radio(/Standard/));
    expect(onChange).toHaveBeenLastCalledWith("standard");
  });

  it("ignores the arrows when disabled", async () => {
    const onChange = vi.fn();
    render(<Harness initial="free" onChange={onChange} disabled />);
    radio(/Free/).focus();
    await userEvent.setup().keyboard("{ArrowRight}");
    expect(onChange).not.toHaveBeenCalled();
  });
});
