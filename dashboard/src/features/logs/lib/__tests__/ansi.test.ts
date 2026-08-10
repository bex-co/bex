import { describe, it, expect } from "vitest";
import { needsAnsiParse, parseAnsi, stripAnsi, type AnsiSpan } from "../ansi";

const ESC = "\u001b";
const BEL = "\u0007";

const text = (spans: AnsiSpan[]) => spans.map((s) => s.text).join("");
const classOf = (spans: AnsiSpan[], needle: string) =>
  spans.find((s) => s.text.includes(needle))?.className ?? "";

describe("needsAnsiParse", () => {
  it("is false for a plain line, so the viewer keeps its fast path", () => {
    expect(needsAnsiParse("hello world")).toBe(false);
    expect(needsAnsiParse("")).toBe(false);
    // A literal bracket sequence with no ESC is real text, not a stray escape.
    expect(needsAnsiParse("array[2m] is fine")).toBe(false);
  });

  it("is true for an escape or a carriage return", () => {
    expect(needsAnsiParse(`${ESC}[33myellow`)).toBe(true);
    expect(needsAnsiParse("50%\r100%")).toBe(true);
  });
});

describe("parseAnsi SGR", () => {
  // The exact residue from the reported Tailwind/BuildKit build log: before
  // this, the viewer drew `[2m│   }[22m` because the browser paints ESC as
  // zero-width and only the parameter tail survived.
  it("leaves no parameter tail for the reported dim-rule line", () => {
    const spans = parseAnsi(`#11 94.34 ${ESC}[2m│   }${ESC}[22m`);
    expect(text(spans)).toBe("#11 94.34 │   }");
    expect(text(spans)).not.toContain("[2m");
    expect(text(spans)).not.toContain("[22m");
    expect(classOf(spans, "│")).toContain("opacity-70");
    // `22m` closed the dim run, so the prefix is unstyled.
    expect(classOf(spans, "#11")).toBe("");
  });

  it("nests dim inside a yellow run and closes both", () => {
    const spans = parseAnsi(
      `${ESC}[33m${ESC}[2m^--${ESC}[22m Unexpected token${ESC}[39m done`,
    );
    expect(text(spans)).toBe("^-- Unexpected token done");
    expect(classOf(spans, "^--")).toContain("opacity-70");
    expect(classOf(spans, "^--")).toContain("text-amber-700");
    // Dim ended but yellow did not.
    expect(classOf(spans, "Unexpected")).toContain("text-amber-700");
    expect(classOf(spans, "Unexpected")).not.toContain("opacity-70");
    // `39m` restored the default foreground.
    expect(classOf(spans, "done")).toBe("");
  });

  it("maps the basic and bright foreground palettes to theme-aware classes", () => {
    expect(classOf(parseAnsi(`${ESC}[31mred`), "red")).toBe(
      "text-red-700 dark:text-red-400",
    );
    expect(classOf(parseAnsi(`${ESC}[92mbright`), "bright")).toBe(
      "text-green-600 dark:text-green-300",
    );
  });

  it("treats 0m and a bare ESC[m as a full reset", () => {
    for (const reset of ["0m", "m"]) {
      const spans = parseAnsi(`${ESC}[1;4;31mloud${ESC}[${reset}quiet`);
      expect(text(spans)).toBe("loudquiet");
      expect(classOf(spans, "loud")).toContain("font-semibold");
      expect(classOf(spans, "loud")).toContain("underline");
      expect(classOf(spans, "quiet")).toBe("");
    }
  });

  it("renders the 256-color cube and truecolor inline", () => {
    // 196 is the cube's pure red.
    const cube = parseAnsi(`${ESC}[38;5;196mERR`);
    expect(cube[0].style?.color).toBe("rgb(255, 0, 0)");
    const grayscale = parseAnsi(`${ESC}[38;5;240mgray`);
    expect(grayscale[0].style?.color).toBe("rgb(88, 88, 88)");
    const truecolor = parseAnsi(`${ESC}[38;2;255;128;0morange`);
    expect(truecolor[0].style?.color).toBe("rgb(255, 128, 0)");
    // 0–15 stay on the theme-aware classes even via the extended form.
    const low = parseAnsi(`${ESC}[38;5;1mred`);
    expect(low[0].style?.color).toBeUndefined();
    expect(low[0].className).toContain("text-red-700");
  });

  it("renders backgrounds and does not honor inverse", () => {
    expect(parseAnsi(`${ESC}[41mblock`)[0].style?.backgroundColor).toBe(
      "rgb(205, 0, 0)",
    );
    expect(
      parseAnsi(`${ESC}[48;2;0;0;255mblue`)[0].style?.backgroundColor,
    ).toBe("rgb(0, 0, 255)");
    // Code 7 would need the panel's background to resolve; ignoring it degrades
    // to readable text rather than text on itself.
    const inverse = parseAnsi(`${ESC}[7mplain`);
    expect(inverse[0].className).toBe("");
    expect(inverse[0].style).toBeUndefined();
  });

  it("carries underline and strikethrough together inline", () => {
    const spans = parseAnsi(`${ESC}[4;9mboth`);
    expect(spans[0].style?.textDecorationLine).toBe("underline line-through");
  });

  it("coalesces a run into one span per style change", () => {
    const spans = parseAnsi(`plain ${ESC}[31mred text${ESC}[39m plain`);
    expect(spans.map((s) => s.text)).toEqual(["plain ", "red text", " plain"]);
  });
});

describe("parseAnsi non-SGR escapes", () => {
  it("swallows cursor moves, erase-display and private modes", () => {
    expect(text(parseAnsi(`${ESC}[1Aup${ESC}[2Jclear${ESC}[?25lhide`))).toBe(
      "upclearhide",
    );
  });

  it("keeps an OSC 8 hyperlink's label and drops its envelope", () => {
    const link = `${ESC}]8;;https://example.com${BEL}click me${ESC}]8;;${BEL}`;
    expect(text(parseAnsi(link))).toBe("click me");
    // The ST-terminated form too.
    const st = `${ESC}]0;window title${ESC}\\after`;
    expect(text(parseAnsi(st))).toBe("after");
  });

  it("swallows charset selection and other short escapes", () => {
    expect(text(parseAnsi(`${ESC}(Bplain${ESC}=more`))).toBe("plainmore");
  });

  it("never leaks bytes from a truncated or malformed sequence", () => {
    for (const input of [
      `done${ESC}`, // lone trailing ESC
      `done${ESC}[`, // truncated CSI
      `done${ESC}[38;5`, // truncated extended color
      `${ESC}[38;9;1mtext`, // unknown extended mode
      `${ESC}]8;;http://x`, // unterminated OSC
    ]) {
      const out = text(parseAnsi(input));
      expect(out).not.toContain(ESC);
      expect(out).not.toContain("[38");
    }
    expect(text(parseAnsi(`${ESC}[38;9;1mtext`))).toBe("text");
  });
});

describe("parseAnsi carriage returns", () => {
  it("overwrites in place instead of smearing progress lines down the viewer", () => {
    expect(text(parseAnsi("Progress:  10%\rProgress: 100%"))).toBe(
      "Progress: 100%",
    );
  });

  it("leaves the tail of a longer previous write visible, as a terminal does", () => {
    expect(text(parseAnsi("abcdef\rXY"))).toBe("XYcdef");
  });

  it("erase-in-line clears the overwritten tail", () => {
    expect(text(parseAnsi(`abcdef\rXY${ESC}[K`))).toBe("XY");
    expect(text(parseAnsi(`abcdef${ESC}[2Kfresh`))).toBe("fresh");
  });

  it("returns to the start of the current line, not of the message", () => {
    expect(text(parseAnsi("first\nsecond\rTHIRD!"))).toBe("first\nTHIRD!");
  });

  it("keeps styling aligned with the surviving characters", () => {
    const spans = parseAnsi(`${ESC}[31maaaa\r${ESC}[32mbb`);
    expect(text(spans)).toBe("bbaa");
    expect(classOf(spans, "bb")).toContain("text-green-700");
    expect(spans[1].className).toContain("text-red-700");
  });
});

describe("stripAnsi", () => {
  it("returns a plain line untouched", () => {
    const plain = "hello world";
    expect(stripAnsi(plain)).toBe(plain);
  });

  it("yields exactly what the reader sees, so search cannot miss a match", () => {
    // The needle straddles a color boundary — a raw substring search fails here.
    const line = `${ESC}[2m│${ESC}[22m     min-height: var(--feed-reserve-${ESC}[33m*${ESC}[39m);`;
    expect(stripAnsi(line)).toBe("│     min-height: var(--feed-reserve-*);");
    expect(stripAnsi(line).includes("reserve-*")).toBe(true);
    expect(line.includes("reserve-*")).toBe(false);
  });

  it("applies carriage-return overwrites", () => {
    expect(stripAnsi("stale\rfresh")).toBe("fresh");
  });
});
