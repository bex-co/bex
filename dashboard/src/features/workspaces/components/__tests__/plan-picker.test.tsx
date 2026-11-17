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

  it("shows workspace fees at 30% off Render plus separate usage billing", () => {
    render(<PlanPicker selected="hobby" onSelect={vi.fn()} />);

    expect(screen.getByText("$0/mo")).toBeInTheDocument();
    expect(screen.getByText("$17.50/mo")).toBeInTheDocument();
    expect(screen.getByText("$349.30/mo per organization")).toBeInTheDocument();
    expect(
      screen.getByText("Custom terms per organization"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Service and datastore usage is billed separately by resource tier.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("1 member")).toBeInTheDocument();
    expect(screen.getByText("Up to 25 services")).toBeInTheDocument();
    expect(screen.getByText("5 Hobby workspaces per user")).toBeInTheDocument();
    expect(screen.getAllByText("Unlimited members").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Unlimited services").length).toBeGreaterThan(0);
    expect(
      screen.getByText("Extra roles (Contributor, Viewer, Billing)"),
    ).toBeInTheDocument();
  });

  // w7/m464 (ADR115 D2): a Scale or Enterprise workspace is its
  // organization's primary, and the one fee covers every workspace in it.
  it("prices Scale and Enterprise per organization, Hobby and Pro per workspace", () => {
    render(<PlanPicker selected="hobby" onSelect={vi.fn()} />);

    expect(screen.getByRole("radio", { name: /Scale/ })).toHaveTextContent(
      "$349.30/mo per organization",
    );
    expect(screen.getByRole("radio", { name: /Enterprise/ })).toHaveTextContent(
      "Custom terms per organization",
    );
    for (const plan of [/Hobby/, /Pro/]) {
      expect(screen.getByRole("radio", { name: plan })).not.toHaveTextContent(
        "per organization",
      );
    }
  });

  it("calls onSelect with the clicked plan's id", async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(<PlanPicker selected="hobby" onSelect={onSelect} />);

    await user.click(screen.getByRole("radio", { name: /Pro/ }));
    expect(onSelect).toHaveBeenCalledWith("pro");
  });

  // w4/m207: the arrows move focus and the draft plan; Tab leaves the group.
  it("moves between plans with the arrow keys and keeps one Tab stop", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const { rerender } = render(
      <>
        <PlanPicker selected="hobby" onSelect={onSelect} />
        <button type="button">after</button>
      </>,
    );
    await user.tab();
    expect(document.activeElement).toBe(
      screen.getByRole("radio", { name: /Hobby/ }),
    );
    await user.keyboard("{ArrowRight}");
    expect(onSelect).toHaveBeenLastCalledWith("pro");
    expect(document.activeElement).toBe(
      screen.getByRole("radio", { name: /Pro/ }),
    );
    rerender(
      <>
        <PlanPicker selected="pro" onSelect={onSelect} />
        <button type="button">after</button>
      </>,
    );
    await user.tab();
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "after" }),
    );
  });
});
