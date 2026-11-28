import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
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

  // w7/088: in the change-plan dialog's ~220 px cards the "per organization"
  // price squeezed the name to "Sca / le" and "Enter / prise". The name never
  // shrinks or breaks; the header row wraps so the price drops under it.
  it("keeps each plan name whole and lets a long price wrap under it", () => {
    render(<PlanPicker selected="hobby" onSelect={vi.fn()} />);

    for (const [name, price] of [
      ["Hobby", "$0/mo"],
      ["Pro", "$17.50/mo"],
      ["Scale", "$349.30/mo per organization"],
      ["Enterprise", "Custom terms per organization"],
    ]) {
      const card = screen.getByRole("radio", { name: new RegExp(name) });
      const nameEl = within(card).getByText(name, { exact: true });
      expect(nameEl).toHaveClass("shrink-0", "whitespace-nowrap");
      const header = nameEl.parentElement;
      expect(header).toHaveClass("flex-wrap");
      expect(within(header!).getByText(price)).not.toHaveClass("text-right");
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

// Render's TeamPlanPicker for an existing workspace (w3/m708): the plan page
// passes the workspace's plan as `current`.
describe("PlanPicker on the plan page", () => {
  it("marks the current plan and keeps it from being chosen again", async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(<PlanPicker current="pro" selected="pro" onSelect={onSelect} />);

    const pro = screen.getByRole("radio", { name: /Pro/ });
    expect(pro).toHaveTextContent("Current plan");
    expect(pro).toBeDisabled();
    expect(pro).toHaveTextContent("Select plan");
    await user.click(pro);
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("says New plan and Plan selected on the chosen other plan", () => {
    render(<PlanPicker current="pro" selected="hobby" onSelect={vi.fn()} />);
    const hobby = screen.getByRole("radio", { name: /Hobby/ });
    expect(hobby).toBeChecked();
    expect(hobby).toHaveTextContent("New plan");
    expect(hobby).toHaveTextContent("Plan selected");
    expect(screen.getByRole("radio", { name: /Scale/ })).toHaveTextContent(
      "Select plan",
    );
  });

  it("draws Enterprise as Get in touch, never a plan to select", () => {
    render(<PlanPicker current="hobby" selected="hobby" onSelect={vi.fn()} />);
    expect(
      screen.queryByRole("radio", { name: /Enterprise/ }),
    ).not.toBeInTheDocument();
    const card = screen.getByTestId("plan-card-enterprise");
    expect(card).toHaveTextContent("Custom terms per organization");
    expect(screen.getByRole("link", { name: "Get in touch" })).toHaveAttribute(
      "href",
      "mailto:hello@bex.co",
    );
    expect(screen.getAllByRole("radio")).toHaveLength(3);
  });

  it("leaves the current plan out of the arrow keys", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<PlanPicker current="pro" selected="pro" onSelect={onSelect} />);
    await user.tab();
    expect(document.activeElement).toBe(
      screen.getByRole("radio", { name: /Hobby/ }),
    );
    await user.keyboard("{ArrowRight}");
    expect(onSelect).toHaveBeenLastCalledWith("scale");
  });

  it("keeps New Workspace's four selectable plans", () => {
    render(<PlanPicker selected="hobby" onSelect={vi.fn()} />);
    expect(screen.getAllByRole("radio")).toHaveLength(4);
    expect(screen.queryByText("Current plan")).not.toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
