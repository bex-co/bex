import { describe, expect, it } from "vitest";
import { formatEnvExport } from "../env-export";

describe("formatEnvExport", () => {
  it("sorts keys and quotes every value deterministically", () => {
    expect(
      formatEnvExport([
        { key: "Z_LAST", value: "line one\nline two" },
        { key: "A_FIRST", value: 'spaces and "quotes"' },
        { key: "EMPTY", value: "" },
      ]),
    ).toBe(
      'A_FIRST="spaces and \\"quotes\\""\nEMPTY=""\nZ_LAST="line one\\nline two"\n',
    );
  });

  it("renders an empty environment as an empty file", () => {
    expect(formatEnvExport([])).toBe("");
  });
});
