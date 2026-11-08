import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AutomationStatusBadge } from "@/features/agent-automations/components/automation-status";
import { AutomationLoadError } from "@/features/agent-automations/components/automation-load-state";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";

describe("AutomationStatusBadge (ADR101 D3, D4)", () => {
  it("shows nothing for an automation that will fire", () => {
    const { container } = render(
      <AutomationStatusBadge
        automation={{ enabled: true, nextFireAt: "2099-01-01T00:00:00Z" }}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("calls a person's pause plain Paused, with no reason", () => {
    render(<AutomationStatusBadge automation={{ enabled: false }} />);
    const badge = screen.getByText("Paused").closest("[data-paused-reason]");
    expect(badge).toHaveAttribute("data-paused-reason", "");
  });

  it.each([
    ["repeated_failures", "Paused: repeated failures"],
    ["creator_lost_access", "Paused: creator lost access"],
    ["template_invalid", "Paused: template invalid"],
    ["something_new", "Paused by the platform"],
  ])("names the platform pause %s", (reason, label) => {
    render(
      <AutomationStatusBadge
        automation={{ enabled: false, pausedReason: reason }}
      />,
    );
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it("marks a finished schedule Ended, not Paused", () => {
    render(
      <AutomationStatusBadge
        automation={{ enabled: true, nextFireAt: null }}
      />,
    );
    expect(screen.getByText("Ended")).toBeInTheDocument();
    expect(screen.queryByText("Paused")).not.toBeInTheDocument();
  });
});

describe("AutomationLoadError", () => {
  it("offers Retry for an outage and retries on click", async () => {
    const onRetry = vi.fn();
    render(
      <AutomationLoadError
        error={uncodedGraphQLError("boom")}
        onRetry={onRetry}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Couldn't load automations.",
    );
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("says a missing automation is gone, with nothing to retry", () => {
    render(
      <AutomationLoadError
        error={codedGraphQLError("AGENT_AUTOMATION_NOT_FOUND")}
        onRetry={vi.fn()}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "This automation doesn't exist or was deleted.",
    );
    expect(
      screen.queryByRole("button", { name: "Retry" }),
    ).not.toBeInTheDocument();
  });

  it("explains a refusal without a Retry that cannot help", () => {
    render(
      <AutomationLoadError
        error={codedGraphQLError("FORBIDDEN")}
        onRetry={vi.fn()}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Only members with the developer role or higher",
    );
    expect(
      screen.queryByRole("button", { name: "Retry" }),
    ).not.toBeInTheDocument();
  });
});
