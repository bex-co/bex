import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import {
  Kind,
  type DocumentNode,
  type FragmentDefinitionNode,
  type SelectionSetNode,
} from "graphql";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import type { RouterContext } from "@/common/types/router-context";
import type { DeployView } from "@/features/deploys/hooks/use-deploy";
import { DeployNotices } from "@/features/deploys/components/deploy-notices";
import { ServiceBaseProvider } from "@/features/services/lib/service-base";
import type { ServiceView } from "@/features/services/types";
import { prefetchDeployPage } from "../deploy-page-prefetch";
import { Route as ServiceDeployRoute } from "@/routes/services.$serviceId.deploys.$deployId";
import { Route as StaticDeployRoute } from "@/routes/static.$serviceId.deploys.$deployId";
import { Route as ServiceRollbackRoute } from "@/routes/services.$serviceId.deploys.$deployId_.rollback";
import { Route as StaticRollbackRoute } from "@/routes/static.$serviceId.deploys.$deployId_.rollback";

// w1/m659: the deploy page's route loader reads the deploy and what its
// Notices read beyond it before the page mounts, so the page mounts with the
// Notices drawn rather than over a frame that then grows by them. Read
// through a real Apollo client from a fake bex-api that records each read.

vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: (await import("@/test/mocks/router-link")).PlainLink,
}));

type Answers = Record<string, unknown>;

/**
 * A response to the operation's whole selection: every field null unless
 * answers names it by its path from the root ("deploy.status"), so the cache
 * holds a complete result for exactly the query that was asked.
 */
function respond(doc: DocumentNode, answers: Answers): unknown {
  const fragments = new Map<string, FragmentDefinitionNode>();
  for (const def of doc.definitions) {
    if (def.kind === Kind.FRAGMENT_DEFINITION) {
      fragments.set(def.name.value, def);
    }
  }
  const fill = (set: SelectionSetNode, path: string): Answers => {
    const out: Answers = {};
    for (const sel of set.selections) {
      if (sel.kind === Kind.FIELD) {
        const key = sel.alias?.value ?? sel.name.value;
        const at = path ? `${path}.${key}` : key;
        const given = answers[at];
        if (given === undefined || given === null || !sel.selectionSet) {
          out[key] = given ?? null;
        } else if (Array.isArray(given)) {
          out[key] = given.map((item: Answers) => ({
            ...fill(sel.selectionSet!, `${at}[]`),
            ...item,
          }));
        } else {
          out[key] = { ...fill(sel.selectionSet, at), ...(given as Answers) };
        }
      } else {
        const inner =
          sel.kind === Kind.INLINE_FRAGMENT
            ? sel.selectionSet
            : fragments.get(sel.name.value)!.selectionSet;
        Object.assign(out, fill(inner, path));
      }
    }
    return out;
  };
  const op = doc.definitions.find(
    (def) => def.kind === Kind.OPERATION_DEFINITION,
  );
  return op && "selectionSet" in op ? fill(op.selectionSet, "") : null;
}

interface Read {
  name: string;
  variables: Record<string, unknown>;
}

function bexApi(answers: Record<string, Answers | "fail">) {
  const asked: Read[] = [];
  const link = new ApolloLink(
    (op) =>
      new Observable((sub) => {
        const name = op.operationName ?? "";
        asked.push({ name, variables: op.variables });
        const given = answers[name];
        if (given === undefined || given === "fail") {
          sub.error(new Error(`no answer for ${name}`));
          return;
        }
        sub.next({ data: respond(op.query, given) as Answers });
        sub.complete();
      }),
  );
  const client = new ApolloClient({
    link,
    cache: new InMemoryCache(apolloCacheConfig),
  });
  return { client, asked, names: () => asked.map((r) => r.name) };
}

const deployAnswer = (deploy: Answers): Answers => ({
  deploy: {},
  "deploy.id": "dep-2",
  "deploy.__typename": "Deploy",
  ...Object.fromEntries(
    Object.entries(deploy).map(([key, value]) => [`deploy.${key}`, value]),
  ),
});

const SERVER: Answers = {
  server: {},
  "server.__typename": "Server",
  "server.id": "srv-1",
  "server.ownerId": "tea-1",
};

const view = (over: Partial<DeployView>): DeployView => ({
  id: "dep-2",
  status: "live",
  trigger: "manual",
  image: "",
  imageSha: "",
  rollbackOf: "",
  buildAvailable: null,
  commitId: "",
  commitMessage: "",
  commitCreatedAt: null,
  createdAt: "2026-11-26T00:00:00Z",
  updatedAt: "2026-11-26T00:01:00Z",
  startedAt: null,
  finishedAt: null,
  preDeployStatus: "",
  initialDeployHookStatus: "",
  initialDeployHookFinishedAt: null,
  failureReason: "",
  cancelReason: "",
  stallReason: "",
  failureReasonTemplate: null,
  stallReasonTemplate: null,
  cancelReasonTemplate: null,
  ...over,
});

function renderNotices(client: ApolloClient, deploy: DeployView) {
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>
      <ServiceBaseProvider value="/services">{children}</ServiceBaseProvider>
    </ApolloProvider>
  );
  return render(
    <DeployNotices
      serviceId="srv-1"
      deploy={deploy}
      service={{ plan: "starter", healthCheckPath: null } as ServiceView}
      ownerId="tea-1"
    />,
    { wrapper },
  );
}

const noticeTexts = () =>
  within(screen.getByRole("region", { name: "Notices" }))
    .getAllByRole("listitem")
    .map((li) => li.textContent);

describe("prefetchDeployPage", () => {
  it("warms a queued deploy's blocker and policy, so its Notices draw on the first render", async () => {
    const api = bexApi({
      Deploy: deployAnswer({ status: "queued" }),
      Server: SERVER,
      Deploys: {
        deploys: [{ __typename: "Deploy", id: "dep-1", status: "created" }],
      },
      WorkspaceOverlappingDeployPolicy: {
        workspace: {},
        "workspace.__typename": "Workspace",
        "workspace.id": "tea-1",
        "workspace.overlappingDeployPolicy": "wait",
      },
    });
    await prefetchDeployPage(api.client, "srv-1", "dep-2");
    expect(api.names().sort()).toEqual([
      "Deploy",
      "Deploys",
      "Server",
      "WorkspaceOverlappingDeployPolicy",
    ]);
    expect(api.asked.find((r) => r.name === "Deploys")?.variables).toEqual({
      serviceId: "srv-1",
      status: [
        "created",
        "build_in_progress",
        "pre_deploy_in_progress",
        "update_in_progress",
      ],
      limit: 5,
    });

    // No waiting: the page's own reads find what the loader read.
    renderNotices(api.client, view({ status: "queued" }));
    expect(noticeTexts()).toEqual([
      "Another deploy is in progress" +
        "To avoid queuing deploys, adjust your overlapping deploy policy.",
    ]);
    expect(
      screen.getByRole("link", { name: "Another deploy" }),
    ).toHaveAttribute("href", "/services/srv-1/deploys/dep-1");
    // The policy came from the cache; only the deploys list refreshes, as
    // its read is cache-and-network.
    expect(
      api.names().filter((n) => n === "WorkspaceOverlappingDeployPolicy"),
    ).toHaveLength(1);
  });

  it("warms the policy alone for a deploy a newer one superseded", async () => {
    const api = bexApi({
      Deploy: deployAnswer({
        status: "canceled",
        cancelReason: "Superseded by dep-3",
        cancelReasonTemplate: {
          __typename: "DeployReasonTemplate",
          code: "superseded_by",
          params: [],
          remainder: null,
          parts: [],
        },
      }),
      Server: SERVER,
      WorkspaceOverlappingDeployPolicy: {
        workspace: {},
        "workspace.__typename": "Workspace",
        "workspace.id": "tea-1",
        "workspace.overlappingDeployPolicy": "override",
      },
    });
    await prefetchDeployPage(api.client, "srv-1", "dep-2");
    expect(api.names().sort()).toEqual([
      "Deploy",
      "Server",
      "WorkspaceOverlappingDeployPolicy",
    ]);
    renderNotices(
      api.client,
      view({
        status: "canceled",
        cancelReason: "Superseded by dep-3",
        cancelReasonTemplate: {
          code: "superseded_by",
          params: { deploy: "dep-3" },
          parts: [],
          remainder: "",
        },
      }),
    );
    expect(noticeTexts()).toEqual([
      "Superseded by dep-3" +
        "To avoid canceling active deploys when new ones are triggered, adjust your overlapping deploy policy.",
    ]);
  });

  it("reads nothing more for a deploy whose Notices stand on the deploy alone", async () => {
    const api = bexApi({
      Deploy: deployAnswer({ status: "live", clearCache: true }),
      Server: SERVER,
    });
    await prefetchDeployPage(api.client, "srv-1", "dep-2");
    expect(api.names().sort()).toEqual(["Deploy", "Server"]);
  });

  it("opens a deploy the session already read without asking again", async () => {
    const api = bexApi({
      Deploy: deployAnswer({ status: "live" }),
      Server: SERVER,
    });
    await prefetchDeployPage(api.client, "srv-1", "dep-2");
    await prefetchDeployPage(api.client, "srv-1", "dep-2");
    expect(api.names().sort()).toEqual(["Deploy", "Server"]);
  });

  it("never blocks a navigation on a failed read", async () => {
    const api = bexApi({ Deploy: "fail", Server: SERVER });
    await expect(
      prefetchDeployPage(api.client, "srv-1", "dep-2"),
    ).resolves.toBeUndefined();
    expect(api.names().sort()).toEqual(["Deploy", "Server"]);
  });

  it("skips the policy when the service's workspace is unknown", async () => {
    const api = bexApi({
      Deploy: deployAnswer({ status: "queued" }),
      Server: "fail",
      Deploys: { deploys: [] },
    });
    await prefetchDeployPage(api.client, "srv-1", "dep-2");
    expect(api.names().sort()).toEqual(["Deploy", "Deploys", "Server"]);
  });
});

describe("the deploy page's routes", () => {
  it.each([
    ["/services/…/deploys/$deployId", ServiceDeployRoute],
    ["/static/…/deploys/$deployId", StaticDeployRoute],
    ["/services/…/deploys/$deployId/rollback", ServiceRollbackRoute],
    ["/static/…/deploys/$deployId/rollback", StaticRollbackRoute],
  ])("%s reads the deploy before the page mounts", async (_, route) => {
    const api = bexApi({
      Deploy: deployAnswer({ status: "live" }),
      Server: SERVER,
    });
    // Through unknown: the router's full loader context is not comparable to
    // this two-key one, and the loader reads only these.
    const loader = route.options.loader as unknown as (ctx: {
      context: Pick<RouterContext, "client">;
      params: { serviceId: string; deployId: string };
    }) => Promise<void>;
    await loader({
      context: { client: api.client },
      params: { serviceId: "srv-1", deployId: "dep-2" },
    });
    expect(api.asked.find((r) => r.name === "Deploy")?.variables).toEqual({
      serviceId: "srv-1",
      deployId: "dep-2",
    });
  });
});
