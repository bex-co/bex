import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { CRON_MINIMUM_USD } from "@/common/lib/cron-minimum";

// The cron hints quote the minimum bex-api bills (w9/m448). A pricing change
// that forgets the copy fails here.
describe("CRON_MINIMUM_USD", () => {
  it("is pricing.yaml's cronJob.minimumUsdPerMonth", () => {
    const pricing = readFileSync(
      `${process.cwd()}/../lego/backend/internal/pricing/pricing.yaml`,
      "utf8",
    );
    const minimum = /^cronJob:\n\s+minimumUsdPerMonth: ([\d.]+)/m.exec(pricing);
    expect(minimum).not.toBeNull();
    expect(Number(CRON_MINIMUM_USD)).toBe(Number(minimum![1]));
    expect(CRON_MINIMUM_USD).toMatch(/^\d+\.\d\d$/);
  });
});
