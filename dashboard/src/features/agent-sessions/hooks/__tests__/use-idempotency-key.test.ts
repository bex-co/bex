import { describe, expect, it } from "vitest";
import { renderHook } from "@testing-library/react";
import { useIdempotencyKey } from "@/features/agent-sessions/hooks/use-idempotency-key";

describe("useIdempotencyKey", () => {
  it("hands the same key to a retry of the same request and a new one to an edited request", () => {
    const { result } = renderHook(() => useIdempotencyKey());
    const first = result.current.keyFor("request A");
    expect(result.current.keyFor("request A")).toBe(first);

    const edited = result.current.keyFor("request B");
    expect(edited).not.toBe(first);
    // Going back to the earlier text is a new request too: the old key is gone.
    expect(result.current.keyFor("request A")).not.toBe(first);
  });

  it("forgets the key once its request succeeded", () => {
    const { result } = renderHook(() => useIdempotencyKey());
    const first = result.current.keyFor("request A");
    result.current.release();
    expect(result.current.keyFor("request A")).not.toBe(first);
  });

  it("keeps its functions stable across renders", () => {
    const { result, rerender } = renderHook(() => useIdempotencyKey());
    const { keyFor, release } = result.current;
    rerender();
    expect(result.current.keyFor).toBe(keyFor);
    expect(result.current.release).toBe(release);
  });
});
