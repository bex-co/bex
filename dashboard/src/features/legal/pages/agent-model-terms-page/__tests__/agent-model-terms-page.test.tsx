import { afterEach, describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import i18n from "@/i18n/init";
import { LegalAgentModelsRouteSkeleton } from "@/common/components/route-skeletons";
import AgentModelTermsPage from "@/features/legal/pages/agent-model-terms-page";

function headings(container: HTMLElement): string[] {
  return [...container.querySelectorAll("h1, h2")].map(
    (h) => h.textContent ?? "",
  );
}

afterEach(async () => {
  await i18n.changeLanguage("en");
});

// ADR091 D8 + D10.7: the public agent model usage terms (2026-11-12).
describe("AgentModelTermsPage", () => {
  it("states the effective date and every section", () => {
    const { container } = render(<AgentModelTermsPage />);
    expect(
      screen.getByRole("heading", {
        level: 1,
        name: "Agent model usage terms",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Effective November 14, 2026")).toBeInTheDocument();
    // ADR108: Muse Code's contributor tier is disclosed as the exception.
    expect(
      screen.getByText(
        /Meta may use the prompts, code, files and context sent/,
      ),
    ).toBeInTheDocument();
    expect(headings(container).slice(1)).toEqual([
      "Scope",
      "Your data",
      "Price",
      "Spend caps",
      "Charges during the month",
      "Deleting a workspace or account",
      "Payment methods",
      "Usage records",
      "Changes to these terms",
    ]);
    // A reader can switch language without signing in.
    expect(
      screen.getByRole("button", { name: "Change language" }),
    ).toBeInTheDocument();
  });

  it("keeps the facts customers rely on", () => {
    render(<AgentModelTermsPage />);
    for (const fact of [
      /bex-provided model rather than the workspace's own model provider key/,
      /prompts, code and tool output to third-party model providers through OpenRouter/,
      /OpenRouter provider routing with data collection denied/,
      /its own provider key.*your provider under your agreement with them.*bex bills no model tokens/,
      /actual cost plus 30%.*metered per request.*Billing → Agent model/,
      /Hobby \$20, Pro \$200, Scale \$2,000.*up to 10 times/,
      /two concurrent sessions on the bex-provided model until its first successful payment/,
      /refused until the next month begins or an admin raises the cap/,
      /charge threshold, in addition to the monthly invoice.*starts at \$10 and rises with the workspace's payment history/,
      /The workspace keeps working while the charge is made/,
      /If a charge fails or needs your authentication, agent model usage is paused until the balance is paid/,
      /If that charge fails, deletion waits until the balance is paid/,
      /usage incurred before deletion, including after the deletion/,
      /Prepaid cards aren't accepted.*unpaid bex balance may be refused/,
      /\(time, model, tokens and price\) for 13 months.*evidence if a charge is disputed/,
      /effective date at the top of this page shows the current version/,
    ]) {
      expect(screen.getByText(fact)).toBeInTheDocument();
    }
  });

  it("reads in Chinese", async () => {
    await i18n.changeLanguage("zh");
    render(<AgentModelTermsPage />);
    expect(
      screen.getByRole("heading", { level: 1, name: "智能体模型使用条款" }),
    ).toBeInTheDocument();
    expect(screen.getByText("生效日期：2026年11月14日")).toBeInTheDocument();
    expect(screen.getByText(/实际成本加 30%/)).toBeInTheDocument();
  });

  // dashboard/AGENTS.md skeleton rule: the pending frame renders the same
  // header and sections as real text, so pending and ready wrap identically.
  it("has a route skeleton with the ready page's geometry", () => {
    const ready = render(<AgentModelTermsPage />);
    const readyHeadings = headings(ready.container);
    const readyParagraphs = ready.container.querySelectorAll("p").length;
    ready.unmount();

    const { container } = render(<LegalAgentModelsRouteSkeleton />);
    const frame = container.querySelector<HTMLElement>(
      '[data-route-skeleton="legal-agent-models"]',
    );
    expect(frame).not.toBeNull();
    expect(
      [...frame!.querySelectorAll<HTMLElement>("[data-skeleton-region]")].map(
        (r) => r.dataset.skeletonRegion,
      ),
    ).toEqual(["language-action", "page-header", "terms-sections"]);
    expect(headings(frame!)).toEqual(readyHeadings);
    expect(frame!.querySelectorAll("p")).toHaveLength(readyParagraphs);
    expect(within(frame!).queryByRole("button")).toBeNull();
  });
});
