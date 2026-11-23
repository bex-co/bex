import fs from "node:fs";
import path from "node:path";
import {
  ansiSpanStyle,
  needsAnsiParse,
  parseAnsi,
  stripAnsi,
  type AnsiSpan,
} from "../ansi";

const ESC = "\u001b";
const BEL = "\u0007";

const text = (spans: AnsiSpan[]) => spans.map((s) => s.text).join("");
const spanOf = (spans: AnsiSpan[], needle: string) =>
  spans.find((s) => s.text.includes(needle));
const styleOf = (spans: AnsiSpan[], needle: string, isDark = false) => {
  const span = spanOf(spans, needle);
  return span ? ansiSpanStyle(span, isDark) : undefined;
};

describe("needsAnsiParse", () => {
  it("is false for a plain line, so the viewer keeps its fast path", () => {
    expect(needsAnsiParse("hello world")).toBe(false);
    expect(needsAnsiParse("")).toBe(false);
    expect(needsAnsiParse("array[2m] is fine")).toBe(false);
  });

  it("is true for an escape or a carriage return", () => {
    expect(needsAnsiParse(`${ESC}[33myellow`)).toBe(true);
    expect(needsAnsiParse("50%\r100%")).toBe(true);
  });
});

describe("parseAnsi SGR", () => {
  it("leaves no parameter tail for dim rules", () => {
    const spans = parseAnsi(`#11 94.34 ${ESC}[2m│   }${ESC}[22m`);
    expect(text(spans)).toBe("#11 94.34 │   }");
    expect(text(spans)).not.toContain("[2m");
    expect(text(spans)).not.toContain("[22m");
    expect(styleOf(spans, "│")?.opacity).toBe(0.7);
    expect(styleOf(spans, "#11")).toBe(undefined);
  });

  it("nests dim inside a yellow run and closes both", () => {
    const spans = parseAnsi(
      `${ESC}[33m${ESC}[2m^--${ESC}[22m Unexpected token${ESC}[39m done`,
    );
    expect(text(spans)).toBe("^-- Unexpected token done");
    expect(styleOf(spans, "^--")?.opacity).toBe(0.7);
    expect(styleOf(spans, "^--")?.color).toBe("#b45309");
    expect(styleOf(spans, "Unexpected")?.color).toBe("#b45309");
    expect(styleOf(spans, "Unexpected")?.opacity).toBe(undefined);
    expect(styleOf(spans, "done")).toBe(undefined);
  });

  it("maps basic and bright foreground palettes for light and dark themes", () => {
    const red = parseAnsi(`${ESC}[31mred`);
    expect(ansiSpanStyle(red[0], false)?.color).toBe("#b91c1c");
    expect(ansiSpanStyle(red[0], true)?.color).toBe("#f87171");

    const brightGreen = parseAnsi(`${ESC}[92mbright`);
    expect(ansiSpanStyle(brightGreen[0], false)?.color).toBe("#16a34a");
    expect(ansiSpanStyle(brightGreen[0], true)?.color).toBe("#86efac");
  });

  it("treats 0m and a bare ESC[m as a full reset", () => {
    for (const reset of ["0m", "m"]) {
      const spans = parseAnsi(`${ESC}[1;4;31mloud${ESC}[${reset}quiet`);
      expect(text(spans)).toBe("loudquiet");
      expect(styleOf(spans, "loud")?.fontWeight).toBe("600");
      expect(styleOf(spans, "loud")?.textDecorationLine).toBe("underline");
      expect(styleOf(spans, "quiet")).toBe(undefined);
    }
  });

  it("renders 256-color cube, grayscale, and truecolor", () => {
    const cube = parseAnsi(`${ESC}[38;5;196mERR`);
    expect(ansiSpanStyle(cube[0], false)?.color).toBe("rgb(255, 0, 0)");
    const grayscale = parseAnsi(`${ESC}[38;5;240mgray`);
    expect(ansiSpanStyle(grayscale[0], false)?.color).toBe("rgb(88, 88, 88)");
    const truecolor = parseAnsi(`${ESC}[38;2;255;128;0morange`);
    expect(ansiSpanStyle(truecolor[0], false)?.color).toBe("rgb(255, 128, 0)");
    const low = parseAnsi(`${ESC}[38;5;1mred`);
    expect(ansiSpanStyle(low[0], false)?.color).toBe("#b91c1c");
    expect(ansiSpanStyle(low[0], true)?.color).toBe("#f87171");
  });

  it("renders backgrounds and ignores inverse", () => {
    expect(
      ansiSpanStyle(parseAnsi(`${ESC}[41mblock`)[0], false)?.backgroundColor,
    ).toBe("rgb(205, 0, 0)");
    expect(
      ansiSpanStyle(parseAnsi(`${ESC}[48;2;0;0;255mblue`)[0], false)
        ?.backgroundColor,
    ).toBe("rgb(0, 0, 255)");
    expect(ansiSpanStyle(parseAnsi(`${ESC}[7mplain`)[0], false)).toBe(
      undefined,
    );
  });

  it("carries underline and strikethrough together", () => {
    const spans = parseAnsi(`${ESC}[4;9mboth`);
    expect(ansiSpanStyle(spans[0], false)?.textDecorationLine).toBe(
      "underline line-through",
    );
  });

  it("resets individual attributes without clearing colors", () => {
    const cases: Array<[string, string, string]> = [
      ["1;2", "21", "fontWeight"],
      ["1;2", "22", "opacity"],
      ["3", "23", "fontStyle"],
      ["4", "24", "textDecorationLine"],
      ["9", "29", "textDecorationLine"],
    ];
    for (const [on, off, prop] of cases) {
      const spans = parseAnsi(`${ESC}[31;44;${on}mbefore${ESC}[${off}mafter`);
      const before = styleOf(spans, "before") as Record<string, unknown>;
      const after = styleOf(spans, "after") as Record<string, unknown>;
      expect(before[prop] !== undefined).toBe(true);
      expect(after[prop]).toBe(undefined);
      expect(after.color).toBe("#b91c1c");
      expect(after.backgroundColor).toBe("rgb(0, 0, 238)");
    }
  });

  it("coalesces a run into one span per style change", () => {
    const spans = parseAnsi(`plain ${ESC}[31mred text${ESC}[39m plain`);
    expect(spans.map((s) => s.text)).toEqual(["plain ", "red text", " plain"]);
  });
});

describe("parseAnsi non-SGR escapes and carriage returns", () => {
  it("swallows non-SGR escapes, OSC envelopes, and truncated sequences", () => {
    expect(text(parseAnsi(`${ESC}[1Aup${ESC}[2Jclear${ESC}[?25lhide`))).toBe(
      "upclearhide",
    );
    expect(
      text(
        parseAnsi(
          `${ESC}]8;;https://example.com${BEL}click me${ESC}]8;;${BEL}`,
        ),
      ),
    ).toBe("click me");
    expect(text(parseAnsi(`${ESC}]0;window title${ESC}\\after`))).toBe("after");
    expect(text(parseAnsi(`${ESC}(Bplain${ESC}=more`))).toBe("plainmore");
    for (const input of [
      `done${ESC}`,
      `done${ESC}[`,
      `done${ESC}[38;5`,
      `${ESC}[38;9;1mtext`,
      `${ESC}]8;;http://x`,
    ]) {
      const out = text(parseAnsi(input));
      expect(out).not.toContain(ESC);
      expect(out).not.toContain("[38");
    }
  });

  it("applies carriage-return overwrites and erase-in-line", () => {
    expect(text(parseAnsi("Progress:  10%\rProgress: 100%"))).toBe(
      "Progress: 100%",
    );
    expect(text(parseAnsi("abcdef\rXY"))).toBe("XYcdef");
    expect(text(parseAnsi(`abcdef\rXY${ESC}[K`))).toBe("XY");
    expect(text(parseAnsi(`abcdef${ESC}[2Kfresh`))).toBe("fresh");
    expect(text(parseAnsi("first\nsecond\rTHIRD!"))).toBe("first\nTHIRD!");
    const styled = parseAnsi(`${ESC}[31maaaa\r${ESC}[32mbb`);
    expect(text(styled)).toBe("bbaa");
    expect(styleOf(styled, "bb")?.color).toBe("#15803d");
    expect(styleOf(styled, "aa")?.color).toBe("#b91c1c");
  });
});

describe("shared display vectors (lego/backend/internal/logs/testdata/display-text-vectors.json)", () => {
  const vectorsPath = path.resolve(
    __dirname,
    "../../../../../lego/backend/internal/logs/testdata/display-text-vectors.json",
  );
  const vectors = JSON.parse(fs.readFileSync(vectorsPath, "utf8")) as Array<{
    name: string;
    in: string;
    want: string;
  }>;

  it("matches every shared display-text vector", () => {
    expect(vectors.length > 0).toBe(true);
    for (const v of vectors) {
      expect(stripAnsi(v.in)).toBe(v.want);
      expect(text(parseAnsi(v.in))).toBe(v.want);
    }
  });
});

describe("Render's `==>` forms (w32/m576, w32/094)", () => {
  const forms = [
    {
      name: "platform step",
      line: `${ESC}[0;34m${ESC}[1m==> ${ESC}[0m${ESC}[1mDeploying...${ESC}[0m`,
      shown: "==> Deploying...",
      lightMarker: "#1d4ed8",
      darkMarker: "#60a5fa",
      text: "Deploying...",
      textColored: false,
    },
    {
      name: "platform success",
      line: `${ESC}[0;32m${ESC}[1m==> ${ESC}[0m${ESC}[1mYour service is live 🎉${ESC}[0m`,
      shown: "==> Your service is live 🎉",
      lightMarker: "#15803d",
      darkMarker: "#4ade80",
      text: "Your service is live",
      textColored: false,
    },
    {
      name: "platform failure",
      line: `${ESC}[0;31m${ESC}[1m==> Exited with status 1${ESC}[0m`,
      shown: "==> Exited with status 1",
      lightMarker: "#b91c1c",
      darkMarker: "#f87171",
      text: "Exited with status 1",
      textColored: true,
    },
    {
      name: "build shell",
      line: `${ESC}[34m${ESC}[1m==>${ESC}(B${ESC}[m ${ESC}[1mDownloading cache...${ESC}(B${ESC}[m`,
      shown: "==> Downloading cache...",
      lightMarker: "#1d4ed8",
      darkMarker: "#60a5fa",
      text: "Downloading cache...",
      textColored: false,
    },
    {
      name: "build runtime",
      line: `${ESC}[34;1m==>${ESC}[0;22m ${ESC}[1mUploading build...${ESC}[22m`,
      shown: "==> Uploading build...",
      lightMarker: "#1d4ed8",
      darkMarker: "#60a5fa",
      text: "Uploading build...",
      textColored: false,
    },
    {
      name: "build runtime success",
      line: `${ESC}[32;1m==>${ESC}[0;22m ${ESC}[1mBuild successful 🎉${ESC}[22m`,
      shown: "==> Build successful 🎉",
      lightMarker: "#15803d",
      darkMarker: "#4ade80",
      text: "Build successful",
      textColored: false,
    },
    {
      name: "instance",
      line: `${ESC}[32m${ESC}[1m==>${ESC}(B${ESC}[m ${ESC}[1mRunning 'node main.js'${ESC}(B${ESC}[m`,
      shown: "==> Running 'node main.js'",
      lightMarker: "#15803d",
      darkMarker: "#4ade80",
      text: "Running 'node main.js'",
      textColored: false,
    },
  ];

  it("colours the marker, bolds the text, leaves no escape bytes or tails, and closes cleanly", () => {
    for (const f of forms) {
      const spans = parseAnsi(`${f.line}after`);
      expect(text(spans)).toBe(`${f.shown}after`);
      expect(stripAnsi(f.line)).toBe(f.shown);
      expect(text(spans)).not.toContain(ESC);
      expect(text(spans)).not.toContain("(B");
      expect(/\[[0-9;]*m/.test(text(spans))).toBe(false);

      const lightMarker = styleOf(spans, "==>", false);
      const darkMarker = styleOf(spans, "==>", true);
      expect(lightMarker?.color).toBe(f.lightMarker);
      expect(darkMarker?.color).toBe(f.darkMarker);
      expect(lightMarker?.fontWeight).toBe("600");

      const lightWords = styleOf(spans, f.text, false);
      expect(lightWords?.fontWeight).toBe("600");
      expect(lightWords?.color).toBe(f.textColored ? f.lightMarker : undefined);

      expect(styleOf(spans, "after", false)).toBe(undefined);
    }
  });

  it("draws the blank line of Render's URL block as its green bold marker alone", () => {
    const spans = parseAnsi(
      `${ESC}[0;32m${ESC}[1m==> ${ESC}[0m${ESC}[1m${ESC}[0m`,
    );
    expect(text(spans)).toBe("==> ");
    expect(styleOf(spans, "==>", false)?.color).toBe("#15803d");
    expect(styleOf(spans, "==>", false)?.fontWeight).toBe("600");
  });
});
