import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { mockWorkspaceContext } from "@/test/mocks/workspace";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));
vi.mock("@/features/workspaces/context/hooks", () =>
  mockWorkspaceContext("tea-1"),
);
vi.mock("@/features/env-groups/hooks/use-env-group-scope-index", () => {
  const scope = {
    projects: [],
    environments: [],
    byId: new Map(),
    serviceEnvironmentById: new Map(),
    loading: false,
    ready: true,
    error: undefined,
    retry: () => undefined,
    ownerId: "tea-1",
  };
  return {
    useEnvGroupScopeIndex: () => scope,
    useWorkspaceEnvironmentIndex: () => scope,
  };
});
vi.mock("@/features/services/hooks/use-env-vars", () => ({
  useEnvVarKeys: () => ({ keys: [], loading: false, error: undefined }),
}));
vi.mock("@/features/services/hooks/use-secret-files", () => ({
  useSecretFileNames: () => ({ names: [], loading: false, error: undefined }),
}));

import { EnvGroupsPanel } from "@/features/services/components/env-groups-panel";

/**
 * w4/322: Link/Unlink answer a bare boolean, so the panel's Server read (link
 * order) stayed cached while the refetched group list already showed the new
 * membership: a just-linked group ranked below the old one and the page
 * struck through the winner's SHARED/shared.txt until a background poll. Real
 * Apollo cache, real panel and group hooks; only the network is faked, with
 * Server reads that can be held or failed after the write.
 */
interface FakeServer {
  linked: string[]; // link order, last wins
  holdServer: boolean;
  failServerReads: number;
  held: Array<() => void>;
}
let server: FakeServer;

const GROUPS = [
  { id: "evg-z", name: "z-new" },
  { id: "evg-a", name: "a-old" },
];

function serverData(id: string) {
  return {
    __typename: "Service",
    id,
    name: "web",
    immutableName: "web",
    slug: "web",
    displayName: "",
    type: "web_service",
    suspended: "not_suspended",
    dashboardUrl: "",
    region: "fsn1",
    url: "",
    publicRoutingNotice: "",
    undeployedChanges: false,
    undeployedChangesApplying: false,
    internalAddress: "",
    createdAt: "2026-10-16T00:00:00Z",
    sshAddress: "",
    phase: "Running",
    replicas: 1,
    revision: "rev-1",
    plan: "free",
    idleTTLSeconds: 0,
    maintenanceMode: null,
    repo: "",
    branch: "",
    imagePath: "docker.io/library/node:24",
    rootDir: "",
    runtime: "image",
    builder: "auto",
    buildCommand: "",
    startCommand: "",
    dockerfilePath: "",
    registryCredentialId: "",
    buildFilter: null,
    autoDeploy: false,
    environmentId: null,
    linkedEnvGroupIds: [...server.linked],
    pushDeliveryMethod: "none",
    notifyOnFail: "default",
    notificationsToSend: "default",
    renderSubdomainPolicy: null,
    port: 8080,
    healthCheckPath: "",
    maxShutdownDelaySeconds: null,
    preDeployCommand: "",
    schedule: "",
    command: "",
    lastSuccessfulRunAt: "",
    nextRunAt: "",
    runs: [],
    publishPath: "",
    routes: [],
    headers: [],
    ipAllowList: [],
    ipAllowListEntries: [],
    ipAllowListProxiedDomains: [],
    outboundIps: { __typename: "OutboundIPs", type: "shared", ips: [] },
  };
}

const link = new ApolloLink(
  (op) =>
    new Observable((sub) => {
      const v = op.variables as { id?: string; serviceId?: string };
      const send = (result: Record<string, unknown>) =>
        setTimeout(() => {
          sub.next(result);
          sub.complete();
        }, 0);
      const fail = () =>
        setTimeout(() => sub.error(new Error("503 Service Unavailable")), 0);
      switch (op.operationName) {
        case "LinkEnvGroup":
          server.linked = [...server.linked.filter((g) => g !== v.id), v.id!];
          send({ data: { linkEnvGroup: true } });
          return;
        case "UnlinkEnvGroup":
          server.linked = server.linked.filter((g) => g !== v.id);
          send({ data: { unlinkEnvGroup: true } });
          return;
        case "EnvGroups":
          send({
            data: {
              envGroups: GROUPS.map((group) => ({
                __typename: "EnvGroup",
                ...group,
                ownerId: "tea-1",
                environmentId: null,
                createdAt: null,
                updatedAt: null,
                revision: null,
                availability: null,
                serviceLinks: server.linked.includes(group.id) ? ["srv-1"] : [],
                envVars: [{ __typename: "EnvGroupVar", key: "SHARED" }],
                secretFiles: [
                  { __typename: "EnvGroupSecretFile", name: "shared.txt" },
                ],
              })),
            },
          });
          return;
        case "Server": {
          if (server.failServerReads > 0) {
            server.failServerReads -= 1;
            fail();
            return;
          }
          const respond = () => send({ data: { server: serverData(v.id!) } });
          if (server.holdServer) server.held.push(respond);
          else respond();
          return;
        }
      }
      sub.error(new Error(`unexpected operation ${op.operationName}`));
    }),
);

function renderPanel() {
  const client = new ApolloClient({
    link,
    cache: new InMemoryCache(apolloCacheConfig),
  });
  return render(
    <ApolloProvider client={client}>
      <EnvGroupsPanel serviceId="srv-1" />
    </ApolloProvider>,
  );
}

const loserTitle = (winner: string) =>
  `Overridden by ${winner}, which is linked later`;

function linkedSection() {
  return screen.getByRole("heading", { name: /Linked/ }).closest("section")!;
}

beforeEach(() => {
  server = {
    linked: ["evg-z"],
    holdServer: false,
    failServerReads: 0,
    held: [],
  };
});

describe("EnvGroupsPanel link order after a membership write (w4/322)", () => {
  it("claims no winner until the post-link Server read lands, then ranks the new link first", async () => {
    const user = userEvent.setup();
    renderPanel();
    await screen.findByRole("button", { name: /^Link$/ });

    server.holdServer = true;
    await user.click(screen.getByRole("button", { name: /^Link$/ }));
    // Membership is fresh (both linked) while the order read is held: the
    // stale cached order [Z] must not strike through A, the actual winner.
    await waitFor(() =>
      expect(within(linkedSection()).getAllByRole("link")).toHaveLength(2),
    );
    expect(
      document.querySelectorAll(`[title="${loserTitle("z-new")}"]`),
    ).toHaveLength(0);
    expect(document.querySelectorAll("s")).toHaveLength(0);
    expect(screen.getByRole("status")).toHaveTextContent(
      "Confirming which linked group wins",
    );

    await act(async () => {
      server.held.splice(0).forEach((respond) => respond());
    });
    await waitFor(() =>
      expect(
        document.querySelectorAll(`[title="${loserTitle("a-old")}"]`),
      ).toHaveLength(2),
    );
    const names = within(linkedSection())
      .getAllByRole("link")
      .map((node) => node.textContent?.trim());
    expect(names).toEqual(["a-old", "z-new"]);
    expect(screen.queryByText(/Confirming which linked group wins/)).toBeNull();
  });

  it("re-reads the order after an unlink and a relink", async () => {
    server.linked = ["evg-z", "evg-a"];
    const user = userEvent.setup();
    renderPanel();
    await waitFor(() =>
      expect(
        document.querySelectorAll(`[title="${loserTitle("a-old")}"]`),
      ).toHaveLength(2),
    );
    // Relinking Z last makes it the winner, with the same two-group membership.
    await user.click(
      within(linkedSection()).getAllByRole("button", { name: /Unlink/ })[1],
    );
    await waitFor(() =>
      expect(within(linkedSection()).getAllByRole("link")).toHaveLength(1),
    );
    await user.click(screen.getByRole("button", { name: /^Link$/ }));
    await waitFor(() =>
      expect(
        document.querySelectorAll(`[title="${loserTitle("z-new")}"]`),
      ).toHaveLength(2),
    );
    expect(
      document.querySelectorAll(`[title="${loserTitle("a-old")}"]`),
    ).toHaveLength(0);
  });

  it("keeps the write but leaves precedence unclaimed with a retry when the order read fails", async () => {
    const user = userEvent.setup();
    renderPanel();
    await screen.findByRole("button", { name: /^Link$/ });

    server.failServerReads = 1;
    await user.click(screen.getByRole("button", { name: /^Link$/ }));
    await screen.findByText(/couldn't confirm which group wins/);
    // The same failed read also leaves the service's scope unknown, which the
    // panel already reports; one successful re-read clears both.
    const alert = screen
      .getByText(/couldn't confirm which group wins/)
      .closest("[role=alert]") as HTMLElement;
    expect(alert).toHaveTextContent("couldn't confirm which group wins");
    expect(within(linkedSection()).getAllByRole("link")).toHaveLength(2);
    expect(document.querySelectorAll("s")).toHaveLength(0);
    expect(server.linked).toEqual(["evg-z", "evg-a"]);

    await user.click(within(alert).getByRole("button", { name: /Try again/ }));
    await waitFor(() =>
      expect(
        document.querySelectorAll(`[title="${loserTitle("a-old")}"]`),
      ).toHaveLength(2),
    );
    expect(screen.queryByRole("alert")).toBeNull();
    // The retry re-read; it did not repeat the link.
    expect(server.linked).toEqual(["evg-z", "evg-a"]);
  });
});
