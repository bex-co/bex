import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useConnectionInfo } from "@/features/keyvalue/hooks/use-connection-info";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

const mockQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ query: mockQuery }),
}));

beforeEach(() => {
  mockQuery.mockReset();
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
});

function connectionInfo(password: string) {
  const uri = `redis://default:${password}@kv.default.svc:6379`;
  return {
    data: {
      keyValueConnectionInfo: {
        __typename: "KeyValueConnectionInfo",
        internalConnectionString: uri,
        externalConnectionString: "",
        cliCommand: `redis-cli -u ${uri}`,
      },
    },
  };
}

/** Renders the hook and records the URI every render handed back. */
function renderLogged(id: string) {
  const rendered: { id: string; uri: string | undefined }[] = [];
  const hook = renderHook(
    ({ id }) => {
      const value = useConnectionInfo(id);
      rendered.push({ id, uri: value.info?.internalConnectionString });
      return value;
    },
    { initialProps: { id } },
  );
  return { ...hook, rendered };
}

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
          internalConnectionString:
            "redis://default:s3cret@kv.default.svc:6379",
          externalConnectionString: "",
          cliCommand: "redis-cli -u redis://default:s3cret@kv.default.svc:6379",
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
      "redis://default:s3cret@kv.default.svc:6379",
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

  it("re-masks on another store without ever rendering the old reveal under its id", async () => {
    mockQuery.mockResolvedValue(connectionInfo("pw-a"));
    const { result, rerender, rendered } = renderLogged("kv-a");
    await act(async () => {
      await result.current.reveal();
    });
    expect(result.current.info?.internalConnectionString).toContain("pw-a");

    rerender({ id: "kv-b" });

    expect(result.current.info).toBeNull();
    expect(rendered.filter((r) => r.id === "kv-b" && r.uri)).toEqual([]);
  });

  it("drops a reveal that resolves after switching to another store", async () => {
    let resolve!: (value: unknown) => void;
    mockQuery.mockReturnValue(new Promise((r) => (resolve = r)));
    const { result, rerender, rendered } = renderLogged("kv-a");
    let pending!: Promise<void>;
    act(() => {
      pending = result.current.reveal();
    });

    rerender({ id: "kv-b" });
    await act(async () => {
      resolve(connectionInfo("pw-a"));
      await pending;
    });

    expect(result.current.info).toBeNull();
    expect(result.current.loading).toBe(false);
    expect(rendered.filter((r) => r.uri)).toEqual([]);
  });

  it("drops the reveal on access loss and on an access-generation change (w6/m144)", async () => {
    mockQuery.mockResolvedValue(connectionInfo("pw"));
    const { result, rerender, rendered } = renderLogged("kv");
    await act(async () => {
      await result.current.reveal();
    });
    expect(result.current.info).not.toBeNull();

    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canViewSensitive: false }),
    );
    rendered.length = 0;
    rerender({ id: "kv" });
    expect(result.current.info).toBeNull();
    expect(rendered.filter((r) => r.uri)).toEqual([]);

    // Regaining access does not bring the dropped reveal back.
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
    rerender({ id: "kv" });
    expect(result.current.info).toBeNull();

    await act(async () => {
      await result.current.reveal();
    });
    expect(result.current.info).not.toBeNull();
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ generation: 2 }),
    );
    rendered.length = 0;
    rerender({ id: "kv" });
    expect(result.current.info).toBeNull();
    expect(rendered.filter((r) => r.uri)).toEqual([]);
  });
});
