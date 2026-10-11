import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";
import { renderHook } from "@testing-library/react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { ServerDocument, ServicesDocument } from "@/graphql/definitions";
import { WorkspaceContext } from "@/features/workspaces/context/context";
import type { WorkspaceContextValue } from "@/features/workspaces/context/context";
import { useCachedServiceType } from "@/features/services/hooks/use-cached-service-type";

const WORKSPACE = "tea-1";

function setup(seed: (cache: InMemoryCache) => void) {
  const cache = new InMemoryCache();
  seed(cache);
  const client = new ApolloClient({ cache, link: ApolloLink.empty() });
  const workspace = {
    currentWorkspaceId: WORKSPACE,
  } as WorkspaceContextValue;
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>
      <WorkspaceContext.Provider value={workspace}>
        {children}
      </WorkspaceContext.Provider>
    </ApolloProvider>
  );
  return (id: string) =>
    renderHook(() => useCachedServiceType(id), { wrapper }).result.current;
}

// Partial writes, as a backend omitting an unrelated field would leave them:
// only `type` decides the skeleton, so the read must not need the rest.
function writeServer(cache: InMemoryCache, id: string, type: string) {
  cache.writeQuery({
    query: ServerDocument,
    variables: { id },
    data: { server: { __typename: "Service", id, type } } as never,
  });
}

describe("useCachedServiceType (w4/m196)", () => {
  it("reads the type from the service's own cached server read", () => {
    const read = setup((cache) => writeServer(cache, "srv-a", "cron_job"));
    expect(read("srv-a")).toBe("cron_job");
  });

  it("falls back to the cached workspace services list", () => {
    const read = setup((cache) =>
      cache.writeQuery({
        query: ServicesDocument,
        variables: { ownerId: WORKSPACE },
        data: {
          services: [
            { __typename: "Service", id: "srv-w", type: "background_worker" },
          ],
        } as never,
      }),
    );
    expect(read("srv-w")).toBe("background_worker");
  });

  it("stays undecided (null) when nothing cached knows the service", () => {
    const read = setup((cache) => writeServer(cache, "srv-a", "web_service"));
    expect(read("srv-other")).toBeNull();
  });
});
