import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

// The colour pairs the dashboard actually uses, in both themes, against WCAG
// 2.1 AA: 4.5:1 for text, 3:1 for the part of a control that identifies it (the
// focus ring). jsdom has no colour, so the page sweep cannot judge it: this
// reads the stylesheet's tokens and Tailwind's palette and computes the ratios.

const ROOT = resolve(import.meta.dirname, "../../..");
const read = (path: string) => readFileSync(resolve(ROOT, path), "utf8");

// ---- colour maths ----------------------------------------------------------

type Rgb = [number, number, number]; // sRGB, 0..1, gamma-encoded
interface Colour {
  rgb: Rgb;
  alpha: number;
}

const clamp = (x: number) => Math.min(1, Math.max(0, x));
const encode = (x: number) =>
  x <= 0.0031308 ? 12.92 * x : 1.055 * x ** (1 / 2.4) - 0.055;
const decode = (x: number) =>
  x <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4;

/** CSS `oklch(L C H)` to gamma-encoded sRGB, out-of-gamut values clipped. */
function oklch(lightness: number, chroma: number, hue: number): Rgb {
  const a = chroma * Math.cos((hue * Math.PI) / 180);
  const b = chroma * Math.sin((hue * Math.PI) / 180);
  const l = (lightness + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const m = (lightness - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const s = (lightness - 0.0894841775 * a - 1.291485548 * b) ** 3;
  return [
    encode(clamp(4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s)),
    encode(clamp(-1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s)),
    encode(clamp(-0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s)),
  ];
}

function luminance([r, g, b]: Rgb): number {
  return 0.2126 * decode(r) + 0.7152 * decode(g) + 0.0722 * decode(b);
}

function contrast(a: Rgb, b: Rgb): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

/** `top` over `bottom`, blended in sRGB as CSS does. */
function over(top: Colour, bottom: Rgb): Rgb {
  return top.rgb.map(
    (c, i) => c * top.alpha + bottom[i] * (1 - top.alpha),
  ) as Rgb;
}

// `oklch(0.52 0.16 138.3)`, `oklch(76.9% 0.188 70.08)`, `oklch(0.5 0.1 20 / 15%)`
function parseOklch(value: string): Colour {
  const match = value
    .trim()
    .match(
      /^oklch\(\s*([\d.]+)(%?)\s+([\d.]+)\s+([\d.]+)(?:\s*\/\s*([\d.]+)(%?))?\s*\)$/,
    );
  if (!match) throw new Error(`not an oklch() colour: ${value}`);
  const lightness = Number(match[1]) / (match[2] ? 100 : 1);
  const alpha =
    match[5] === undefined ? 1 : Number(match[5]) / (match[6] ? 100 : 1);
  return { rgb: oklch(lightness, Number(match[3]), Number(match[4])), alpha };
}

// ---- the stylesheet's tokens and Tailwind's palette -------------------------

type Tokens = Record<string, string>;

function declarations(block: string): Tokens {
  const tokens: Tokens = {};
  for (const match of block.matchAll(
    /--([a-z0-9-]+):\s*(oklch\([^)]*\))\s*;/g,
  )) {
    tokens[match[1]] = match[2];
  }
  return tokens;
}

function blockAfter(css: string, selector: RegExp): string {
  const start = css.search(selector);
  if (start < 0) throw new Error(`style.css has no ${selector}`);
  const open = css.indexOf("{", start);
  const close = css.indexOf("\n}", open);
  return css.slice(open + 1, close);
}

const css = read("src/style.css");
const LIGHT: Tokens = declarations(blockAfter(css, /\n:root \{/));
const DARK: Tokens = {
  ...LIGHT,
  ...declarations(blockAfter(css, /\n\.dark \{/)),
};

const palette: Tokens = {};
for (const match of read("node_modules/tailwindcss/theme.css").matchAll(
  /--color-([a-z]+-\d+):\s*(oklch\([^)]*\))\s*;/g,
)) {
  palette[match[1]] = match[2];
}

// ---- the pairs --------------------------------------------------------------

type Theme = "light" | "dark";
const TOKENS: Record<Theme, Tokens> = { light: LIGHT, dark: DARK };

interface Pair {
  name: string;
  /** Text needs 4.5:1; the part of a control that identifies it, 3:1. */
  min: 4.5 | 3;
  /** Resolve both colours in a theme: foreground and the page it sits on. */
  colours: (tokens: Tokens) => { fg: Rgb; bg: Rgb };
}

const token = (tokens: Tokens, name: string): Colour => {
  const value = tokens[name];
  if (!value) throw new Error(`the stylesheet no longer defines --${name}`);
  return parseOklch(value);
};
const colour = (name: string): Colour => {
  const value = palette[name];
  if (!value) throw new Error(`Tailwind's palette has no ${name}`);
  return parseOklch(value);
};
const tokenPair = (fg: string, bg: string, min: Pair["min"] = 4.5): Pair => ({
  name: `--${fg} on --${bg}`,
  min,
  colours: (tokens) => ({
    fg: token(tokens, fg).rgb,
    bg: token(tokens, bg).rgb,
  }),
});

const TOKEN_PAIRS: Pair[] = [
  tokenPair("foreground", "background"),
  tokenPair("foreground", "card"),
  tokenPair("card-foreground", "card"),
  tokenPair("popover-foreground", "popover"),
  tokenPair("secondary-foreground", "secondary"),
  tokenPair("muted-foreground", "background"),
  tokenPair("muted-foreground", "card"),
  tokenPair("muted-foreground", "muted"),
  tokenPair("accent-foreground", "accent"),
  tokenPair("primary-foreground", "primary"),
  tokenPair("destructive", "background"),
  tokenPair("destructive", "card"),
  tokenPair("sidebar-foreground", "sidebar"),
  tokenPair("sidebar-primary-foreground", "sidebar-primary"),
  tokenPair("sidebar-accent-foreground", "sidebar-accent"),
  tokenPair("primary", "background"),
  tokenPair("primary", "card"),
  // The focus ring identifies the focused control (WCAG 1.4.11).
  tokenPair("ring", "background", 3),
  tokenPair("ring", "card", 3),
];

// A status colour is a Tailwind palette entry, with `dark:` naming the entry
// the dark theme uses. `files` are the sources that carry the classes: the test
// fails when one stops containing them, so a change of colour there comes back
// to this table instead of slipping past it.
interface Status {
  name: string;
  file: { path: string; contains: string[] };
  light: { fg: string; tint?: [string, number] };
  dark: { fg: string; tint?: [string, number] };
  /** What the text sits on: the card, or the log viewport's tinted page. */
  on: "card" | "logViewport";
}

const STATUS: Status[] = [
  {
    name: "log 2xx chip",
    file: {
      path: "src/features/logs/components/log-line-list.tsx",
      contains: ["bg-green-500/15 text-green-800 dark:text-green-400"],
    },
    light: { fg: "green-800", tint: ["green-500", 0.15] },
    dark: { fg: "green-400", tint: ["green-500", 0.15] },
    on: "logViewport",
  },
  {
    name: "log 3xx chip",
    file: {
      path: "src/features/logs/components/log-line-list.tsx",
      contains: ["bg-blue-500/15 text-blue-700 dark:text-blue-400"],
    },
    light: { fg: "blue-700", tint: ["blue-500", 0.15] },
    dark: { fg: "blue-400", tint: ["blue-500", 0.15] },
    on: "logViewport",
  },
  {
    name: "log 4xx chip",
    file: {
      path: "src/features/logs/components/log-line-list.tsx",
      contains: ["bg-amber-500/15 text-amber-800 dark:text-amber-400"],
    },
    light: { fg: "amber-800", tint: ["amber-500", 0.15] },
    dark: { fg: "amber-400", tint: ["amber-500", 0.15] },
    on: "logViewport",
  },
  {
    name: "log 5xx chip",
    file: {
      path: "src/features/logs/components/log-line-list.tsx",
      contains: ["bg-red-500/15 text-red-700 dark:text-red-400"],
    },
    light: { fg: "red-700", tint: ["red-500", 0.15] },
    dark: { fg: "red-400", tint: ["red-500", 0.15] },
    on: "logViewport",
  },
  {
    name: "project card health line",
    file: {
      path: "src/features/projects/components/project-card.tsx",
      contains: ["text-emerald-700 dark:text-emerald-400"],
    },
    light: { fg: "emerald-700" },
    dark: { fg: "emerald-400" },
    on: "card",
  },
  {
    name: "billing Partial data label",
    file: {
      path: "src/features/usage/components/charges-card.tsx",
      contains: ["text-amber-700", "dark:text-amber-500"],
    },
    light: { fg: "amber-700" },
    dark: { fg: "amber-500" },
    on: "card",
  },
  {
    name: "notifications 'bex extension' chip (text-primary on bg-primary/10)",
    file: {
      path: "src/features/notifications/components/push-notification-settings-panel.tsx",
      contains: ["bg-primary/10", "text-primary"],
    },
    light: { fg: "--primary", tint: ["--primary", 0.1] },
    dark: { fg: "--primary", tint: ["--primary", 0.1] },
    on: "card",
  },
  {
    name: "success badge",
    file: {
      path: "src/common/components/ui/badge.tsx",
      contains: ["bg-emerald-600/15 text-emerald-800 dark:text-emerald-400"],
    },
    light: { fg: "emerald-800", tint: ["emerald-600", 0.15] },
    dark: { fg: "emerald-400", tint: ["emerald-600", 0.15] },
    on: "card",
  },
];

function statusColours(status: Status, theme: Theme, tokens: Tokens) {
  // A colour is a Tailwind palette entry (`amber-800`) or a stylesheet token
  // (`--primary`).
  const lookup = (name: string): Colour =>
    name.startsWith("--") ? token(tokens, name.slice(2)) : colour(name);
  const page =
    status.on === "card" ? token(tokens, "card").rgb : logViewport(tokens);
  const spec = status[theme];
  const bg = spec.tint
    ? over({ ...lookup(spec.tint[0]), alpha: spec.tint[1] }, page)
    : page;
  return { fg: lookup(spec.fg).rgb, bg };
}

// The log viewport is `bg-muted/30` over the page.
function logViewport(tokens: Tokens): Rgb {
  return over(
    { ...token(tokens, "muted"), alpha: 0.3 },
    token(tokens, "background").rgb,
  );
}

// The log line's own dim text: the timestamp and instance id.
const LOG_META: Pair = {
  name: "log line timestamp and instance id (--muted-foreground on the log viewport)",
  min: 4.5,
  colours: (tokens) => ({
    fg: token(tokens, "muted-foreground").rgb,
    bg: logViewport(tokens),
  }),
};

// ---- the check --------------------------------------------------------------

interface Failure {
  theme: Theme;
  pair: string;
  ratio: number;
  min: number;
}

function failures(tokensByTheme: Record<Theme, Tokens>): Failure[] {
  const found: Failure[] = [];
  for (const theme of ["light", "dark"] as const) {
    const tokens = tokensByTheme[theme];
    const check = (pair: string, min: number, fg: Rgb, bg: Rgb) => {
      const ratio = contrast(fg, bg);
      if (ratio < min)
        found.push({ theme, pair, ratio: Math.round(ratio * 100) / 100, min });
    };
    for (const pair of [...TOKEN_PAIRS, LOG_META]) {
      const { fg, bg } = pair.colours(tokens);
      check(pair.name, pair.min, fg, bg);
    }
    for (const status of STATUS) {
      const { fg, bg } = statusColours(status, theme, tokens);
      check(status.name, 4.5, fg, bg);
    }
  }
  return found;
}

const describeFailures = (list: Failure[]) =>
  list
    .map((f) => `${f.theme}: ${f.pair} is ${f.ratio}:1, needs ${f.min}:1`)
    .join("\n");

// ---- tests ------------------------------------------------------------------

describe("the contrast calculation", () => {
  it("matches known values", () => {
    const white = oklch(1, 0, 0);
    const black = oklch(0, 0, 0);
    expect(contrast(black, white)).toBeCloseTo(21, 1);
    expect(contrast(white, white)).toBeCloseTo(1, 5);
    // sRGB red, #ff0000: 3.998:1 on white (the WCAG figure).
    expect(contrast(oklch(0.628, 0.2577, 29.23), white)).toBeCloseTo(4.0, 1);
    // #767676, the darkest grey that passes 4.5:1 on white (4.54:1).
    expect(contrast(oklch(0.5544, 0, 0), white)).toBeGreaterThan(4.4);
    expect(contrast(oklch(0.5544, 0, 0), white)).toBeLessThan(4.9);
  });

  it("blends a translucent colour over its page in sRGB", () => {
    const blended = over({ rgb: oklch(0, 0, 0), alpha: 0.5 }, oklch(1, 0, 0));
    expect(blended[0]).toBeCloseTo(0.5, 2);
  });
});

describe("the stylesheet's colour pairs", () => {
  it("reads both themes' tokens", () => {
    expect(Object.keys(LIGHT).length).toBeGreaterThan(20);
    expect(DARK.background).not.toBe(LIGHT.background);
    expect(DARK.border).toBeDefined();
  });

  it("meets WCAG AA in the light and the dark theme", () => {
    const found = failures(TOKENS);
    expect(found, describeFailures(found)).toEqual([]);
  });

  it("fails when a token's lightness drops below the threshold, in the theme it drops in", () => {
    const lightened = failures({
      light: { ...LIGHT, "muted-foreground": "oklch(0.7 0.02 260)" },
      dark: DARK,
    });
    expect(lightened.length).toBeGreaterThan(0);
    expect(lightened.every((f) => f.theme === "light")).toBe(true);
    expect(lightened.map((f) => f.pair).join("\n")).toMatch(/muted-foreground/);

    const dimmed = failures({
      light: LIGHT,
      dark: { ...DARK, "muted-foreground": "oklch(0.3 0.02 260)" },
    });
    expect(dimmed.length).toBeGreaterThan(0);
    expect(dimmed.every((f) => f.theme === "dark")).toBe(true);
  });

  it("fails a pair that names a token the stylesheet no longer defines", () => {
    const { "muted-foreground": _removed, ...without } = LIGHT;
    void _removed;
    expect(() => failures({ light: without, dark: DARK })).toThrow(
      /--muted-foreground/,
    );
  });
});

describe("the status colours the components use", () => {
  it.each(STATUS.map((s) => [s.name, s.file] as const))(
    "%s is still spelled the way this table has it",
    (_name, file) => {
      const source = read(file.path);
      for (const classes of file.contains) {
        expect(
          source,
          `${file.path} no longer contains "${classes}": re-check its contrast and update the table`,
        ).toContain(classes);
      }
    },
  );
});
