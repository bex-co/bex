import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { PricingWithTerms } from "@/features/legal/components/agent-model-terms-link";
import {
  AGENT_MODEL_TERMS_URL,
  agentModelTermsUrl,
} from "@/features/legal/lib/agent-model-terms-url";

describe("PricingWithTerms", () => {
  it("opens the terms on bex.co in a new tab", () => {
    render(<PricingWithTerms pricing="Cost + 30%." />);
    const link = screen.getByRole("link", { name: "Terms" });
    expect(link).toHaveAttribute("href", AGENT_MODEL_TERMS_URL);
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noreferrer");
  });

  it("renders a skeleton placeholder as the same text with nothing to follow", () => {
    render(<PricingWithTerms pricing="Cost + 30%." placeholder />);
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.getByText("Terms")).toHaveClass("underline");
  });
});

describe("agentModelTermsUrl", () => {
  it("links bex.co's copy in the reader's language", () => {
    expect(agentModelTermsUrl("en")).toBe(
      "https://bex.co/terms-of-service#agent-model",
    );
    expect(agentModelTermsUrl("zh-CN")).toBe(
      "https://bex.co/zh/terms-of-service#agent-model",
    );
    expect(agentModelTermsUrl("pt-BR")).toBe(
      "https://bex.co/pt/terms-of-service#agent-model",
    );
    expect(agentModelTermsUrl("fr")).toBe(
      "https://bex.co/terms-of-service#agent-model",
    );
  });
});
