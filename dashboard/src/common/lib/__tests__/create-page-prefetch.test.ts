import { describe, expect, it, vi } from "vitest";
import {
  DatabaseInstanceTypesDocument,
  GitConnectionsDocument,
  ProjectsDocument,
} from "@/graphql/definitions";
import type { RouterContext } from "@/common/types/router-context";
import { prefetchCreatePage } from "../create-page-prefetch";

function contextWith(workspaceId: string | null) {
  const query = vi.fn().mockResolvedValue({});
  const context = {
    client: { query },
    workspaceId,
  } as unknown as RouterContext;
  return { context, query };
}

describe("prefetchCreatePage", () => {
  // A New-menu hover preloads the route, so the page opens on a warm cache:
  // the plan catalog, the Project row's projects and the source picker's Git
  // connection.
  it("warms the catalog, the workspace's projects and its Git connection", async () => {
    const { context, query } = contextWith("tea-1");
    await prefetchCreatePage(
      { context, cause: "preload" },
      { catalog: DatabaseInstanceTypesDocument, git: true },
    );
    expect(query.mock.calls.map(([options]) => options)).toEqual([
      expect.objectContaining({
        query: DatabaseInstanceTypesDocument,
        fetchPolicy: "network-only",
      }),
      expect.objectContaining({
        query: ProjectsDocument,
        variables: { ownerId: "tea-1" },
      }),
      expect.objectContaining({
        query: GitConnectionsDocument,
        variables: { ownerId: "tea-1" },
      }),
    ]);
  });

  it("reads the cache on a re-run and skips workspace reads with no workspace", async () => {
    const { context, query } = contextWith(null);
    await prefetchCreatePage(
      { context, cause: "stay" },
      { catalog: DatabaseInstanceTypesDocument, git: true },
    );
    expect(query).toHaveBeenCalledTimes(1);
    expect(query).toHaveBeenCalledWith(
      expect.objectContaining({ fetchPolicy: "cache-first" }),
    );
  });

  it("never blocks the page on a failed read", async () => {
    const { context, query } = contextWith("tea-1");
    query.mockRejectedValue(new Error("offline"));
    await expect(
      prefetchCreatePage({ context, cause: "enter" }, { git: true }),
    ).resolves.toBeUndefined();
  });
});
