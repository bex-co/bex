import { describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import type { Session } from "@ory/client-fetch";

const root = vi.hoisted(() => ({ session: null as Session | null }));
vi.mock("@/common/hooks/use-root-context", () => ({
  useRootContext: () => ({ session: root.session }),
}));

import { useHasVerifiedSession } from "@/common/hooks/use-has-verified-session";

// The rule itself is pinned in email-verification.test.ts (hasVerifiedSession);
// this checks the hook reads the root context's session.
describe("useHasVerifiedSession", () => {
  it("follows the root context's session", () => {
    root.session = null;
    expect(renderHook(() => useHasVerifiedSession()).result.current).toBe(
      false,
    );
    root.session = {
      id: "ses-1",
      identity: {
        traits: { email: "dev@example.com" },
        verifiable_addresses: [{ value: "dev@example.com", verified: true }],
      },
    } as unknown as Session;
    expect(renderHook(() => useHasVerifiedSession()).result.current).toBe(true);
  });
});
