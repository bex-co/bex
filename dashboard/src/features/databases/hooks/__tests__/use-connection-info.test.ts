import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useConnectionInfo } from "@/features/databases/hooks/use-connection-info";
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
  return {
    data: {
      databaseConnectionInfo: {
        __typename: "PostgresConnectionInfo",
        password,
        internalConnectionString: `postgresql://u:${password}@db-rw:5432/db`,
        externalConnectionString: "",
        psqlCommand: `PGPASSWORD=${password} psql`,
      },
    },
  };
}

/** Renders the hook and records the password every render handed back. */
function renderLogged(id: string) {
  const rendered: { id: string; password: string | undefined }[] = [];
  const hook = renderHook(
    ({ id }) => {
      const value = useConnectionInfo(id);
      rendered.push({ id, password: value.info?.password });
      return value;
    },
    { initialProps: { id } },
  );
  return { ...hook, rendered };
}

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

  it("re-masks on another database without ever rendering the old reveal under its id", async () => {
    mockQuery.mockResolvedValue(connectionInfo("pw-a"));
    const { result, rerender, rendered } = renderLogged("db-a");
    await act(async () => {
      await result.current.reveal();
    });
    expect(result.current.info?.password).toBe("pw-a");

    rerender({ id: "db-b" });

    expect(result.current.info).toBeNull();
    expect(rendered.filter((r) => r.id === "db-b" && r.password)).toEqual([]);
  });

  it("drops a reveal that resolves after switching to another database", async () => {
    let resolve!: (value: unknown) => void;
    mockQuery.mockReturnValue(new Promise((r) => (resolve = r)));
    const { result, rerender, rendered } = renderLogged("db-a");
    let pending!: Promise<void>;
    act(() => {
      pending = result.current.reveal();
    });

    rerender({ id: "db-b" });
    await act(async () => {
      resolve(connectionInfo("pw-a"));
      await pending;
    });

    expect(result.current.info).toBeNull();
    expect(result.current.loading).toBe(false);
    expect(rendered.filter((r) => r.password)).toEqual([]);
  });

  it("drops the reveal on access loss and on an access-generation change (w6/m144)", async () => {
    mockQuery.mockResolvedValue(connectionInfo("pw"));
    const { result, rerender, rendered } = renderLogged("db");
    await act(async () => {
      await result.current.reveal();
    });
    expect(result.current.info?.password).toBe("pw");

    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canViewSensitive: false }),
    );
    rendered.length = 0;
    rerender({ id: "db" });
    expect(result.current.info).toBeNull();
    expect(rendered.filter((r) => r.password)).toEqual([]);

    // Regaining access does not bring the dropped reveal back.
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
    rerender({ id: "db" });
    expect(result.current.info).toBeNull();

    await act(async () => {
      await result.current.reveal();
    });
    expect(result.current.info?.password).toBe("pw");
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ generation: 2 }),
    );
    rendered.length = 0;
    rerender({ id: "db" });
    expect(result.current.info).toBeNull();
    expect(rendered.filter((r) => r.password)).toEqual([]);
  });
});
