import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ServiceTypePicker } from "../service-type-picker";

// w4/m207: the Service Type cards are a named radio group with arrow keys.
describe("ServiceTypePicker keyboard", () => {
  it("is named and moves Web to Private with the arrows", async () => {
    const onChange = vi.fn();
    render(<ServiceTypePicker value="web_service" onChange={onChange} />);
    const group = screen.getByRole("radiogroup", { name: "Service Type" });
    expect(group).toBeTruthy();
    const radios = screen.getAllByRole("radio");
    expect(radios.filter((r) => r.tabIndex === 0)).toHaveLength(1);
    radios[0].focus();
    await userEvent.setup().keyboard("{ArrowDown}");
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith("private_service");
    expect(document.activeElement).toBe(radios[1]);
  });
});
