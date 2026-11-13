import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  AGENT_MODEL_TERMS_PATH,
  PricingWithTerms,
} from "@/features/legal/components/agent-model-terms-link";

describe("PricingWithTerms", () => {
  it("opens the public terms in a new tab", () => {
    render(<PricingWithTerms pricing="Cost + 30%." />);
    const link = screen.getByRole("link", { name: "Terms" });
    expect(AGENT_MODEL_TERMS_PATH).toBe("/legal/agent-models");
    expect(link).toHaveAttribute("href", "/legal/agent-models");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noreferrer");
  });

  it("renders a skeleton placeholder as the same text with nothing to follow", () => {
    render(<PricingWithTerms pricing="Cost + 30%." placeholder />);
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.getByText("Terms")).toHaveClass("underline");
  });
});
