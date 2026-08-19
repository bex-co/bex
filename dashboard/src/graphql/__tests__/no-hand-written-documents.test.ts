import { describe, it, expect } from "vitest";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

/**
 * The no-shadow-documents invariant (w1/m79).
 *
 * Every GraphQL operation document the dashboard sends comes from generated
 * code: an operation lives in a feature's `*.graphql` file, `yarn codegen`
 * turns it into a TypedDocumentNode in `src/graphql/definitions.ts`, and hooks
 * import it from `@/graphql/definitions`. For a while five features instead
 * kept hand-written parallel documents (`gql` templates cast
 * `as unknown as TypedDocumentNode<...>`) with hand-maintained result types —
 * ~1,200 lines whose types silently lied about `__typename` and nullability.
 * w1/m79 deleted that shadow layer; this guard keeps it from growing back.
 *
 * Allowed: `import type { TypedDocumentNode }` as a GENERIC CONSTRAINT over
 * documents that come from definitions (see `use-field-mutation.ts`). Not
 * allowed under `src/features/` or `src/routes/`: importing `graphql-tag`,
 * authoring a `gql` template, or casting a value to a TypedDocumentNode.
 *
 * If this test fails on your new operation: add it to the feature's
 * `.graphql` file and regenerate (dashboard/CLAUDE.md § Offline codegen),
 * then import the document from `@/graphql/definitions`.
 */
const SRC_DIR = join(import.meta.dirname, "..", "..");
const SCANNED_DIRS = ["features", "routes"];

// One walk + one read, shared by every assertion below.
const sources: Array<[file: string, content: string]> = SCANNED_DIRS.flatMap(
  (dir) =>
    readdirSync(join(SRC_DIR, dir), { recursive: true })
      .map(String)
      .filter((f) => /\.(ts|tsx)$/.test(f))
      .map((f) => join(dir, f))
      .sort(),
).map((file) => [file, readFileSync(join(SRC_DIR, file), "utf8")]);

function offenders(pattern: RegExp): string[] {
  return sources
    .filter(([, content]) => pattern.test(content))
    .map(([file]) => file);
}

describe("no hand-written GraphQL documents", () => {
  it("no feature or route imports graphql-tag", () => {
    expect(offenders(/from\s+["']graphql-tag["']/)).toEqual([]);
  });

  it("no feature or route authors a gql template", () => {
    expect(offenders(/\bgql\s*`/)).toEqual([]);
  });

  it("no feature or route casts a value to a TypedDocumentNode", () => {
    // The shadow-layer signature: `... as unknown as TypedDocumentNode<...>`.
    // (`import type { TypedDocumentNode }` as a generic constraint is fine.)
    expect(offenders(/\bas\s+(?:unknown\s+as\s+)?TypedDocumentNode\b/)).toEqual(
      [],
    );
  });

  it("guards a non-empty set of source files", () => {
    // Keeps the assertions above from passing vacuously if the walk breaks.
    expect(sources.length).toBeGreaterThan(200);
  });
});
