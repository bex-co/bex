import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useSetKeyValueMaxmemoryPolicy } from "../use-set-key-value-maxmemory-policy";
import { useSetKeyValuePersistenceMode } from "../use-set-key-value-persistence-mode";

let mutate = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: () => ({
    data: undefined,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
  useMutation: () => [
    (...args: unknown[]) => mutate(...args),
    { loading: false },
  ],
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

beforeEach(() => {
  mutate = vi.fn();
});

// A maxmemory or persistence save restarts the store. The hooks hand the page
// a callback so the detail header polls through that restart instead of
// sitting on "Available" at the baseline cadence (w4/m137 t008).
describe.each([
  [
    "maxmemory policy",
    (onSaved: () => void) => useSetKeyValueMaxmemoryPolicy("red-x", onSaved),
    "allkeys-lfu",
  ],
  [
    "persistence mode",
    (onSaved: () => void) => useSetKeyValuePersistenceMode("red-x", onSaved),
    "snapshot",
  ],
])("%s save", (_name, useHook, value) => {
  it("tells the page after a successful save", async () => {
    mutate.mockResolvedValue({});
    const onSaved = vi.fn();
    const { result } = renderHook(() => useHook(onSaved));

    let ok = false;
    await act(async () => {
      ok = await result.current.save(value);
    });
    expect(ok).toBe(true);
    expect(onSaved).toHaveBeenCalledTimes(1);
  });

  it("stays quiet when the save fails", async () => {
    mutate = vi.fn().mockRejectedValue(new Error("nope"));
    const onSaved = vi.fn();
    const { result } = renderHook(() => useHook(onSaved));

    let ok = true;
    await act(async () => {
      ok = await result.current.save(value);
    });
    expect(ok).toBe(false);
    expect(onSaved).not.toHaveBeenCalled();
  });
});
