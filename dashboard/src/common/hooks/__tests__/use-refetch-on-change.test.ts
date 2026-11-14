import { describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useRefetchOnChange } from "../use-refetch-on-change";

describe("useRefetchOnChange", () => {
  it("does not read on mount, reads when the key changes, and not for the same key", () => {
    const refetch = vi.fn().mockResolvedValue(undefined);
    const { rerender } = renderHook(
      ({ key }: { key: string | undefined }) =>
        useRefetchOnChange(key, refetch),
      { initialProps: { key: "running:1" as string | undefined } },
    );
    expect(refetch).not.toHaveBeenCalled();

    rerender({ key: "running:1" });
    expect(refetch).not.toHaveBeenCalled();

    rerender({ key: "completed:1" });
    expect(refetch).toHaveBeenCalledTimes(1);

    rerender({ key: "running:2" });
    expect(refetch).toHaveBeenCalledTimes(2);
  });

  it("takes note of a change while disabled without reading, and does not replay it", () => {
    const refetch = vi.fn().mockResolvedValue(undefined);
    const { rerender } = renderHook(
      ({ key, enabled }) => useRefetchOnChange(key, refetch, enabled),
      { initialProps: { key: "a", enabled: false } },
    );
    rerender({ key: "b", enabled: false });
    expect(refetch).not.toHaveBeenCalled();

    // Enabling alone is not a change of key: nothing is replayed.
    rerender({ key: "b", enabled: true });
    expect(refetch).not.toHaveBeenCalled();

    rerender({ key: "c", enabled: true });
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("swallows a failed read: the query reports it", async () => {
    const refetch = vi.fn().mockRejectedValue(new Error("offline"));
    const { rerender } = renderHook(
      ({ key }) => useRefetchOnChange(key, refetch),
      { initialProps: { key: "a" } },
    );
    rerender({ key: "b" });
    await Promise.resolve();
    expect(refetch).toHaveBeenCalledTimes(1);
  });
});
