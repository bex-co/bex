import { appendUnique, knownEventType, mergeTimeline } from "../timeline";

describe("supervision timeline", () => {
  it("orders deploys and events newest-first", () => {
    const items = mergeTimeline(
      [
        {
          id: "dep-one",
          status: "live",
          createdAt: "2026-01-01T10:00:00Z",
          updatedAt: "2026-01-01T10:03:00Z",
          commitId: null,
          commitMessage: null,
          image: null,
          trigger: null,
        },
      ],
      [
        {
          id: "evt-one",
          cursor: "cursor-one",
          type: "deploy_started",
          timestamp: "2026-01-01T10:04:00Z",
          details: null,
        },
      ],
    );
    expect(items.map((item) => item.key)).toEqual([
      "event:evt-one",
      "deploy:dep-one",
    ]);
  });

  it("deduplicates overlapping keyset pages", () => {
    expect(
      appendUnique(
        [{ id: "a" }, { id: "b" }],
        [{ id: "b" }, { id: "c" }],
        (item) => item.id,
      ).map((item) => item.id),
    ).toEqual(["a", "b", "c"]);
  });

  it("falls back for unknown event vocabulary", () => {
    expect(knownEventType("deploy_started")).toBe(true);
    expect(knownEventType("future_platform_event")).toBe(false);
  });
});
