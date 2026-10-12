import { describe, expect, it } from "vitest";
import {
  NO_APPENDED,
  appendPage,
  joinAppendedPages,
  type AppendedPages,
} from "../appended-pages";

const row = (id: string) => ({ id });
const rows = (...ids: string[]) => ids.map(row);

function tail(
  anchorId: string,
  ids: string[],
  lastPageSize = ids.length,
  key = "k",
): AppendedPages<{ id: string }> {
  return { key, anchorId, rows: rows(...ids), lastPageSize };
}

describe("joinAppendedPages", () => {
  it("appends a tail that hangs off the head's last row", () => {
    const joined = joinAppendedPages(
      rows("5", "4"),
      tail("4", ["3", "2"]),
      "k",
      2,
    );
    expect(joined.rows).toEqual(rows("5", "4", "3", "2"));
    expect(joined.next).toEqual({ key: "k", anchorId: "4", cursor: "2" });
  });

  it("drops the tail once the head no longer ends at its anchor", () => {
    // A gap (anchor fell off) and a shift that keeps the anchor mid-page both
    // mean the tail no longer continues the head.
    for (const head of [rows("7", "6"), rows("5", "4", "3")]) {
      const joined = joinAppendedPages(head, tail("4", ["2", "1"]), "k", 2);
      expect(joined.rows).toEqual(head);
      expect(joined.tail).toBe(NO_APPENDED);
      expect(joined.next?.anchorId).toBe(head.at(-1)?.id);
      expect(joined.next?.cursor).toBe(head.at(-1)?.id);
    }
  });

  it("drops an overlapping or foreign-context tail", () => {
    expect(
      joinAppendedPages(rows("5", "4"), tail("4", ["5"]), "k", 2).rows,
    ).toEqual(rows("5", "4"));
    expect(
      joinAppendedPages(rows("5", "4"), tail("4", ["3"], 1, "other"), "k", 2)
        .rows,
    ).toEqual(rows("5", "4"));
  });

  it("offers no Load more request without rows", () => {
    expect(joinAppendedPages([], NO_APPENDED, "k", 2).next).toBeNull();
  });
});

describe("joinAppendedPages hasMore", () => {
  it("follows the head until a page is appended, then the last page", () => {
    const hasMore = (head: string[], t: AppendedPages<{ id: string }>) =>
      joinAppendedPages(rows(...head), t, "k", 2).hasMore;
    expect(hasMore(["1", "2"], NO_APPENDED)).toBe(true);
    expect(hasMore(["1"], NO_APPENDED)).toBe(false);
    expect(hasMore(["1", "2"], tail("2", ["3"]))).toBe(false);
    expect(hasMore(["1", "2"], tail("2", ["3", "4"]))).toBe(true);
    // A tail that no longer joins hands the decision back to the head.
    expect(hasMore(["0", "1"], tail("2", ["3"]))).toBe(true);
  });
});

describe("appendPage", () => {
  it("starts a fresh tail when read from the head's end, replacing a stale one", () => {
    const stale = tail("3", ["2", "1"]);
    expect(
      appendPage(stale, { key: "k", anchorId: "4", cursor: "4" }, rows("3")),
    ).toEqual(tail("4", ["3"]));
  });

  it("extends the tail it was read from", () => {
    expect(
      appendPage(
        tail("4", ["3", "2"]),
        { key: "k", anchorId: "4", cursor: "2" },
        rows("2", "1"),
      ),
    ).toEqual({ ...tail("4", ["3", "2", "1"]), lastPageSize: 2 });
  });

  it("discards a page read from a tail that has since been replaced", () => {
    const current = tail("5", ["4"]);
    expect(
      appendPage(current, { key: "k", anchorId: "3", cursor: "2" }, rows("1")),
    ).toBe(current);
    expect(
      appendPage(
        current,
        { key: "other", anchorId: "5", cursor: "4" },
        rows("3"),
      ),
    ).toBe(current);
  });
});
