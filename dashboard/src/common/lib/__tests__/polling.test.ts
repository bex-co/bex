import { describe, it, expect, afterEach, vi } from "vitest";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { PRIMED_FETCH_POLICY } from "@/common/lib/fetch-policy";

// The two shared primitives w9/m62 leans on across the metrics/deploy/usage
// hooks (visibility-gated polling) and the primed list/detail hooks
// (cache-first mount). Guarding them here catches a regression once, centrally.

describe("skipPollWhenHidden", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("skips the poll tick while the document is hidden", () => {
    vi.spyOn(document, "hidden", "get").mockReturnValue(true);
    expect(skipPollWhenHidden()).toBe(true);
  });

  it("allows the poll tick while the document is visible", () => {
    vi.spyOn(document, "hidden", "get").mockReturnValue(false);
    expect(skipPollWhenHidden()).toBe(false);
  });
});

describe("PRIMED_FETCH_POLICY", () => {
  it("reads the warm (SSR/prefetch-primed) cache without a mount refetch", () => {
    // cache-first is what makes a primed navigation render from cache with no
    // duplicate network request; a regression to cache-and-network would refire
    // on mount and throw the prefetch away.
    expect(PRIMED_FETCH_POLICY).toBe("cache-first");
  });
});
