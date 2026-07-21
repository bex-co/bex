// The drift gate for the email brand codegen (w1/m54): the committed
// lego/backend/internal/email/brand_gen.go must equal what the generator
// produces from the current src/style.css. Runs under vitest (dashboard CI),
// which is exactly when style.css can change.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { generate, oklchToHex, parseTokens } from "../generate-email-brand.mjs";

// Paths from the vitest root (dashboard/) — the script's own URL constants
// resolve against import.meta.url, which vitest's transform rewrites off the
// file: scheme, so they are CLI-only.
const STYLE_CSS = join(process.cwd(), "src/style.css");
const BRAND_GO = join(
  process.cwd(),
  "../lego/backend/internal/email/brand_gen.go",
);

describe("generate-email-brand", () => {
  it("brand_gen.go is in sync with src/style.css (run `yarn generate:email-brand` on drift)", () => {
    const want = generate(readFileSync(STYLE_CSS, "utf8"));
    const got = readFileSync(BRAND_GO, "utf8");
    expect(got).toBe(want);
  });

  it("converts oklch achromatic extremes exactly", () => {
    expect(oklchToHex(1, 0, 0)).toBe("#ffffff");
    expect(oklchToHex(0, 0, 0)).toBe("#000000");
  });

  it("converts the brand primary to a green (g channel dominant)", () => {
    const hex = oklchToHex(0.52, 0.16, 138.3);
    const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16));
    expect(g).toBeGreaterThan(r);
    expect(g).toBeGreaterThan(b);
  });

  it("extracts every token the layout needs from style.css", () => {
    const tokens = parseTokens(readFileSync(STYLE_CSS, "utf8"));
    expect(tokens.colors.map((c) => c.goName)).toContain("BrandPrimary");
    for (const c of tokens.colors) expect(c.hex).toMatch(/^#[0-9a-f]{6}$/);
    expect(tokens.radiusPx).toBeGreaterThan(0);
    // The stack must stay email-safe: generic/system families, no webfont.
    expect(tokens.fontFamily).toMatch(/sans-serif$/);
  });
});
