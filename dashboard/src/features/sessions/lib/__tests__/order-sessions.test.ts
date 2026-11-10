import { describe, expect, it } from "vitest";
import { orderSessions } from "../order-sessions";
import type { SessionView } from "@/features/sessions/types";

const s = (
  id: string,
  authenticatedAt: string | null,
  current = false,
): SessionView => ({ id, current, authenticatedAt });

describe("orderSessions (w21/032)", () => {
  it("keeps the current session first, then newest sign-in first", () => {
    const out = orderSessions([
      s("old", "2026-11-04T00:00:00Z"),
      s("cur", "2026-11-01T00:00:00Z", true),
      s("new", "2026-11-09T00:00:00Z"),
      s("none", null),
      s("mid", "2026-11-06T00:00:00Z"),
    ]);
    expect(out.map((x) => x.id)).toEqual(["cur", "new", "mid", "old", "none"]);
  });

  it("does not mutate its input", () => {
    const input = [
      s("a", "2026-11-01T00:00:00Z"),
      s("b", "2026-11-02T00:00:00Z"),
    ];
    orderSessions(input);
    expect(input.map((x) => x.id)).toEqual(["a", "b"]);
  });
});
