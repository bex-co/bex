import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import type { TranslationEntry } from "@/i18n";
import enUsage from "@/features/usage/locales/en";
import zhUsage from "@/features/usage/locales/zh";
import enAgentEnvironment from "@/features/agent-environment/locales/en";
import zhAgentEnvironment from "@/features/agent-environment/locales/zh";

// The agent model pricing statements quote the markup bex-api bills from
// lego/backend/internal/pricing/pricing.yaml (ADR091 D1). A pricing change that
// forgets the copy fails here. The terms on bex.co quote no figures.
const PRICING = readFileSync(
  `${process.cwd()}/../lego/backend/internal/pricing/pricing.yaml`,
  "utf8",
);
const AGENT_MODEL = PRICING.slice(PRICING.indexOf("\nagentModel:\n"));
const markup = /^\s+markup: ([\d.]+)\s*$/m.exec(AGENT_MODEL);
if (!markup) throw new Error("pricing.yaml agentModel has no markup");
const markupPercent = `${Math.round((Number(markup[1]) - 1) * 100)}%`;

const LOCALES: Array<[string, Record<string, TranslationEntry>]> = [
  ["en", { ...enUsage, ...enAgentEnvironment }],
  ["zh", { ...zhUsage, ...zhAgentEnvironment }],
];

describe.each(LOCALES)("%s agent model copy matches pricing.yaml", (_, m) => {
  it("quotes the markup wherever the price is stated", () => {
    for (const key of [
      "usage.agentModelPricing",
      "agentEnvironment.spendPricing",
    ]) {
      expect(m[key]!.message).toContain(markupPercent);
    }
  });
});
