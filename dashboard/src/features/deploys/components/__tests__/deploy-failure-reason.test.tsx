import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { DeployFailureReason } from "@/features/deploys/components/deploy-failure-reason";

describe("DeployFailureReason", () => {
  it("renders the reason with the destructive treatment", () => {
    const { container } = render(
      <DeployFailureReason reason="image pull failed: not found" />,
    );

    const el = screen.getByText("image pull failed: not found");
    expect(el.tagName).toBe("P");
    expect(el.className).toContain("text-destructive");
    expect(container.querySelector("p[title]")).toBeNull();
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
