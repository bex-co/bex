// ANSI escape handling for log lines.
//
// Build tools (BuildKit, Vite, Tailwind, npm) colorize their output with ANSI
// escape sequences, and bex's log pipeline is byte-transparent: the ESC (0x1b)
// byte survives the pod's stdout, Loki, JSON, and GraphQL fully intact. The
// browser then paints ESC as zero-width, so drawing `line.message` as plain
// text left only the *parameter tail* visible — the literal `[2m` / `[22m` /
// `[33m` garbage interleaved through every build log.
//
// So this is not an encoding bug and stripping is not the fix: the colors are
// load-bearing (Vite's `^--` error caret and BuildKit's step rules are yellow
// on dim for a reason). This module interprets the sequences instead of leaking
// them — SGR becomes theme-aware styling, every other escape is swallowed, and
// carriage returns get terminal overwrite semantics.
//
// Deliberately client-side only: the wire format is shared with REST/GraphQL/
// MCP and the official Render CLI, which colorizes on its own TTY. Stripping in
// bex-api would be lossy, irreversible for already-stored history, and would
// break CLI parity.

const ESC = "\u001b";
const BEL = "\u0007";

/** Inline colors, for the palette entries that have no theme-aware class. */
export interface AnsiStyle {
  color?: string;
  backgroundColor?: string;
  textDecorationLine?: string;
}

/** One run of text sharing a single resolved style. */
export interface AnsiSpan {
  text: string;
  /** Tailwind classes: the theme-aware 16-color palette + text decorations. */
  className: string;
  /** Set only for the 256-color cube / truecolor, which cannot be expressed as
   *  a static class. Undefined when the className carries the whole style. */
  style?: AnsiStyle;
}

// The 16 ANSI foreground colors mapped onto theme-aware Tailwind pairs rather
// than their literal terminal RGB. Raw ANSI assumes a dark terminal: bright
// yellow and bright white are unreadable on the light theme, and ANSI black is
// invisible on the dark one. These follow the same light/dark shade convention
// as the request-status chips in log-line-list.tsx.
const FG_CLASS = [
  "text-neutral-500 dark:text-neutral-400", // 0 black
  "text-red-700 dark:text-red-400", // 1 red
  "text-green-700 dark:text-green-400", // 2 green
  "text-amber-700 dark:text-amber-400", // 3 yellow
  "text-blue-700 dark:text-blue-400", // 4 blue
  "text-fuchsia-700 dark:text-fuchsia-400", // 5 magenta
  "text-cyan-700 dark:text-cyan-400", // 6 cyan
  "text-neutral-700 dark:text-neutral-300", // 7 white
  "text-neutral-500 dark:text-neutral-500", // 8 bright black
  "text-red-600 dark:text-red-300", // 9 bright red
  "text-green-600 dark:text-green-300", // 10 bright green
  "text-amber-600 dark:text-amber-300", // 11 bright yellow
  "text-blue-600 dark:text-blue-300", // 12 bright blue
  "text-fuchsia-600 dark:text-fuchsia-300", // 13 bright magenta
  "text-cyan-600 dark:text-cyan-300", // 14 bright cyan
  "text-neutral-900 dark:text-neutral-100", // 15 bright white
];

// Backgrounds have no honest theme-aware mapping (a tool that paints a red
// block means a red block), so the 16 base backgrounds use the standard xterm
// RGB directly.
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

type Color =
  | { kind: "basic"; index: number } // 0..15 — resolved through the tables above
  | { kind: "rgb"; r: number; g: number; b: number };

interface Sgr {
  fg?: Color;
  bg?: Color;
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
 * Whether a message carries anything this module has to interpret. The fast
 * path matters: most app log lines are plain, and the viewer re-renders on
 * every live-tail frame — a plain line skips parsing entirely and renders as
 * the same single text node it always did.
 *
 * Carriage return counts: under `white-space: pre-wrap` a bare `\r` is a
 * segment break, so a progress-bar line would otherwise smear down the viewer
 * instead of overwriting in place.
 */
export function needsAnsiParse(input: string): boolean {
  return input.includes(ESC) || input.includes("\r");
}

/**
 * Interpret a log line into styled spans. Never throws and never returns
 * escape bytes: an unterminated or malformed sequence is swallowed with the
 * rest of the string, so the worst case is losing styling, not leaking `[2m`.
 */
export function parseAnsi(input: string): AnsiSpan[] {
  const cells: Cell[] = [];
  let sgr = RESET;
  let cursor = 0;
  // Where a `\r` returns to. A message is normally one line, but the deploy
  // panel does merge multi-line frames, so track the start of the current one.
  let lineStart = 0;

  for (let i = 0; i < input.length; ) {
    const ch = input[i];

    if (ch === ESC) {
      const next = input[i + 1];

      if (next === "[") {
        // CSI: parameter bytes 0x30–0x3f, intermediate bytes 0x20–0x2f, then a
        // final byte 0x40–0x7e. Only SGR (`m`) and erase-in-line (`K`) mean
        // anything here; cursor moves, scroll regions and private modes are
        // consumed and dropped.
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
        // OSC (window titles, OSC 8 hyperlinks): terminated by BEL or ST.
        // Dropping the envelope keeps a hyperlink's visible label, which is
        // the part the reader wants.
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
        i = input.length; // trailing lone ESC
        continue;
      }
      // Charset selection (`ESC ( B`) is three bytes; the rest of the two-byte
      // escapes are two. Either way: swallowed.
      i += next === "(" || next === ")" || next === "#" || next === "%" ? 3 : 2;
      continue;
    }

    if (ch === "\r") {
      cursor = lineStart;
      i++;
      continue;
    }

    writeCell(cells, cursor, ch, sgr);
    cursor++;
    if (ch === "\n") lineStart = cursor;
    i++;
  }

  return coalesce(cells);
}

/**
 * The displayed text with all styling removed — what search, and anything else
 * matching against what the user can actually see, must compare. Carriage
 * returns are applied here too, so a match can't hide in overwritten text.
 */
export function stripAnsi(input: string): string {
  if (!needsAnsiParse(input)) return input;
  let out = "";
  for (const span of parseAnsi(input)) out += span.text;
  return out;
}

function isParamByte(code: number): boolean {
  return code >= 0x30 && code <= 0x3f;
}

function isIntermediateByte(code: number): boolean {
  return code >= 0x20 && code <= 0x2f;
}

function writeCell(cells: Cell[], at: number, ch: string, sgr: Sgr): void {
  // The cursor only ever advances one past the end, but pad defensively so an
  // exotic sequence can't punch a hole of `undefined` into the array.
  while (cells.length < at) cells.push({ ch: " ", sgr: RESET });
  cells[at] = { ch, sgr };
}

function applySgr(base: Sgr, params: string): Sgr {
  if (params === "") return RESET; // bare `ESC[m` is a reset
  const codes = params
    .split(";")
    .map((p) => (p === "" ? 0 : Number.parseInt(p, 10)));
  let out: Sgr = { ...base };
  for (let i = 0; i < codes.length; i++) {
    const c = codes[i];
    if (!Number.isFinite(c)) continue; // private/experimental params
    if (c === 0) out = { ...RESET };
    else if (c === 1) out.bold = true;
    else if (c === 2) out.dim = true;
    else if (c === 3) out.italic = true;
    else if (c === 4) out.underline = true;
    else if (c === 9) out.strike = true;
    else if (c === 21 || c === 22) {
      out.bold = false;
      out.dim = false;
    } else if (c === 23) out.italic = false;
    else if (c === 24) out.underline = false;
    else if (c === 29) out.strike = false;
    else if (c >= 30 && c <= 37) out.fg = { kind: "basic", index: c - 30 };
    else if (c === 38) {
      const ext = readExtended(codes, i);
      if (ext.color) out.fg = ext.color;
      i = ext.next;
    } else if (c === 39) out.fg = undefined;
    else if (c >= 40 && c <= 47) out.bg = { kind: "basic", index: c - 40 };
    else if (c === 48) {
      const ext = readExtended(codes, i);
      if (ext.color) out.bg = ext.color;
      i = ext.next;
    } else if (c === 49) out.bg = undefined;
    else if (c >= 90 && c <= 97) out.fg = { kind: "basic", index: c - 90 + 8 };
    else if (c >= 100 && c <= 107)
      out.bg = { kind: "basic", index: c - 100 + 8 };
    // Everything else (inverse, conceal, font selection, framing) is ignored on
    // purpose: honoring inverse would mean guessing the panel's background and
    // could render text against itself.
  }
  return out;
}

// 38/48 introduce a sub-sequence: `5;<n>` (256-color) or `2;<r>;<g>;<b>`
// (truecolor). Returns the index of the last parameter consumed.
function readExtended(
  codes: number[],
  i: number,
): { color?: Color; next: number } {
  const mode = codes[i + 1];
  if (mode === 5) return { color: paletteColor(codes[i + 2]), next: i + 2 };
  if (mode === 2) {
    return {
      color: rgbColor(codes[i + 2], codes[i + 3], codes[i + 4]),
      next: i + 4,
    };
  }
  return { next: i }; // malformed — consume nothing past the 38/48 itself
}

function paletteColor(n: number): Color | undefined {
  if (!Number.isInteger(n) || n < 0 || n > 255) return undefined;
  if (n < 16) return { kind: "basic", index: n };
  if (n < 232) {
    // The 6×6×6 color cube.
    const c = n - 16;
    const level = (v: number) => (v === 0 ? 0 : 55 + v * 40);
    return {
      kind: "rgb",
      r: level(Math.floor(c / 36)),
      g: level(Math.floor(c / 6) % 6),
      b: level(c % 6),
    };
  }
  const v = 8 + (n - 232) * 10; // the 24-step grayscale ramp
  return { kind: "rgb", r: v, g: v, b: v };
}

function rgbColor(r: number, g: number, b: number): Color | undefined {
  const ok = (v: number) => Number.isInteger(v) && v >= 0 && v <= 255;
  return ok(r) && ok(g) && ok(b) ? { kind: "rgb", r, g, b } : undefined;
}

function css(r: number, g: number, b: number): string {
  return `rgb(${r}, ${g}, ${b})`;
}

// One resolved style per distinct SGR state. Consecutive cells share the SGR
// object by reference, so this is computed once per style change per line.
const attrCache = new WeakMap<Sgr, { className: string; style?: AnsiStyle }>();

function attrsFor(sgr: Sgr): { className: string; style?: AnsiStyle } {
  const hit = attrCache.get(sgr);
  if (hit) return hit;

  const classes: string[] = [];
  const style: AnsiStyle = {};

  if (sgr.bold) classes.push("font-semibold");
  if (sgr.italic) classes.push("italic");
  // Tailwind's `underline` and `line-through` both write text-decoration-line,
  // so the pair has to go inline to survive together.
  if (sgr.underline && sgr.strike) {
    style.textDecorationLine = "underline line-through";
  } else if (sgr.underline) classes.push("underline");
  else if (sgr.strike) classes.push("line-through");
  // Dim is the single most common code in build output (BuildKit's `│` rules,
  // Vite's context lines) — opacity reads as dim against any theme.
  if (sgr.dim) classes.push("opacity-70");

  if (sgr.fg?.kind === "basic") classes.push(FG_CLASS[sgr.fg.index]);
  else if (sgr.fg?.kind === "rgb")
    style.color = css(sgr.fg.r, sgr.fg.g, sgr.fg.b);

  if (sgr.bg?.kind === "basic") {
    const [r, g, b] = BG_RGB[sgr.bg.index];
    style.backgroundColor = css(r, g, b);
  } else if (sgr.bg?.kind === "rgb") {
    style.backgroundColor = css(sgr.bg.r, sgr.bg.g, sgr.bg.b);
  }

  const attrs = {
    className: classes.join(" "),
    style: Object.keys(style).length > 0 ? style : undefined,
  };
  attrCache.set(sgr, attrs);
  return attrs;
}

function sameStyle(
  a: AnsiStyle | undefined,
  b: AnsiStyle | undefined,
): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  return (
    a.color === b.color &&
    a.backgroundColor === b.backgroundColor &&
    a.textDecorationLine === b.textDecorationLine
  );
}

function coalesce(cells: Cell[]): AnsiSpan[] {
  const spans: AnsiSpan[] = [];
  for (const cell of cells) {
    const attrs = attrsFor(cell.sgr);
    const last = spans[spans.length - 1];
    if (
      last &&
      last.className === attrs.className &&
      sameStyle(last.style, attrs.style)
    ) {
      last.text += cell.ch;
    } else {
      spans.push({
        text: cell.ch,
        className: attrs.className,
        style: attrs.style,
      });
    }
  }
  return spans;
}
