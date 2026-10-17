import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider, useQuery } from "@apollo/client/react";
import type {
  DocumentNode,
  FragmentDefinitionNode,
  OperationDefinitionNode,
  SelectionSetNode,
} from "graphql";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import { ServerDocument, ServicesDocument } from "@/graphql/definitions";
import { useDisplayName } from "../use-display-name";

const invalidate = vi.fn(async () => undefined);
const toasts = vi.hoisted(() => ({
  warning: [] as Array<{ action?: { onClick: () => void } }>,
  success: [] as string[],
}));
vi.mock("sonner", () => ({
  toast: {
    success: (message: string) => toasts.success.push(message),
    error: vi.fn(),
    warning: (
      _message: string,
      options: { action?: { onClick: () => void } },
    ) => toasts.warning.push(options),
  },
}));
vi.mock("@tanstack/react-router", async (original) => ({
  ...(await original<typeof import("@tanstack/react-router")>()),
  useRouter: () => ({ invalidate }),
}));

/**
 * w4/m225 (a879): a Server read that started before SetDisplayName finished
 * after it and put the old label back, while the rename already reported
 * success. Real Apollo with the app's cache and defaults; only the network is
 * faked, and the old reads are held in flight across the write.
 */

// Every field a selection set selects, as null: a complete cache write
// without copying the Server selection set.
function blank(
  set: SelectionSetNode,
  fragments: Map<string, FragmentDefinitionNode>,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const s of set.selections) {
    if (s.kind === "Field") out[s.alias?.value ?? s.name.value] = null;
    else if (s.kind === "FragmentSpread")
      Object.assign(
        out,
        blank(fragments.get(s.name.value)!.selectionSet, fragments),
      );
    else Object.assign(out, blank(s.selectionSet, fragments));
  }
  return out;
}
function serviceShape(doc: DocumentNode, field: string) {
  const fragments = new Map(
    doc.definitions
      .filter(
        (d): d is FragmentDefinitionNode => d.kind === "FragmentDefinition",
      )
      .map((d) => [d.name.value, d]),
  );
  const op = doc.definitions.find(
    (d): d is OperationDefinitionNode => d.kind === "OperationDefinition",
  )!;
  const root = op.selectionSet.selections.find(
    (s) => s.kind === "Field" && s.name.value === field,
  );
  if (!root || root.kind !== "Field" || !root.selectionSet)
    throw new Error(field);
  return blank(root.selectionSet, fragments);
}
const SERVER_SHAPE = serviceShape(ServerDocument, "server");
const SERVICES_SHAPE = serviceShape(ServicesDocument, "services");

function fakeApi() {
  const state = { label: "Old label", hold: false, failReads: 0, renames: 0 };
  const held: Array<() => void> = [];
  const reads = { Server: 0, Services: 0 };
  const service = (shape: Record<string, unknown>, label: string) => ({
    ...shape,
    __typename: "Service",
    id: "srv-1",
    name: "web",
    displayName: label,
  });
  const link = new ApolloLink(
    (op) =>
      new Observable((sub) => {
        const reply = (result: Record<string, unknown>) => {
          sub.next(result);
          sub.complete();
        };
        if (op.operationName === "SetDisplayName") {
          state.renames += 1;
          state.label = (op.variables as { displayName: string }).displayName;
          setTimeout(() =>
            reply({ data: { setDisplayName: service({}, state.label) } }),
          );
          return;
        }
        const name = op.operationName as "Server" | "Services";
        reads[name] += 1;
        if (state.label !== "Old label" && state.failReads > 0) {
          state.failReads -= 1;
          setTimeout(() => sub.error(new Error("503 Service Unavailable")));
          return;
        }
        // The answer as of when the read STARTED.
        const snapshot =
          name === "Server"
            ? { data: { server: service(SERVER_SHAPE, state.label) } }
            : { data: { services: [service(SERVICES_SHAPE, state.label)] } };
        const send = () => setTimeout(() => reply(snapshot));
        if (state.hold) held.push(send);
        else send();
      }),
  );
  const client = new ApolloClient({
    link,
    cache: new InMemoryCache(apolloCacheConfig),
    defaultOptions: apolloDefaultOptions,
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  return {
    client,
    wrapper,
    state,
    reads,
    release: () => {
      state.hold = false;
      held.splice(0).forEach((send) => send());
    },
  };
}

// The Settings page's two label readers plus the rename control.
function useSettings() {
  const server = useQuery(ServerDocument, { variables: { id: "srv-1" } });
  const services = useQuery(ServicesDocument, {
    variables: { ownerId: "tea-1" },
  });
  return {
    serverLabel: server.data?.server?.displayName,
    listLabel: services.data?.services?.[0]?.displayName,
    rename: useDisplayName(),
  };
}

describe("service rename publication (w4/m225)", () => {
  beforeEach(() => {
    invalidate.mockClear();
    toasts.warning.length = 0;
    toasts.success.length = 0;
  });

  it("publishes the saved label after reads that started before the write", async () => {
    const api = fakeApi();
    const { result } = renderHook(() => useSettings(), {
      wrapper: api.wrapper,
    });
    await waitFor(() => expect(result.current.serverLabel).toBe("Old label"));
    await waitFor(() => expect(result.current.listLabel).toBe("Old label"));

    // Background reads of both documents start, and are held, before the
    // write; reads issued after it go straight through.
    api.state.hold = true;
    void api.client.refetchQueries({ include: "active" }).catch(() => []);
    api.state.hold = false;

    let saved!: Promise<boolean>;
    act(() => {
      saved = result.current.rename.setDisplayName("srv-1", "New label");
    });
    await waitFor(() => expect(api.state.label).toBe("New label"));
    await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
    // The write landed, but the older reads have not: not reported done, and
    // no authoritative read issued yet.
    expect(result.current.rename.busy).toBe(true);
    expect(invalidate).not.toHaveBeenCalled();
    expect(api.reads.Server).toBe(2);

    await act(async () => {
      api.release();
      expect(await saved).toBe(true);
    });
    // The older reads landed first; the final authoritative read wins.
    await waitFor(() => expect(result.current.serverLabel).toBe("New label"));
    expect(result.current.listLabel).toBe("New label");
    expect(result.current.rename.busy).toBe(false);
    expect(invalidate).toHaveBeenCalledTimes(1);
    // One authoritative read, issued after the held ones settled.
    expect(api.reads.Server).toBe(3);
  });

  it("reports a failed refresh after a committed rename and retries only the read", async () => {
    const api = fakeApi();
    const { result } = renderHook(() => useSettings(), {
      wrapper: api.wrapper,
    });
    await waitFor(() => expect(result.current.serverLabel).toBe("Old label"));
    api.state.failReads = 1; // the authoritative post-write read fails once

    await act(async () => {
      // The write committed, so the rename itself succeeded.
      expect(
        await result.current.rename.setDisplayName("srv-1", "New label"),
      ).toBe(true);
    });
    expect(toasts.warning).toHaveLength(1);
    expect(toasts.success).toHaveLength(0);
    // Editing waits for a successful re-read.
    expect(result.current.rename.unpublished).toBe(true);

    await act(async () => {
      toasts.warning[0].action!.onClick();
    });
    await waitFor(() => expect(toasts.success).toHaveLength(1));
    await waitFor(() => expect(result.current.serverLabel).toBe("New label"));
    expect(result.current.rename.unpublished).toBe(false);
    expect(api.state.renames).toBe(1);
  });

  it("re-reads only once when no older read is in flight", async () => {
    const api = fakeApi();
    const { result } = renderHook(() => useSettings(), {
      wrapper: api.wrapper,
    });
    await waitFor(() => expect(result.current.listLabel).toBe("Old label"));
    await act(async () => {
      expect(
        await result.current.rename.setDisplayName("srv-1", "New label"),
      ).toBe(true);
    });
    expect(api.reads).toEqual({ Server: 2, Services: 1 });
    await waitFor(() => expect(result.current.listLabel).toBe("New label"));
  });

  it("toasts and refreshes nothing on a page the customer left", async () => {
    const api = fakeApi();
    const { result, unmount } = renderHook(() => useSettings(), {
      wrapper: api.wrapper,
    });
    await waitFor(() => expect(result.current.serverLabel).toBe("Old label"));
    api.state.hold = true;
    void api.client.refetchQueries({ include: "active" }).catch(() => []);
    api.state.hold = false;
    let saved!: Promise<boolean>;
    act(() => {
      saved = result.current.rename.setDisplayName("srv-1", "New label");
    });
    await waitFor(() => expect(api.state.label).toBe("New label"));
    unmount();
    api.release();
    expect(await saved).toBe(true);
    // The cache still learns the saved label for when they come back.
    await waitFor(() => expect(api.reads.Server).toBe(3));
    expect(toasts.success).toHaveLength(0);
    expect(invalidate).not.toHaveBeenCalled();
  });
});
