import { readFileSync } from "node:fs";
import { resolve } from "node:path";

// The stylesheet's theme tokens and the colour maths that reads them, for the
// tests that judge colour (jsdom has none): theme-contrast.test.ts holds the
// pairs to WCAG, and the theme-color tests (w3/098) pin each theme's page
// colour to `PAGE_COLOR`.

const ROOT = resolve(import.meta.dirname, "../..");
/** A file of the dashboard, by its path from `dashboard/`. */
export const read = (path: string) => readFileSync(resolve(ROOT, path), "utf8");

// ---- colour maths ----------------------------------------------------------

export type Rgb = [number, number, number]; // sRGB, 0..1, gamma-encoded
export interface Colour {
  rgb: Rgb;
  alpha: number;
}

const clamp = (x: number) => Math.min(1, Math.max(0, x));
const encode = (x: number) =>
  x <= 0.0031308 ? 12.92 * x : 1.055 * x ** (1 / 2.4) - 0.055;

/** CSS `oklch(L C H)` to gamma-encoded sRGB, out-of-gamut values clipped. */
export function oklch(lightness: number, chroma: number, hue: number): Rgb {
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

// `oklch(0.52 0.16 138.3)`, `oklch(76.9% 0.188 70.08)`, `oklch(0.5 0.1 20 / 15%)`
export function parseOklch(value: string): Colour {
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

/** `#rrggbb`, each channel rounded to the nearest of 256. */
const hex = (rgb: Rgb) =>
  `#${rgb
    .map((c) =>
      Math.round(c * 255)
        .toString(16)
        .padStart(2, "0"),
    )
    .join("")}`;

// ---- the stylesheet's tokens ------------------------------------------------

export type Tokens = Record<string, string>;

function declarations(block: string): Tokens {
  const tokens: Tokens = {};
  for (const match of block.matchAll(
    /--([a-z0-9-]+):\s*(oklch\([^)]*\))\s*;/g,
  )) {
    tokens[match[1]] = match[2];
  }
  return tokens;
}

export function blockAfter(css: string, selector: RegExp): string {
  const start = css.search(selector);
  if (start < 0) throw new Error(`style.css has no ${selector}`);
  const open = css.indexOf("{", start);
  const close = css.indexOf("\n}", open);
  return css.slice(open + 1, close);
}

export const css = read("src/style.css");
export const LIGHT: Tokens = declarations(
  blockAfter(css, /\n:root,\n\.light \{/),
);
export const DARK: Tokens = {
  ...LIGHT,
  ...declarations(blockAfter(css, /\n\.dark \{/)),
};
// High Contrast Mode's twins: each theme with its high-contrast overrides.
export const LIGHT_HC_OVERRIDES = declarations(
  blockAfter(css, /\n\.light\.high-contrast \{/),
);
export const DARK_HC_OVERRIDES = declarations(
  blockAfter(css, /\n\.dark\.high-contrast \{/),
);
export const LIGHT_HC: Tokens = { ...LIGHT, ...LIGHT_HC_OVERRIDES };
export const DARK_HC: Tokens = { ...DARK, ...DARK_HC_OVERRIDES };

/** The page colour, `--background` as `#rrggbb`, of each Dashboard Theme
 *  setting that names its theme: the classes it puts on <html>. */
export const PAGE_COLOR = {
  LIGHT: hex(parseOklch(LIGHT.background).rgb),
  DARK: hex(parseOklch(DARK.background).rgb),
  LIGHT_HIGH_CONTRAST: hex(parseOklch(LIGHT_HC.background).rgb),
  DARK_HIGH_CONTRAST: hex(parseOklch(DARK_HC.background).rgb),
};
