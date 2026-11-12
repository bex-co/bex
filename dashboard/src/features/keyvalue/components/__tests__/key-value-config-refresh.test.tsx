import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import { RESOURCE_POLL_INTERVAL_MS } from "@/common/lib/polling";
import { useKeyValue } from "@/features/keyvalue/hooks/use-key-value";
import { KeyValuePersistenceModeSection } from "../key-value-persistence-mode-section";
import { KeyValueMaxmemoryPolicySection } from "../key-value-maxmemory-policy-section";

/**
 * w4/m226 (a881): the polled detail read omitted the configuration fields, so
 * a page left open kept the persistence mode it first read. After another
 * client turned persistence Off, a change back to a durable mode skipped the
 * typed data-loss confirmation. Real Apollo with the app's cache and defaults,
 * the real poll owner and editors; only the network and clock are faked.
 */
function fakeApi() {
  const store = {
    persistenceMode: "journal_snapshot",
    maxmemoryPolicy: "volatile_ttl",
  };
  const writes: string[] = [];
  // Detail reads started while `holdDetail` is set wait for `release()`,
  // answering as of when they started.
  const control = { holdDetail: false, held: [] as Array<() => void> };
  const keyValue = (fields: Record<string, unknown>) => ({
    __typename: "KeyValue",
    id: "red-abc",
    ...fields,
  });
  const link = new ApolloLink(
    (op) =>
      new Observable((sub) => {
        const send = (data: Record<string, unknown>) =>
          setTimeout(() => {
            sub.next({ data });
            sub.complete();
          });
        switch (op.operationName) {
          case "KeyValue": {
            const answer = {
              keyValue: keyValue({
                name: "cache-a",
                plan: "free",
                version: "8",
                status: "available",
                statusReason: null,
                statusReasonCode: null,
                suspended: "not_suspended",
                createdAt: "2026-10-17T00:00:00Z",
                updatedAt: "2026-10-17T00:00:00Z",
                externalHost: null,
                public: false,
                region: "fsn1",
                ...store,
              }),
            };
            if (control.holdDetail) control.held.push(() => send(answer));
            else send(answer);
            return;
          }
          case "KeyValuePersistenceMode":
            send({
              keyValue: keyValue({ persistenceMode: store.persistenceMode }),
            });
            return;
          case "SetKeyValueMaxmemoryPolicy":
            store.maxmemoryPolicy = (
              op.variables as { maxmemoryPolicy: string }
            ).maxmemoryPolicy;
            send({
              setKeyValueMaxmemoryPolicy: keyValue({
                maxmemoryPolicy: store.maxmemoryPolicy,
              }),
            });
            return;
          case "KeyValueMaxmemoryPolicy":
            send({
              keyValue: keyValue({ maxmemoryPolicy: store.maxmemoryPolicy }),
            });
            return;
          default:
            writes.push(op.operationName ?? "anonymous");
            sub.error(new Error(`unexpected ${op.operationName}`));
        }
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
    store,
    writes,
    wrapper,
    control,
    release: () => {
      control.holdDetail = false;
      control.held.splice(0).forEach((send) => send());
    },
  };
}

// The detail page's poll owner beside both configuration editors.
function KeyValuePage() {
  useKeyValue("red-abc");
  return (
    <>
      <KeyValueMaxmemoryPolicySection id="red-abc" />
      <KeyValuePersistenceModeSection id="red-abc" name="cache-a" />
    </>
  );
}

describe("Key Value configuration refresh (w4/m226)", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("asks for the data-loss confirmation after another client turned persistence Off", async () => {
    const api = fakeApi();
    const user = userEvent.setup({
      advanceTimers: vi.advanceTimersByTime,
      pointerEventsCheck: 0,
    });
    render(<KeyValuePage />, { wrapper: api.wrapper });
    const persistence = () =>
      screen.getByRole("combobox", { name: "Persistence mode" });
    await waitFor(() =>
      expect(persistence()).toHaveTextContent("Journal + Snapshot"),
    );

    // Another client confirms Off and changes the eviction policy.
    api.store.persistenceMode = "off";
    api.store.maxmemoryPolicy = "allkeys_lru";
    await act(() =>
      vi.advanceTimersByTimeAsync(RESOURCE_POLL_INTERVAL_MS + 10),
    );
    await waitFor(() => expect(persistence()).toHaveTextContent("Off"));
    expect(
      screen.getByRole("combobox", { name: "Maxmemory policy" }),
    ).toHaveTextContent("allkeys-lru");

    await user.click(
      screen.getByRole("button", { name: "Edit persistence mode" }),
    );
    await user.click(persistence());
    await user.click(screen.getByRole("option", { name: "Snapshot only" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    const dialog = screen.getByRole("alertdialog");
    expect(dialog).toHaveTextContent("sudo clear key value cache-a");
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(api.writes).toEqual([]);
  });

  it("keeps a saved policy when a detail poll sent before the save lands after it", async () => {
    const api = fakeApi();
    const user = userEvent.setup({
      advanceTimers: vi.advanceTimersByTime,
      pointerEventsCheck: 0,
    });
    render(<KeyValuePage />, { wrapper: api.wrapper });
    const policy = () =>
      screen.getByRole("combobox", { name: "Maxmemory policy" });
    await waitFor(() => expect(policy()).toHaveTextContent("volatile-ttl"));

    // The next poll starts, answering with the old policy, and is held.
    api.control.holdDetail = true;
    await act(() =>
      vi.advanceTimersByTimeAsync(RESOURCE_POLL_INTERVAL_MS + 10),
    );
    expect(api.control.held).toHaveLength(1);
    api.control.holdDetail = false;

    await user.click(screen.getByRole("button", { name: /Edit maxmemory/i }));
    await user.click(policy());
    await user.click(screen.getByRole("option", { name: /allkeys-lru/ }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(api.store.maxmemoryPolicy).toBe("allkeys-lru"));
    await act(async () => {
      api.release();
      await vi.advanceTimersByTimeAsync(10);
    });
    await waitFor(() => expect(policy()).toHaveTextContent("allkeys-lru"));
    await act(() => vi.advanceTimersByTimeAsync(50));
    expect(policy()).toHaveTextContent("allkeys-lru");
  });
});
