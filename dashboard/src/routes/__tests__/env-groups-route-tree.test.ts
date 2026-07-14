import { describe, expect, it } from "vitest";
import { createMemoryHistory, createRouter } from "@tanstack/react-router";
import { routeTree } from "@/routeTree.gen";

interface RuntimeRoute {
  fullPath: string;
  parentRoute?: RuntimeRoute;
  children?: Record<string, RuntimeRoute>;
}

describe("environment-group file routes", () => {
  it("mounts list and detail as sibling pages", () => {
    createRouter({
      routeTree,
      history: createMemoryHistory({ initialEntries: ["/env-groups"] }),
      context: { client: {} as never, session: null },
    });
    const root = routeTree as unknown as RuntimeRoute;
    const topLevel = Object.values(root.children ?? {});
    const list = topLevel.find((route) => route.fullPath === "/env-groups");
    const detail = topLevel.find(
      (route) => route.fullPath === "/env-groups/$groupId",
    );

    expect(list).toBeDefined();
    expect(detail).toBeDefined();
    expect(detail?.parentRoute).toBe(root);
    expect(detail?.parentRoute).not.toBe(list);
  });
});
