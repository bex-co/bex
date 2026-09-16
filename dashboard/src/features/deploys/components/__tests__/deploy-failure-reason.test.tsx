import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { DeployFailureReason } from "@/features/deploys/components/deploy-failure-reason";

describe("DeployFailureReason", () => {
  it("renders the reason with the destructive treatment by default", () => {
    const { container } = render(
      <DeployFailureReason reason="image pull failed: not found" />,
    );

    const el = screen.getByText("image pull failed: not found");
    expect(el.tagName).toBe("P");
    expect(el.className).toContain("text-destructive");
    expect(container.querySelector("p[title]")).toBeNull();
  });

  it("renders a supersede cancel with the neutral treatment", () => {
    render(
      <DeployFailureReason
        reason="Superseded by dep-abc"
        tone="neutral"
      />,
    );

    const el = screen.getByText("Superseded by dep-abc");
    expect(el.className).toContain("text-muted-foreground");
    expect(el.className).not.toContain("text-destructive");
  });

  it("renders nothing without a reason", () => {
    const { container } = render(<DeployFailureReason reason="" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("truncates with a hover tooltip for list rows", () => {
    render(
      <DeployFailureReason reason="image pull failed: not found" truncate />,
    );

    const el = screen.getByText("image pull failed: not found");
    expect(el.className).toContain("truncate");
    expect(el).toHaveAttribute("title", "image pull failed: not found");
  });
});
