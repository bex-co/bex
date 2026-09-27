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

  // w4/159: a quoted value may span lines, as dotenv parses a PEM key.
  it("keeps a literal multi-line quoted value with its line breaks", () => {
    const text = [
      "QA_URL=https://example.com/a#frag",
      "export QA_EXPORTED=yes",
      "QA_EQ=a=b=c",
      'QA_PEM="-----BEGIN KEY-----',
      "MIIBOgIBAAJBAKj",
      '-----END KEY-----"',
      "QA_AFTER=1",
      "QA_SINGLE='line one",
      "  line two'",
    ].join("\n");
    expect(parseDotenv(text)).toEqual([
      { key: "QA_URL", value: "https://example.com/a#frag", line: 1 },
      { key: "QA_EXPORTED", value: "yes", line: 2 },
      { key: "QA_EQ", value: "a=b=c", line: 3 },
      {
        key: "QA_PEM",
        value: "-----BEGIN KEY-----\nMIIBOgIBAAJBAKj\n-----END KEY-----",
        line: 4,
      },
      { key: "QA_AFTER", value: "1", line: 7 },
      { key: "QA_SINGLE", value: "line one\n  line two", line: 8 },
    ]);
  });

  it("names the opening line of a quote that never closes", () => {
    expect.assertions(2);
    try {
      parseDotenv('OK=1\nQA_PEM="-----BEGIN KEY-----\nMIIB\nNEXT=2');
    } catch (error) {
      expect((error as DotenvParseError).reason).toBe("quote");
      expect((error as DotenvParseError).line).toBe(2);
    }
  });

  it("names the physical line of text after a multi-line value's closing quote", () => {
    expect.assertions(2);
    try {
      parseDotenv('A="one\ntwo" stray\nB=2');
    } catch (error) {
      expect((error as DotenvParseError).reason).toBe("trailing");
      expect((error as DotenvParseError).line).toBe(2);
    }
  });

  it("still ends an unquoted value at its line", () => {
    expect(parseDotenv("A=one\ntwo=2")).toEqual([
      { key: "A", value: "one", line: 1 },
      { key: "two", value: "2", line: 2 },
    ]);
  });
});
