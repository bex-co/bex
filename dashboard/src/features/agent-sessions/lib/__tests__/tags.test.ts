import { describe, expect, it } from "vitest";
import { parseTagInput } from "@/features/agent-sessions/lib/tags";

describe("parseTagInput (w7/m296)", () => {
  it("splits on commas and whitespace and drops empty pieces", () => {
    expect(parseTagInput(" bug, ui  triage,,\np1 ")).toEqual([
      "bug",
      "ui",
      "triage",
      "p1",
    ]);
  });

  it("reads an empty field as clearing the tags", () => {
    expect(parseTagInput("  , ")).toEqual([]);
  });

  it("leaves validation to the server rather than silently fixing input", () => {
    expect(parseTagInput("Bad")).toEqual(["Bad"]);
  });
});
