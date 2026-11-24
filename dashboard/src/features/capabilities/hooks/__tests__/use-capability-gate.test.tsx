import { afterEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { CREATE_REASON, mockCapabilities } from "@/test/mocks/capabilities";

afterEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
});

describe("useCapabilityGate (w9/m619)", () => {
  it("denies a refused role and names the role's reason", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const { result } = renderHook(() => useCapabilityGate("canCreate"));
    expect(result.current).toEqual({ denied: true, reason: CREATE_REASON });
  });

  it("allows a role that holds the capability, with no reason", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    const { result } = renderHook(() => useCapabilityGate("canCreate"));
    expect(result.current).toEqual({ denied: false, reason: undefined });
  });

  it("holds the control, fail-closed, without a reason while still checking", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canCreate: false, loading: true, loaded: false }),
    );
    const { result } = renderHook(() => useCapabilityGate("canCreate"));
    expect(result.current).toEqual({ denied: true, reason: undefined });
  });
});
