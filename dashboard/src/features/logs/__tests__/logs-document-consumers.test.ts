import { describe, expect, it } from "vitest";

// Every dashboard module that reads the generic `logs(...)` query (the
// `LogsDocument` operation) must page past the 100-row server cap through
// useOlderLogPages. w4/m107 fixed paging for the service Logs tab only, and the
// Key Value, Postgres, and deploy log viewers kept silently stopping at the
// newest 100 lines until w4/m136. A new log surface that queries LogsDocument
// without paging fails here instead of in production.
const sources = import.meta.glob<string>(
  ["/src/**/*.{ts,tsx}", "!/src/**/__tests__/**", "!/src/graphql/**"],
  { query: "?raw", import: "default", eager: true },
);

const USES_LOGS_DOCUMENT = /\bLogsDocument\b/;
const PAGES = /\buseOlderLogPages\b/;
// The pager itself issues the older-page queries.
const PAGER = "/src/features/logs/hooks/use-older-log-pages.ts";

describe("LogsDocument consumers", () => {
  const consumers = Object.entries(sources)
    .filter(([path, src]) => path !== PAGER && USES_LOGS_DOCUMENT.test(src))
    .map(([path]) => path)
    .sort();

  it("finds the known consumers (guards the glob itself)", () => {
    expect(consumers).toEqual([
      "/src/features/deploys/hooks/use-deploy-logs.ts",
      "/src/features/logs/hooks/use-log-history.ts",
    ]);
  });

  it.each(consumers)("%s pages through useOlderLogPages", (path) => {
    expect(sources[path]).toMatch(PAGES);
  });
});
