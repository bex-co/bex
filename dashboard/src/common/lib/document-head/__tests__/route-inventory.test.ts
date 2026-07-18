import { readdirSync, readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  CLASSIFIED_ROUTE_FILES,
  ROUTE_HEAD_INVENTORY,
} from "@/common/lib/document-head/route-inventory";

const routesDirectory = `${process.cwd()}/src/routes`;

function routeFiles() {
  return readdirSync(routesDirectory)
    .filter((file) => file.endsWith(".tsx"))
    .sort();
}

describe("route head inventory", () => {
  it("classifies every route exactly once with an actionable failure", () => {
    const actual = routeFiles();
    const classified = [...CLASSIFIED_ROUTE_FILES].sort();
    const duplicates = classified.filter(
      (file, index) => classified.indexOf(file) !== index,
    );

    expect(
      duplicates,
      "A route appears in more than one head category",
    ).toEqual([]);
    expect(
      classified,
      "A dashboard route was added or removed. Classify it in route-inventory.ts as content, inherited-layout, redirect-only, non-html, or fallback.",
    ).toEqual(actual);
  });

  it("keeps competing title metadata off inherited, redirect, and API routes", () => {
    const routesWithoutOwnHead = [
      ...ROUTE_HEAD_INVENTORY["inherited-layout"],
      ...ROUTE_HEAD_INVENTORY["redirect-only"],
      ...ROUTE_HEAD_INVENTORY["non-html"],
    ];

    for (const file of routesWithoutOwnHead) {
      const source = readFileSync(`${routesDirectory}/${file}`, "utf8");
      expect(
        source,
        `${file} must inherit, redirect, or return an API response`,
      ).not.toMatch(/\bhead\s*:/);
    }
  });

  it("requires every content route to own an explicit head", () => {
    for (const file of ROUTE_HEAD_INVENTORY.content) {
      const source = readFileSync(`${routesDirectory}/${file}`, "utf8");
      expect(
        source,
        `${file} is classified as content but has no head`,
      ).toMatch(/\bhead\s*:/);
    }
  });

  it("keeps route files off client-only and hand-built title paths", () => {
    for (const file of routeFiles()) {
      const source = readFileSync(`${routesDirectory}/${file}`, "utf8");
      expect(source, `${file} must use route head metadata`).not.toMatch(
        /document\.title/,
      );
      expect(source, `${file} must use the shared title formatter`).not.toMatch(
        /title\s*:\s*[`"'][^\n]*[·・—][^\n]*bex/i,
      );
    }
  });
});
