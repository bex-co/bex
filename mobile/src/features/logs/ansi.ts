import type { TextStyle } from "react-native";
import { fontWeights } from "../../common/theme/typography";

// ANSI escape handling for mobile log lines.
//
// Matches the dashboard's interpreter (`dashboard/src/features/logs/lib/ansi.ts`)
// and bex-api's `textMode=display` projection (`lego/backend/internal/logs/display.go`):
// SGR becomes theme-aware React Native TextStyle, every other escape is swallowed,
// and carriage returns overwrite in place by Unicode code point.

const ESC = "\u001b";
const BEL = "\u0007";

export type AnsiColor =
  | { kind: "basic"; index: number }
  | { kind: "rgb"; r: number; g: number; b: number };

/** One run of text sharing a single resolved SGR state. */
export interface AnsiSpan {
  text: string;
  fg?: AnsiColor;
  bg?: AnsiColor;
  bold: boolean;
  dim: boolean;
  italic: boolean;
  underline: boolean;
  strike: boolean;
}

// The 16 ANSI foreground colors mapped onto light/dark theme-aware hex pairs
// matching the dashboard's Tailwind palette (`dashboard/src/features/logs/lib/ansi.ts`).
const FG_LIGHT: readonly string[] = [
  "#737373", // 0 black (neutral-500)
  "#b91c1c", // 1 red (red-700)
  "#15803d", // 2 green (green-700)
  "#b45309", // 3 yellow (amber-700)
  "#1d4ed8", // 4 blue (blue-700)
  "#a21caf", // 5 magenta (fuchsia-700)
  "#0e7490", // 6 cyan (cyan-700)
  "#404040", // 7 white (neutral-700)
  "#737373", // 8 bright black (neutral-500)
  "#dc2626", // 9 bright red (red-600)
  "#16a34a", // 10 bright green (green-600)
  "#d97706", // 11 bright yellow (amber-600)
  "#2563eb", // 12 bright blue (blue-600)
  "#c026d3", // 13 bright magenta (fuchsia-600)
  "#0891b2", // 14 bright cyan (cyan-600)
  "#171717", // 15 bright white (neutral-900)
];

const FG_DARK: readonly string[] = [
  "#a3a3a3", // 0 black (neutral-400)
  "#f87171", // 1 red (red-400)
  "#4ade80", // 2 green (green-400)
  "#fbbf24", // 3 yellow (amber-400)
  "#60a5fa", // 4 blue (blue-400)
  "#e879f9", // 5 magenta (fuchsia-400)
  "#22d3ee", // 6 cyan (cyan-400)
  "#d4d4d4", // 7 white (neutral-300)
  "#737373", // 8 bright black (neutral-500)
  "#fca5a5", // 9 bright red (red-300)
  "#86efac", // 10 bright green (green-300)
  "#fcd34d", // 11 bright yellow (amber-300)
  "#93c5fd", // 12 bright blue (blue-300)
  "#f0abfc", // 13 bright magenta (fuchsia-300)
  "#67e8f9", // 14 bright cyan (cyan-300)
  "#f5f5f5", // 15 bright white (neutral-100)
];

const BG_RGB: ReadonlyArray<readonly [number, number, number]> = [
  [0, 0, 0],
  [205, 0, 0],
  [0, 205, 0],
  [205, 205, 0],
  [0, 0, 238],
  [205, 0, 205],
  [0, 205, 205],
  [229, 229, 229],
  [127, 127, 127],
  [255, 0, 0],
  [0, 255, 0],
  [255, 255, 0],
  [92, 92, 255],
  [255, 0, 255],
  [0, 255, 255],
  [255, 255, 255],
];

interface Sgr {
  fg?: AnsiColor;
  bg?: AnsiColor;
  bold: boolean;
  dim: boolean;
  italic: boolean;
  underline: boolean;
  strike: boolean;
}

const RESET: Sgr = {
  bold: false,
  dim: false,
  italic: false,
  underline: false,
  strike: false,
};

interface Cell {
  ch: string;
  sgr: Sgr;
}

/**
 * Whether a message carries anything this module has to interpret. Plain lines
 * skip parsing entirely on the fast path.
 */
export function needsAnsiParse(input: string): boolean {
  return input.includes(ESC) || input.includes("\r");
}

/**
 * Interpret a log line into styled spans. Never throws and never returns
 * escape bytes: an unterminated or malformed sequence is swallowed with the
 * rest of the string.
 */
export function parseAnsi(input: string): AnsiSpan[] {
  const cells: Cell[] = [];
  let sgr = RESET;
  let cursor = 0;
  let lineStart = 0;

  for (let i = 0; i < input.length;) {
    const ch = input[i];

    if (ch === ESC) {
      const next = input[i + 1];

      if (next === "[") {
        let j = i + 2;
        while (j < input.length && isParamByte(input.charCodeAt(j))) j++;
        while (j < input.length && isIntermediateByte(input.charCodeAt(j))) j++;
        const final = input[j];
        const params = input.slice(i + 2, j);
        if (final === "m") {
          sgr = applySgr(sgr, params);
        } else if (final === "K") {
          const mode = params === "" ? 0 : Number.parseInt(params, 10) || 0;
          if (mode === 0) {
            cells.length = Math.min(cursor, cells.length);
          } else if (mode === 1) {
            for (let k = lineStart; k <= cursor && k < cells.length; k++) {
              cells[k] = { ch: " ", sgr };
            }
          } else if (mode === 2) {
            cells.length = lineStart;
            cursor = lineStart;
          }
        }
        i = j < input.length ? j + 1 : input.length;
        continue;
      }

      if (next === "]") {
        let j = i + 2;
        while (j < input.length) {
          if (input[j] === BEL) {
            j++;
            break;
          }
          if (input[j] === ESC && input[j + 1] === "\\") {
            j += 2;
            break;
          }
          j++;
        }
        i = j;
        continue;
      }

      if (next === undefined) {
        i = input.length;
        continue;
      }
      i += next === "(" || next === ")" || next === "#" || next === "%" ? 3 : 2;
      continue;
    }

    if (ch === "\r") {
      cursor = lineStart;
      i++;
      continue;
    }

    const char = String.fromCodePoint(input.codePointAt(i)!);
    writeCell(cells, cursor, char, sgr);
    cursor++;
    if (char === "\n") lineStart = cursor;
    i += char.length;
  }

  return coalesce(cells);
}

/**
 * The displayed text with all styling removed, matching bex-api's
 * `textMode=display` search projection.
 */
export function stripAnsi(input: string): string {
  if (!needsAnsiParse(input)) return input;
  let out = "";
  for (const span of parseAnsi(input)) out += span.text;
  return out;
}

/**
 * Resolve one parsed span into a React Native TextStyle for the current theme.
 * Returns `undefined` when the span carries no styling attributes.
 */
export function ansiSpanStyle(
  span: AnsiSpan,
  isDark: boolean,
): TextStyle | undefined {
  const style: TextStyle = {};
  let hasStyle = false;

  if (span.bold) {
    style.fontWeight = fontWeights.semibold;
    hasStyle = true;
  }
  if (span.italic) {
    style.fontStyle = "italic";
    hasStyle = true;
  }
  if (span.underline && span.strike) {
    style.textDecorationLine = "underline line-through";
    hasStyle = true;
  } else if (span.underline) {
    style.textDecorationLine = "underline";
    hasStyle = true;
  } else if (span.strike) {
    style.textDecorationLine = "line-through";
    hasStyle = true;
  }
  if (span.dim) {
    style.opacity = 0.7;
    hasStyle = true;
  }

  if (span.fg?.kind === "basic") {
    style.color = (isDark ? FG_DARK : FG_LIGHT)[span.fg.index];
    hasStyle = true;
  } else if (span.fg?.kind === "rgb") {
    style.color = css(span.fg.r, span.fg.g, span.fg.b);
    hasStyle = true;
  }

  if (span.bg?.kind === "basic") {
    const [r, g, b] = BG_RGB[span.bg.index];
    style.backgroundColor = css(r, g, b);
    hasStyle = true;
  } else if (span.bg?.kind === "rgb") {
    style.backgroundColor = css(span.bg.r, span.bg.g, span.bg.b);
    hasStyle = true;
  }

  return hasStyle ? style : undefined;
}

function isParamByte(code: number): boolean {
  return code >= 0x30 && code <= 0x3f;
}

function isIntermediateByte(code: number): boolean {
  return code >= 0x20 && code <= 0x2f;
}

function writeCell(cells: Cell[], at: number, ch: string, sgr: Sgr): void {
  while (cells.length < at) cells.push({ ch: " ", sgr: RESET });
  cells[at] = { ch, sgr };
}

const SGR_ATTRIBUTES: Partial<Record<number, Partial<Sgr>>> = {
  1: { bold: true },
  2: { dim: true },
  3: { italic: true },
  4: { underline: true },
  9: { strike: true },
  21: { bold: false, dim: false },
  22: { bold: false, dim: false },
  23: { italic: false },
  24: { underline: false },
  29: { strike: false },
  39: { fg: undefined },
  49: { bg: undefined },
};

function applySgr(base: Sgr, params: string): Sgr {
  if (params === "") return RESET;
  const codes = params
    .split(";")
    .map((p) => (p === "" ? 0 : Number.parseInt(p, 10)));
  let out: Sgr = { ...base };
  for (let i = 0; i < codes.length; i++) {
    const c = codes[i];
    if (!Number.isFinite(c)) continue;
    if (c === 0) {
      out = { ...RESET };
      continue;
    }
    const attributes = SGR_ATTRIBUTES[c];
    if (attributes) {
      Object.assign(out, attributes);
      continue;
    }
    if (c === 38 || c === 48) {
      const ext = readExtended(codes, i);
      if (ext.color) out[c === 38 ? "fg" : "bg"] = ext.color;
      i = ext.next;
      continue;
    }
    if (c >= 30 && c <= 37) out.fg = { kind: "basic", index: c - 30 };
    else if (c >= 40 && c <= 47) out.bg = { kind: "basic", index: c - 40 };
    else if (c >= 90 && c <= 97) out.fg = { kind: "basic", index: c - 90 + 8 };
    else if (c >= 100 && c <= 107)
      out.bg = { kind: "basic", index: c - 100 + 8 };
  }
  return out;
}

function readExtended(
  codes: number[],
  i: number,
): { color?: AnsiColor; next: number } {
  const mode = codes[i + 1];
  if (mode === 5) return { color: paletteColor(codes[i + 2]), next: i + 2 };
  if (mode === 2) {
    return {
      color: rgbColor(codes[i + 2], codes[i + 3], codes[i + 4]),
      next: i + 4,
    };
  }
  return { next: i };
}

function paletteColor(n: number): AnsiColor | undefined {
  if (!Number.isInteger(n) || n < 0 || n > 255) return undefined;
  if (n < 16) return { kind: "basic", index: n };
  if (n < 232) {
    const c = n - 16;
    const level = (v: number) => (v === 0 ? 0 : 55 + v * 40);
    return {
      kind: "rgb",
      r: level(Math.floor(c / 36)),
      g: level(Math.floor(c / 6) % 6),
      b: level(c % 6),
    };
  }
  const v = 8 + (n - 232) * 10;
  return { kind: "rgb", r: v, g: v, b: v };
}

function rgbColor(r: number, g: number, b: number): AnsiColor | undefined {
  const ok = (v: number) => Number.isInteger(v) && v >= 0 && v <= 255;
  return ok(r) && ok(g) && ok(b) ? { kind: "rgb", r, g, b } : undefined;
}

function css(r: number, g: number, b: number): string {
  return `rgb(${r}, ${g}, ${b})`;
}

function sameColor(
  a: AnsiColor | undefined,
  b: AnsiColor | undefined,
): boolean {
  if (a === b) return true;
  if (!a || !b || a.kind !== b.kind) return false;
  if (a.kind === "basic" && b.kind === "basic") return a.index === b.index;
  if (a.kind === "rgb" && b.kind === "rgb") {
    return a.r === b.r && a.g === b.g && a.b === b.b;
  }
  return false;
}

function sameSgr(a: Sgr, b: Sgr): boolean {
  if (a === b) return true;
  return (
    a.bold === b.bold &&
    a.dim === b.dim &&
    a.italic === b.italic &&
    a.underline === b.underline &&
    a.strike === b.strike &&
    sameColor(a.fg, b.fg) &&
    sameColor(a.bg, b.bg)
  );
}

function coalesce(cells: Cell[]): AnsiSpan[] {
  const spans: AnsiSpan[] = [];
  for (const cell of cells) {
    const last = spans[spans.length - 1];
    if (last && sameSgr(last, cell.sgr)) {
      last.text += cell.ch;
    } else {
      spans.push({
        text: cell.ch,
        fg: cell.sgr.fg,
        bg: cell.sgr.bg,
        bold: cell.sgr.bold,
        dim: cell.sgr.dim,
        italic: cell.sgr.italic,
        underline: cell.sgr.underline,
        strike: cell.sgr.strike,
      });
    }
  }
  return spans;
}
