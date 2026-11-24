import { describe, expect, it } from "vitest";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { ROUTE_SKELETON_MANIFEST } from "@/common/lib/route-skeleton-manifest";

const SRC_DIR = join(import.meta.dirname, "../..");
const GENERATED_TREE = join(SRC_DIR, "routeTree.gen.ts");

function generatedFullPaths(): string[] {
  const source = readFileSync(GENERATED_TREE, "utf8");
  const body = source.match(
    /export interface FileRoutesByFullPath \{([\s\S]*?)\n\}/,
  )?.[1];
  if (!body) throw new Error("FileRoutesByFullPath is missing");
  return [...body.matchAll(/^\s+["']([^"']+)["']:/gm)].map((match) => match[1]);
}

function ownerSource(owner: string): string {
  return readFileSync(join(SRC_DIR, owner), "utf8");
}

describe("route skeleton manifest (w5/m79)", () => {
  it("classifies every generated full path exactly once", () => {
    const generated = generatedFullPaths().sort();
    const classified = Object.keys(ROUTE_SKELETON_MANIFEST).sort();

    expect(classified).toEqual(generated);
    // 110: + /build-sources and /build-sources/new (w2/m613). 108 before them:
    // the create pages at Render's URLs (/web/new, /pserv/new, /worker/new,
    // /cron/new, /blueprint/new; /static/new and /new/redis turned from
    // redirects into pages). 103 before them: + /sandbox-groups and
    // /sandbox-groups/$sandboxGroupId (w2/m533); before them 101: + a
    // project's Service Metrics view (w3/m508); before it 100: + a project's
    // Application Logs and Request Logs views (w3/m505); before them 98: + a
    // service's Compute page (w6/m483)
    expect(classified).toHaveLength(110);
  });

  it("points every classification at a real route owner", () => {
    const missing = Object.entries(ROUTE_SKELETON_MANIFEST)
      .filter(
        ([, disposition]) => !existsSync(join(SRC_DIR, disposition.owner)),
      )
      .map(([path]) => path);

    expect(missing).toEqual([]);
  });

  it("requires every rendering route to own tailored pending geometry", () => {
    const offenders = Object.entries(ROUTE_SKELETON_MANIFEST).flatMap(
      ([path, disposition]) => {
        if (disposition.kind !== "render") return [];
        const source = ownerSource(disposition.owner);
        const pending = source.match(
          /pendingComponent:\s*([A-Za-z_$][\w$]*)/,
        )?.[1];

        // The workspace alias has no blocking loader: its component remains a
        // destination-shaped skeleton until it completes the redirect.
        if (path === "/w/$") {
          return source.includes("<WorkspaceAliasSkeleton") ? [] : [path];
        }
        if (!pending) return [path];
        return [
          "RoutePending",
          "ListPageSkeleton",
          "FormPageSkeleton",
        ].includes(pending)
          ? [path]
          : [];
      },
    );

    expect(offenders).toEqual([]);
  });

  it("keeps redirects, server handlers, and not-found routes skeleton-free", () => {
    const offenders = Object.entries(ROUTE_SKELETON_MANIFEST).flatMap(
      ([path, disposition]) => {
        if (disposition.kind === "render") return [];
        return /pendingComponent:\s*[A-Za-z_$]/.test(
          ownerSource(disposition.owner),
        )
          ? [path]
          : [];
      },
    );

    expect(offenders).toEqual([]);
  });

  it("gives every rendering shape a non-empty, duplicate-free region contract", () => {
    const offenders = Object.entries(ROUTE_SKELETON_MANIFEST).flatMap(
      ([path, disposition]) => {
        if (disposition.kind !== "render") return [];
        const regions: readonly string[] = disposition.regions;
        return regions.length === 0 || new Set(regions).size !== regions.length
          ? [path]
          : [];
      },
    );

    expect(offenders).toEqual([]);
  });
});
