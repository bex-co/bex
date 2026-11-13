import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import type { TranslationEntry } from "@/i18n";
import enLegal from "@/features/legal/locales/en";
import zhLegal from "@/features/legal/locales/zh";
import enUsage from "@/features/usage/locales/en";
import zhUsage from "@/features/usage/locales/zh";
import enAgentEnvironment from "@/features/agent-environment/locales/en";
import zhAgentEnvironment from "@/features/agent-environment/locales/zh";

// The agent model terms (/legal/agent-models) and the pricing statements quote
// figures that bex-api enforces from lego/backend/internal/pricing/pricing.yaml
// (ADR091 D1, D5, D10.1). A pricing change that forgets the copy fails here.
const PRICING = readFileSync(
  `${process.cwd()}/../lego/backend/internal/pricing/pricing.yaml`,
  "utf8",
);
const AGENT_MODEL = PRICING.slice(PRICING.indexOf("\nagentModel:\n"));

function field(block: string, name: string): number {
  const match = new RegExp(`^\\s+${name}: ([\\d.]+)\\s*$`, "m").exec(block);
  if (!match) throw new Error(`pricing.yaml agentModel has no ${name}`);
  return Number(match[1]);
}

function planCap(plan: string): number {
  const match = new RegExp(`- plan: ${plan}\\n\\s+usd: (\\d+)`).exec(
    AGENT_MODEL,
  );
  if (!match) throw new Error(`pricing.yaml has no ${plan} spend cap`);
  return Number(match[1]);
}

const T0 = AGENT_MODEL.slice(
  AGENT_MODEL.indexOf("- tier: T0"),
  AGENT_MODEL.indexOf("- tier: T1"),
);
const usd = (n: number) => `$${n.toLocaleString("en-US")}`;
const markupPercent = `${Math.round((field(AGENT_MODEL, "markup") - 1) * 100)}%`;

const LOCALES: Array<[string, Record<string, TranslationEntry>]> = [
  ["en", { ...enLegal, ...enUsage, ...enAgentEnvironment }],
  ["zh", { ...zhLegal, ...zhUsage, ...zhAgentEnvironment }],
];

describe.each(LOCALES)("%s agent model copy matches pricing.yaml", (_, m) => {
  const message = (key: string) => m[key]!.message;

  it("quotes each plan's monthly spend cap and the override multiplier", () => {
    const caps = message("legal.agentModels.capsPlan");
    expect(caps).toContain(`Hobby ${usd(planCap("hobby"))}`);
    expect(caps).toContain(`Pro ${usd(planCap("pro"))}`);
    expect(caps).toContain(`Scale ${usd(planCap("scale"))}`);
    expect(caps).toContain(`${field(AGENT_MODEL, "maxCapMultiplier")} `);
  });

  // ADR091 D10.2a (2026-11-13): reaching a threshold charges the card and the
  // workspace keeps working, so the terms quote no new-workspace cap.
  it("quotes the new-workspace (T0) concurrency and charge threshold", () => {
    // The copy spells the T0 concurrency out ("two" / "两个").
    expect(field(T0, "maxConcurrentSessions")).toBe(2);
    expect(message("legal.agentModels.chargesThreshold")).toContain(
      usd(field(T0, "chargeThresholdUsd")),
    );
  });

  it("quotes the markup wherever the price is stated", () => {
    for (const key of [
      "legal.agentModels.priceBody",
      "usage.agentModelPricing",
      "agentEnvironment.spendPricing",
    ]) {
      expect(message(key)).toContain(markupPercent);
    }
  });
});
