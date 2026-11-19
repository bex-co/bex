import { describe, expect, it, vi } from "vitest";
import { defaultSavedSearchId } from "../project-log-views";
import { SavedSearchType } from "@/graphql/definitions";
import type { RouterContext } from "@/common/types/router-context";

// A view URL without `savedSearchId` opens the project's first view of its
// type, as Render's sidebar links always name one (w3/m505).
const views = [
  { id: "svs-metrics", type: SavedSearchType.Metrics },
  { id: "svs-app", type: SavedSearchType.ApplicationLogs },
  { id: "svs-req", type: SavedSearchType.RequestLogs },
  { id: "svs-app-2", type: SavedSearchType.ApplicationLogs },
];

function client(query: () => Promise<unknown>) {
  return { query: vi.fn(query) } as unknown as RouterContext["client"];
}

describe("defaultSavedSearchId", () => {
  it("answers the project's first view of the type, reading it fresh on entry", async () => {
    const c = client(async () => ({ data: { savedSearches: views } }));
    expect(
      await defaultSavedSearchId(
        c,
        "prj-1",
        SavedSearchType.ApplicationLogs,
        "enter",
      ),
    ).toBe("svs-app");
    expect(
      await defaultSavedSearchId(
        c,
        "prj-1",
        SavedSearchType.RequestLogs,
        "stay",
      ),
    ).toBe("svs-req");
    const calls = (c.query as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls[0][0]).toMatchObject({
      variables: { projectId: "prj-1" },
      fetchPolicy: "network-only",
    });
    expect(calls[1][0].fetchPolicy).toBe("cache-first");
  });

  it("answers nothing when the project has no such view or the read fails", async () => {
    expect(
      await defaultSavedSearchId(
        client(async () => ({ data: { savedSearches: [] } })),
        "prj-1",
        SavedSearchType.ApplicationLogs,
        "enter",
      ),
    ).toBeUndefined();
    expect(
      await defaultSavedSearchId(
        client(async () => {
          throw new Error("network");
        }),
        "prj-1",
        SavedSearchType.ApplicationLogs,
        "enter",
      ),
    ).toBeUndefined();
  });
});
