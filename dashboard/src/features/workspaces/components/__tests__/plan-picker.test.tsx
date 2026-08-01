import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PlanPicker } from "@/features/workspaces/components/plan-picker";

describe("PlanPicker", () => {
  it("renders the bex capability lineup — Hobby, Pro, Scale, Enterprise", () => {
    render(<PlanPicker selected="hobby" onSelect={vi.fn()} />);
    expect(screen.getByRole("radio", { name: /Hobby/ })).toBeChecked();
    expect(screen.getByRole("radio", { name: /Pro/ })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Scale/ })).toBeInTheDocument();
    expect(
      screen.getByRole("radio", { name: /Enterprise/ }),
    ).toBeInTheDocument();
  });

  it("states the actual resource-based billing model without Render's workspace fees", () => {
    render(<PlanPicker selected="hobby" onSelect={vi.fn()} />);

    expect(screen.getAllByText("No workspace fee")).toHaveLength(3);
    expect(screen.getByText("Custom terms")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Service and datastore usage is billed separately by resource tier.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("$25/mo")).not.toBeInTheDocument();
    expect(screen.queryByText("$499/mo")).not.toBeInTheDocument();
  });

  it("calls onSelect with the clicked plan's id", async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(<PlanPicker selected="hobby" onSelect={onSelect} />);

    await user.click(screen.getByRole("radio", { name: /Pro/ }));
    expect(onSelect).toHaveBeenCalledWith("pro");
  });
});
