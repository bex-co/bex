import { describe, it, expect } from "vitest";
import {
  formatInstanceCPU,
  formatInstanceMemory,
} from "@/features/services/lib/instance-type";

describe("formatInstanceCPU", () => {
  it("converts millicores to fractional cores", () => {
    expect(formatInstanceCPU("100m")).toBe("0.1 CPU");
    expect(formatInstanceCPU("500m")).toBe("0.5 CPU");
  });

  it("keeps whole-core values as whole numbers, not 1.0", () => {
    expect(formatInstanceCPU("1")).toBe("1 CPU");
    expect(formatInstanceCPU("2")).toBe("2 CPU");
    expect(formatInstanceCPU("8")).toBe("8 CPU");
  });
});

describe("formatInstanceMemory", () => {
  it("relabels Mi/Gi as MB/GB (Render's card unit spelling)", () => {
    expect(formatInstanceMemory("512Mi")).toBe("512 MB");
    expect(formatInstanceMemory("2Gi")).toBe("2 GB");
    expect(formatInstanceMemory("32Gi")).toBe("32 GB");
  });

  it("returns the raw string for an unrecognized unit rather than throwing", () => {
    expect(formatInstanceMemory("512")).toBe("512");
  });
});
