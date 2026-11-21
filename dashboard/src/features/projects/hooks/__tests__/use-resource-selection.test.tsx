import { describe, it, expect } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useResourceSelection } from "@/features/projects/hooks/use-resource-selection";
import type { ResourceRow } from "@/features/projects/types";

function row(kind: ResourceRow["kind"], id: string): ResourceRow {
  return {
    kind,
    id,
    name: id,
    createdAt: null,
    updatedAt: null,
    runtime: null,
    region: null,
  };
}

const api = row("service", "api");
const db = row("database", "db");
const cache = row("keyvalue", "cache");

describe("useResourceSelection (w4/m550)", () => {
  it("drops rows that left the list, and brings nothing back on its own", () => {
    const { result, rerender } = renderHook(
      ({ rows }) => useResourceSelection(rows),
      { initialProps: { rows: [api, db, cache] } },
    );
    act(() =>
      result.current.setSelected(new Set(["service:api", "database:db"])),
    );
    expect(result.current.selectedRows).toEqual([api, db]);

    // The database moved away: it no longer counts, and the rows keep list order.
    rerender({ rows: [cache, api] });
    expect([...result.current.selected]).toEqual(["service:api"]);
    expect(result.current.selectedRows).toEqual([api]);
  });

  it("deselects only the rows an action changed, keeping a selection made meanwhile", () => {
    const { result } = renderHook(() => useResourceSelection([api, db, cache]));
    act(() =>
      result.current.setSelected(new Set(["service:api", "database:db"])),
    );
    // A deselect captured before the user added the cache to the selection.
    const deselect = result.current.deselect;
    act(() =>
      result.current.setSelected(
        new Set(["service:api", "database:db", "keyvalue:cache"]),
      ),
    );
    act(() => deselect(new Set(["service:api"])));
    expect(result.current.selectedRows).toEqual([db, cache]);
  });
});
