import { describe, expect, it } from "vitest";
import { DotenvParseError, parseDotenv } from "../dotenv-import";

describe("parseDotenv", () => {
  it("parses comments, export, quotes, CRLF, and embedded equals", () => {
    expect(
      parseDotenv(
        "# ignored\r\nexport A=\"hello\\nworld\"\r\nB=left=right\r\nC=plain # note\r\nD='single=quoted'",
      ),
    ).toEqual([
      { key: "A", value: "hello\nworld", line: 2 },
      { key: "B", value: "left=right", line: 3 },
      { key: "C", value: "plain", line: 4 },
      { key: "D", value: "single=quoted", line: 5 },
    ]);
  });

  it("uses the last duplicate assignment deterministically", () => {
    expect(parseDotenv("A=first\nA=second")).toEqual([
      { key: "A", value: "second", line: 2 },
    ]);
  });

  it("reports a bad line without echoing its value", () => {
    expect.assertions(3);
    try {
      parseDotenv("OK=one\nBAD KEY=must-not-leak");
    } catch (error) {
      expect(error).toBeInstanceOf(DotenvParseError);
      expect((error as DotenvParseError).line).toBe(2);
      expect((error as Error).message).not.toContain("must-not-leak");
    }
  });
});
