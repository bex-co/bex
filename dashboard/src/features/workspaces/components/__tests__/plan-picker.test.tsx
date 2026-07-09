import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PlanPicker } from "@/features/workspaces/components/plan-picker";

describe("PlanPicker", () => {
  it("renders Render's flat-rate lineup — Hobby, Pro, Scale, Enterprise", () => {
    render(<PlanPicker selected="hobby" onSelect={vi.fn()} />);
    expect(screen.getByRole("radio", { name: /Hobby/ })).toBeChecked();
    expect(screen.getByRole("radio", { name: /Pro/ })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Scale/ })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Enterprise/ })).toBeInTheDocument();
  });

  it("calls onSelect with the clicked plan's id", async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(<PlanPicker selected="hobby" onSelect={onSelect} />);

    await user.click(screen.getByRole("radio", { name: /Pro/ }));
    expect(onSelect).toHaveBeenCalledWith("pro");
  });
});
