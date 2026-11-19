import { describe, expect, it } from "vitest";
import type { Translate } from "@/common/hooks/use-translations";
import {
  namedTemplateLabel,
  templateLabel,
} from "@/features/agent-sessions/lib/templates";

// A translator that shows exactly what the helpers hand the catalog.
const t = ((key: string, params: Record<string, string | number> = {}) =>
  key === "agentSessions.sandboxSize"
    ? `${params.cpu} CPU, ${params.memory} GB`
    : `${params.name} · ${params.size}`) as Translate;

// w10/m519: a template's size is the server's numbers, written the way the
// dashboard language writes numbers.
describe("templateLabel", () => {
  const half = { name: "half-box", cpu: 0.5, memoryGb: 1.5 };

  it("writes the server's CPU and memory in the dashboard language", () => {
    expect(templateLabel(half, t, "en")).toBe("half-box · 0.5 CPU, 1.5 GB");
    expect(templateLabel(half, t, "de")).toBe("half-box · 0,5 CPU, 1,5 GB");
    expect(
      templateLabel({ name: "big", cpu: 16, memoryGb: 64 }, t, "zh-CN"),
    ).toBe("big · 16 CPU, 64 GB");
  });

  it("names a template the list does not carry without a size", () => {
    expect(namedTemplateLabel("half-box", [half], t, "en")).toBe(
      "half-box · 0.5 CPU, 1.5 GB",
    );
    expect(namedTemplateLabel("retired", [half], t, "en")).toBe("retired");
  });
});
