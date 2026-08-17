import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

/**
 * The Tiptap lazy-boundary guard.
 *
 * `inline-mention-editor` pulls the whole @tiptap stack (~100 KB gzip). It must
 * stay behind the dynamic import in `lazy-mention-editor`, or the /agents
 * route's combined create + history page pays for it before the composer
 * mounts in the browser — the same reason `session-conversation` wraps its
 * impl. This guard asserts the eager import never creeps back into the
 * composer's static graph.
 */
const COMPONENTS_DIR = join(import.meta.dirname, "..");

function source(file: string): string {
  return readFileSync(join(COMPONENTS_DIR, file), "utf8");
}

describe("lazy mention-editor boundary", () => {
  it("new-session-composer does not statically import the Tiptap editor", () => {
    const src = source("new-session-composer.tsx");
    expect(src).not.toMatch(/^import\s[^;]*inline-mention-editor["']/m);
    expect(src).not.toMatch(/^import\s[^;]*@tiptap\//m);
    expect(src).toMatch(/^import\s[^;]*lazy-mention-editor["']/m);
  });

  it("lazy-mention-editor loads the impl only through a dynamic import", () => {
    const src = source("lazy-mention-editor.tsx");
    expect(src).toMatch(/import\(\s*["'][^"']*inline-mention-editor["']\s*\)/);
    // Type-only references are erased at compile time; a runtime static import
    // would defeat the split.
    expect(src).not.toMatch(
      /^import\s+(?!type\b)[^;]*inline-mention-editor["']/m,
    );
  });
});
