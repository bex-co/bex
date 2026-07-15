import { describe, expect, it } from "vitest";
import { generateEnvValue } from "@/features/services/lib/generate-env-value";

describe("generateEnvValue", () => {
  it("returns fresh base64-encoded 256-bit values", () => {
    const first = generateEnvValue();
    const second = generateEnvValue();
    expect(first).toHaveLength(44);
    expect(second).toHaveLength(44);
    expect(second).not.toBe(first);
    expect(
      Uint8Array.from(globalThis.atob(first), (c) => c.charCodeAt(0)),
    ).toHaveLength(32);
  });
});
