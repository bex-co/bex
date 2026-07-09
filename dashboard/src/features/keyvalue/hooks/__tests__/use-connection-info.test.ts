import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useConnectionInfo } from "@/features/keyvalue/hooks/use-connection-info";

const mockQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ query: mockQuery }),
}));

beforeEach(() => mockQuery.mockReset());

describe("useConnectionInfo", () => {
  it("does NOT fetch on mount — nothing is on the wire until reveal", () => {
    renderHook(() => useConnectionInfo("kv"));
    expect(mockQuery).not.toHaveBeenCalled();
  });

  it("fetches network-only on reveal and maps the connection info", async () => {
    mockQuery.mockResolvedValue({
      data: {
        keyValueConnectionInfo: {
          __typename: "KeyValueConnectionInfo",
          internalConnectionString: "redis://:s3cret@kv.default.svc:6379",
          externalConnectionString: "",
          cliCommand: "redis-cli -u redis://:s3cret@kv.default.svc:6379",
        },
      },
    });

    const { result } = renderHook(() => useConnectionInfo("kv"));
    await act(async () => {
      await result.current.reveal();
    });

    expect(mockQuery).toHaveBeenCalledTimes(1);
    const arg = mockQuery.mock.calls[0][0];
    expect(arg.fetchPolicy).toBe("network-only");
    expect(arg.variables).toEqual({ id: "kv" });
    expect(result.current.info?.internalConnectionString).toBe(
      "redis://:s3cret@kv.default.svc:6379",
    );
  });

  it("hide() clears revealed info back to masked", async () => {
    mockQuery.mockResolvedValue({
      data: {
        keyValueConnectionInfo: {
          __typename: "KeyValueConnectionInfo",
          internalConnectionString: "i",
          externalConnectionString: "e",
          cliCommand: "c",
        },
      },
    });
    const { result } = renderHook(() => useConnectionInfo("kv"));
    await act(async () => {
      await result.current.reveal();
    });
    expect(result.current.info).not.toBeNull();
    act(() => result.current.hide());
    expect(result.current.info).toBeNull();
  });
});
