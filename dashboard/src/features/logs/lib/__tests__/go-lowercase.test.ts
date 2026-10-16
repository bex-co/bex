import { describe, expect, it } from "vitest";

import { readRepoVectors } from "@/test/repo-vectors";
import { GO_LOWERCASE_RUNS, goLowercase } from "../go-lowercase";

// The shared table: lego/backend/internal/logs/lowercase_vectors_test.go pins
// it to Go's unicode.ToLower over every scalar value; this pins the dashboard's
// copy to it, so deploy search cannot drift from server log search (w4/m216).
const RUNS = readRepoVectors<[number, number, number, number]>(
  "lego/backend/internal/logs/testdata/lowercase-runs.json",
);

describe("goLowercase", () => {
  it("carries exactly the Go-generated table", () => {
    expect(GO_LOWERCASE_RUNS).toEqual(RUNS);
  });

  it.each([
    // a743: Go folds Σ to σ wherever it stands; JavaScript makes a final ς.
    ["QA_A743 ΟΣ", "qa_a743 οσ"],
    ["QA_A743 ος", "qa_a743 ος"],
    ["Alpha", "alpha"],
    // Dotted capital I is one code point to Go, two to JavaScript.
    ["İSTANBUL", "istanbul"],
    ["\u212A", "k"], // KELVIN SIGN
    ["ПРИВЕТ", "привет"],
    ["\u{10400}", "\u{10428}"], // DESERET, outside the BMP
    ["ǅ", "ǆ"],
    ["日本 [x] (y)*", "日本 [x] (y)*"], // no mapping, literal punctuation
  ])("%s → %s", (input, want) => {
    expect(goLowercase(input)).toBe(want);
  });

  it("never changes the number of code points", () => {
    const text = "İ ΟΣ ẞ ǅ \u{1E900}";
    expect(Array.from(goLowercase(text))).toHaveLength(Array.from(text).length);
  });
});
