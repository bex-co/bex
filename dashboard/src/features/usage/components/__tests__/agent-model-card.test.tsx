import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AgentModelCard } from "@/features/usage/components/agent-model-card";

const USAGE = {
  monthToDateUsd: "3.73",
  models: [
    {
      model: "anthropic/claude-sonnet",
      costUsd: "2.50",
      inputTokens: 12000,
      outputTokens: 3400,
      cacheReadTokens: 50000,
      cacheWriteTokens: 1200,
    },
    {
      model: "deepseek/deepseek-v4",
      costUsd: "1.23",
      inputTokens: 10,
      outputTokens: 2,
      cacheReadTokens: 0,
      cacheWriteTokens: 0,
    },
  ],
};

// ADR091 D6: the billing page's Agent model line.
describe("AgentModelCard", () => {
  it("shows month-to-date spend with the pricing statement and a collapsed breakdown", async () => {
    const user = userEvent.setup();
    render(<AgentModelCard agentModel={USAGE} loading={false} current />);
    expect(screen.getByText("Agent model")).toBeInTheDocument();
    expect(
      screen.getByText("Billed at the model provider's cost + 30%."),
    ).toBeInTheDocument();
    expect(screen.getByText("Month to date")).toBeInTheDocument();
    expect(screen.getByTestId("agent-model-total")).toHaveTextContent("$3.73");
    expect(screen.getByText("anthropic/claude-sonnet")).not.toBeVisible();

    const toggle = screen.getByRole("button", {
      name: "Breakdown by model (2 models)",
    });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    // The toggle names the region it shows and hides.
    const region = document.getElementById(
      toggle.getAttribute("aria-controls") ?? "",
    );
    expect(region).toContainElement(
      screen.getByText("anthropic/claude-sonnet"),
    );
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("anthropic/claude-sonnet")).toBeVisible();
    expect(screen.getByText("$2.50")).toBeInTheDocument();
    expect(screen.getByText("$1.23")).toBeInTheDocument();
    expect(screen.getAllByText("Cache read tokens")).toHaveLength(2);
    expect(screen.getByText("50,000")).toBeInTheDocument();
  });

  it("reads $0.00 with no usage, and a closed month's total", () => {
    render(
      <AgentModelCard
        agentModel={{ monthToDateUsd: "0.00", models: [] }}
        loading={false}
        current={false}
      />,
    );
    expect(screen.getByTestId("agent-model-total")).toHaveTextContent("$0.00");
    expect(screen.getByText("Total for the month")).toBeInTheDocument();
    expect(
      screen.getByText("No agent model usage in this period."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("renders its skeleton while the first read is in flight", () => {
    render(<AgentModelCard agentModel={null} loading current />);
    expect(screen.getByTestId("agent-model-card-skeleton")).toBeInTheDocument();
  });

  // ADR091 D8: the pricing statement links the public terms; the skeleton
  // carries the same title, statement and "Terms" text, so the header keeps
  // its ready geometry, with nothing to follow while loading.
  it("links the agent model terms, and holds the link text in its skeleton", () => {
    const { unmount } = render(
      <AgentModelCard agentModel={USAGE} loading={false} current />,
    );
    expect(screen.getByRole("link", { name: "Terms" })).toHaveAttribute(
      "href",
      "/legal/agent-models",
    );
    unmount();

    render(<AgentModelCard agentModel={null} loading current />);
    const skeleton = within(screen.getByTestId("agent-model-card-skeleton"));
    expect(skeleton.getByText("Agent model")).toBeInTheDocument();
    expect(
      skeleton.getByText("Billed at the model provider's cost + 30%."),
    ).toBeInTheDocument();
    expect(skeleton.getByText("Terms")).toBeInTheDocument();
    expect(skeleton.queryByRole("link")).toBeNull();
  });
});
