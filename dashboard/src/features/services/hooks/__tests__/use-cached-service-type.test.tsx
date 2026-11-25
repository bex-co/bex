import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";
import { renderHook } from "@testing-library/react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { ServerDocument, ServicesDocument } from "@/graphql/definitions";
import { WorkspaceContext } from "@/features/workspaces/context/context";
import type { WorkspaceContextValue } from "@/features/workspaces/context/context";
import {
  useCachedServiceName,
  useCachedServiceSource,
  useCachedServiceType,
} from "@/features/services/hooks/use-cached-service-type";

const WORKSPACE = "tea-1";

function setup<T>(
  seed: (cache: InMemoryCache) => void,
  hook: (id: string) => T,
) {
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
  return (id: string) => renderHook(() => hook(id), { wrapper }).result.current;
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

function writeServices(
  cache: InMemoryCache,
  services: Array<Record<string, unknown>>,
) {
  cache.writeQuery({
    query: ServicesDocument,
    variables: { ownerId: WORKSPACE },
    data: {
      services: services.map((service) => ({
        __typename: "Service",
        ...service,
      })),
    } as never,
  });
}

describe("useCachedServiceType (w4/m196)", () => {
  it("reads the type from the service's own cached server read", () => {
    const read = setup(
      (cache) => writeServer(cache, "srv-a", "cron_job"),
      useCachedServiceType,
    );
    expect(read("srv-a")).toBe("cron_job");
  });

  it("falls back to the cached workspace services list", () => {
    const read = setup(
      (cache) =>
        writeServices(cache, [{ id: "srv-w", type: "background_worker" }]),
      useCachedServiceType,
    );
    expect(read("srv-w")).toBe("background_worker");
  });

  it("stays undecided (null) when nothing cached knows the service", () => {
    const read = setup(
      (cache) => writeServer(cache, "srv-a", "web_service"),
      useCachedServiceType,
    );
    expect(read("srv-other")).toBeNull();
  });
});

// w3/m555: the header's loading frame lays the name out invisibly, so its
// first row wraps where the ready one does.
describe("useCachedServiceName (w3/m555)", () => {
  it("reads the name from the cached services list, else null", () => {
    const read = setup(
      (cache) =>
        writeServices(cache, [
          { id: "srv-a", type: "cron_job", name: "nightly" },
        ]),
      useCachedServiceName,
    );
    expect(read("srv-a")).toBe("nightly");
    expect(read("srv-other")).toBeNull();
  });
});

// w3/094: the header's loading frame draws a repository item only where the
// ready header will. bex-api answers `repo` as "" for an image-backed service.
describe("useCachedServiceSource (w3/094)", () => {
  it("reads an image-backed service's missing repository from the list", () => {
    const read = setup(
      (cache) =>
        writeServices(cache, [
          {
            id: "srv-img",
            type: "web_service",
            repo: "",
            registryCredentialId: null,
          },
          {
            id: "srv-git",
            type: "web_service",
            repo: "https://github.com/acme/app",
            registryCredentialId: null,
          },
        ]),
      useCachedServiceSource,
    );
    expect(read("srv-img")).toEqual({
      repository: false,
      registryCredential: false,
      cron: false,
      suspended: false,
    });
    expect(read("srv-git")).toEqual({
      repository: true,
      registryCredential: false,
      cron: false,
      suspended: false,
    });
  });

  // w3/m555: a cron job's header frame has its own action and facts, and a
  // suspended one's none and its notice.
  it("says which services are cron jobs and which are suspended", () => {
    const read = setup(
      (cache) =>
        writeServices(cache, [
          {
            id: "srv-run",
            type: "cron_job",
            repo: "",
            suspended: "not_suspended",
          },
          { id: "srv-off", type: "cron_job", repo: "", suspended: "suspended" },
        ]),
      useCachedServiceSource,
    );
    expect(read("srv-run")).toMatchObject({ cron: true, suspended: false });
    expect(read("srv-off")).toMatchObject({ cron: true, suspended: true });
  });

  it("says an image pulled with a registry credential names it", () => {
    const read = setup(
      (cache) =>
        writeServices(cache, [
          {
            id: "srv-img",
            type: "web_service",
            repo: null,
            registryCredentialId: "rgc-private",
          },
        ]),
      useCachedServiceSource,
    );
    expect(read("srv-img")).toEqual({
      repository: false,
      registryCredential: true,
      cron: false,
      suspended: false,
    });
  });

  it("prefers the service's own cached read", () => {
    const read = setup((cache) => {
      cache.writeQuery({
        query: ServerDocument,
        variables: { id: "srv-a" },
        data: {
          server: {
            __typename: "Service",
            id: "srv-a",
            repo: "https://github.com/acme/app",
            registryCredentialId: "rgc-docker",
          },
        } as never,
      });
    }, useCachedServiceSource);
    expect(read("srv-a")).toEqual({
      repository: true,
      registryCredential: true,
      cron: false,
      suspended: false,
    });
  });

  it("stays undecided until a cached read has answered repo", () => {
    const read = setup((cache) => {
      // The type alone, as a list from before w3/094 wrote it.
      writeServer(cache, "srv-a", "web_service");
      writeServices(cache, [{ id: "srv-b", type: "web_service" }]);
    }, useCachedServiceSource);
    expect(read("srv-a")).toBeNull();
    expect(read("srv-b")).toBeNull();
    expect(read("srv-other")).toBeNull();
  });
});
