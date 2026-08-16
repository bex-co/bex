import { describe, it, expect } from "vitest";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

/**
 * Route files should not export their page components (w5/m67).
 *
 * TanStack's code-splitter (the `tanstackRouter` plugin, loudest in `yarn
 * vitest run`) warns that a route module's non-`Route` export "will not be
 * code-split", because the named export must stay in the route's critical
 * chunk. The fix is to keep the component route-file-local (unexported) or
 * move it into a feature module (`src/features/<area>/pages|components/`) and
 * import it — so the route file exports only `Route`.
 *
 * The production build already lazy-splits every route regardless (the export
 * costs a vitest-time warning, not a bundle byte — see w5/m67's corrected
 * premise), so this is a hygiene guard, not a perf gate: it stops the count of
 * exported route components from *growing*. The `ALLOWLIST` below is the set
 * that existed when the guard was added; every entry is benign and relocating
 * them into feature modules is deferred (`.pm/w5/043.md`). A NEW route module
 * that exports a component fails this test — define it locally (no `export`)
 * or put it in a feature module instead. Removing/relocating an allowlisted
 * export is fine (it just stops matching); do not add to the allowlist.
 */
const ROUTES_DIR = join(import.meta.dirname, "..");

// `${routeFile} ${ExportedComponentName}` for every route-file component export
// that predates this guard. Do not extend it — new entries mean a regression.
const ALLOWLIST = new Set<string>([
  "blueprints.$blueprintId.tsx BlueprintDetailPage",
  "blueprints.new.tsx NewBlueprintPage",
  "blueprints.tsx BlueprintsPage",
  "env-groups_.$groupId.tsx EnvGroupDetailPage",
  "env-groups.tsx EnvGroupsPage",
  "index.tsx HomePage",
  "keyvalue.$keyValueId.tsx KeyValueDetailPage",
  "keyvalue.new.tsx NewKeyValuePage",
  "new.workspace.tsx NewWorkspacePage",
  "project.$projectId.index.tsx ProjectPage",
  "project.$projectId.settings.tsx ProjectSettingsPage",
  "services.$serviceId.env.tsx ServiceEnvPage",
  "services.$serviceId.events.tsx ServiceEventsPage",
  "services.$serviceId.headers.tsx ServiceHeadersPage",
  "services.$serviceId.logs.tsx ServiceLogsPage",
  "services.$serviceId.metrics.tsx ServiceMetricsPage",
  "services.$serviceId.redirects.tsx ServiceRedirectsPage",
  "services.$serviceId.scaling.tsx ServiceScalingPage",
  "services.$serviceId.settings.tsx ServiceSettingsPage",
  "services.new.tsx NewServicePage",
  "webhooks_.new.tsx NewWebhookPage",
]);

// `export function Foo()` / `export const Foo =` where `Foo` is a PascalCase
// component-shaped name — the shape the code-splitter warns about. `Route`
// (the one legitimate route export) is excluded by construction.
const COMPONENT_EXPORT =
  /^export (?:function|const) ([A-Z][A-Za-z0-9]*(?:Page|Detail|Pane|View|Component))\b/gm;

function routeFiles(): string[] {
  return readdirSync(ROUTES_DIR)
    .filter((f) => f.endsWith(".tsx"))
    .sort();
}

describe("no new route-file component exports", () => {
  it("no route module exports a component outside the allowlist", () => {
    const offenders: string[] = [];
    for (const file of routeFiles()) {
      const src = readFileSync(join(ROUTES_DIR, file), "utf8");
      for (const m of src.matchAll(COMPONENT_EXPORT)) {
        const key = `${file} ${m[1]}`;
        if (!ALLOWLIST.has(key)) offenders.push(key);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("guards a non-empty set of route files", () => {
    // Keep the assertion above from passing vacuously if the glob breaks.
    expect(routeFiles().length).toBeGreaterThan(20);
  });
});
