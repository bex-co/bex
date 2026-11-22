import { describe, expect, it } from "vitest";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";

/**
 * Every chrome page's scroll container, width and header come from
 * `common/components/page-frame.tsx`; a page that hand-rolls them drifts.
 */
const SRC = join(import.meta.dirname, "../..");

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory())
      return name === "__tests__" ? [] : sources(path);
    return path.endsWith(".tsx") ? [path] : [];
  });
}

const FILES = [
  ...sources(join(SRC, "routes")),
  ...sources(join(SRC, "features")),
  ...sources(join(SRC, "common")),
].filter((path) => !path.endsWith("page-frame.tsx"));

describe("page frame invariant", () => {
  it("leaves the page scroll container to PageFrame / PAGE_SCROLL_CLASS", () => {
    const offenders = FILES.filter((path) =>
      readFileSync(path, "utf8").includes("flex-1 overflow-auto p-4 sm:p-6"),
    ).map((path) => relative(SRC, path));
    expect(offenders).toEqual([]);
  });

  it("titles list, settings and create pages with PageHeader", () => {
    const framed = [
      "routes/index.tsx",
      "routes/webhooks.tsx",
      "routes/notifications.tsx",
      "routes/env-groups.tsx",
      "routes/blueprints.tsx",
      "routes/workspace.settings.tsx",
      "routes/new.workspace.tsx",
      // The service create page's heading, shared with its skeleton.
      "features/services/components/service-create-header.tsx",
      "features/blueprints/pages/new-blueprint-page.tsx",
      "features/keyvalue/pages/new-key-value-page.tsx",
      "routes/webhooks_.new.tsx",
      "routes/project.$projectId.settings.tsx",
      "features/usage/components/usage-page.tsx",
      "features/auth/pages/settings-page/index.tsx",
      "features/agent-automations/pages/automations-page.tsx",
      "features/router/page.tsx",
    ];
    const offenders = framed.filter((path) => {
      const source = readFileSync(join(SRC, path), "utf8");
      return (
        /^\s*<h1\b/m.test(source) ||
        /CardTitle as="h1"/.test(source) ||
        !/PageHeader/.test(source)
      );
    });
    expect(offenders).toEqual([]);
  });
});
