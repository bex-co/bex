import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { AutomationSettingsCard } from "@/features/agent-automations/components/automation-settings-card";
import { automation } from "./fixtures";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, ...props }: React.ComponentProps<"a">) => (
    <a {...props}>{children}</a>
  ),
}));

// "Created by" resolves a member through Apollo; this card test does not need it.
vi.mock("@/common/hooks/use-workspace-subject-label", () => ({
  useWorkspaceSubjectLabel: (subject: string) => subject,
}));

function withLimit(maxSpendUSD: number | null) {
  const base = automation();
  return {
    ...base,
    template: { ...base.template!, maxSpendUSD },
  };
}

// w21/039: the per-run limit goes through the same currency helper the agent
// session pages use, so it is grouped and always has two decimals.
describe("AutomationSettingsCard spend limit", () => {
  it.each([
    [0.5, "$0.50"],
    [5, "$5.00"],
    [12345.5, "$12,345.50"],
    [100000, "$100,000.00"],
  ])("renders %s as %s", (limit, text) => {
    render(<AutomationSettingsCard automation={withLimit(limit)} />);
    expect(screen.getByText(text)).toBeInTheDocument();
  });

  it("keeps the no-limit copy when none is set", () => {
    render(<AutomationSettingsCard automation={withLimit(null)} />);
    expect(screen.queryByText(/^\$/)).toBeNull();
  });
});
