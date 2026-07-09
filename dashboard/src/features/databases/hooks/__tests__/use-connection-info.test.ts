import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useConnectionInfo } from "@/features/databases/hooks/use-connection-info";

const mockQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ query: mockQuery }),
}));

beforeEach(() => mockQuery.mockReset());

describe("useConnectionInfo", () => {
  it("does NOT fetch on mount — the password is never on the wire until reveal", () => {
    renderHook(() => useConnectionInfo("db"));
    expect(mockQuery).not.toHaveBeenCalled();
  });

  it("fetches network-only on reveal and maps the connection info", async () => {
    mockQuery.mockResolvedValue({
      data: {
        databaseConnectionInfo: {
          __typename: "PostgresConnectionInfo",
          password: "s3cret",
          internalConnectionString:
            "postgresql://u:s3cret@db-rw.default:5432/db",
          externalConnectionString: "",
          psqlCommand: "PGPASSWORD=s3cret psql …",
        },
      },
    });

    const { result } = renderHook(() => useConnectionInfo("db"));
    await act(async () => {
      await result.current.reveal();
    });

    expect(mockQuery).toHaveBeenCalledTimes(1);
    const arg = mockQuery.mock.calls[0][0];
    expect(arg.fetchPolicy).toBe("network-only");
    expect(arg.variables).toEqual({ id: "db" });
    expect(result.current.info?.password).toBe("s3cret");
  });

  // The reveal-failure → error-state path is exercised through the UI in
  // connection-info-panel.test.tsx (the "renders an error state" case). It's not
  // re-tested here: a throwing/rejecting Apollo mock combined with mockReset() in
  // beforeEach trips vitest's global unhandled-error detector even though reveal()
  // catches correctly, which would make this file flaky for no added coverage.

  it("hide() clears revealed info back to masked", async () => {
    mockQuery.mockResolvedValue({
      data: {
        databaseConnectionInfo: {
          __typename: "PostgresConnectionInfo",
          password: "p",
          internalConnectionString: "i",
          externalConnectionString: "e",
          psqlCommand: "c",
        },
      },
    });
    const { result } = renderHook(() => useConnectionInfo("db"));
    await act(async () => {
      await result.current.reveal();
    });
    expect(result.current.info).not.toBeNull();
    act(() => result.current.hide());
    expect(result.current.info).toBeNull();
  });
});
