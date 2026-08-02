import { NetworkStatus } from "@apollo/client";
import { hasAuthoritativeCurrentEvidence } from "../authoritative-evidence";

describe("authoritative mutation evidence", () => {
  it("accepts only successful current network evidence", () => {
    expect(
      hasAuthoritativeCurrentEvidence({
        networkStatus: NetworkStatus.ready,
        error: null,
        hasData: true,
      }),
    ).toBe(true);
    for (const networkStatus of [
      NetworkStatus.loading,
      NetworkStatus.setVariables,
      NetworkStatus.fetchMore,
      NetworkStatus.refetch,
      NetworkStatus.poll,
      NetworkStatus.error,
    ]) {
      expect(
        hasAuthoritativeCurrentEvidence({
          networkStatus,
          error: null,
          hasData: true,
        }),
      ).toBe(false);
    }
    expect(
      hasAuthoritativeCurrentEvidence({
        networkStatus: NetworkStatus.ready,
        error: new Error("refresh failed"),
        hasData: true,
      }),
    ).toBe(false);
    expect(
      hasAuthoritativeCurrentEvidence({
        networkStatus: NetworkStatus.ready,
        error: null,
        hasData: false,
      }),
    ).toBe(false);
  });
});
