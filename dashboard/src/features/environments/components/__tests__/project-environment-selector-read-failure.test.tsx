import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import { mockWorkspaceContext } from "@/test/mocks/workspace";
import { ProjectEnvironmentSelector } from "../project-environment-selector";
import {
  buildCreateServiceInput,
  type NewServiceForm,
} from "@/features/services/lib/create-service-input";
import { useCreateService } from "@/features/services/hooks/use-create-service";

/**
 * w4/m221 (a856): a failed Projects or Environments read used to look like an
 * authoritative empty list, so the selector cleared a valid contextual
 * placement and the create went out without its environmentId. Real Apollo
 * with the app's defaults and the real list hooks; only the network is faked.
 */
vi.mock("@/features/workspaces/context/hooks", () =>
  mockWorkspaceContext("tea-1"),
);

type Reply = "ok" | "error" | "transport" | "missing" | "partial";

const project = {
  __typename: "Project",
  id: "prj-1",
  name: "Storefront",
  ownerId: "tea-1",
  createdAt: "2026-10-17T00:00:00Z",
  serviceIds: [],
  databaseIds: [],
  keyValueIds: [],
};
const otherProject = { ...project, id: "prj-2", name: "Docs" };
const environment = {
  __typename: "Environment",
  id: "evm-1",
  projectId: "prj-1",
  name: "Production",
  ownerId: "tea-1",
  createdAt: "2026-10-17T00:00:00Z",
  serviceIds: [],
  databaseIds: [],
  keyValueIds: [],
  envGroupIds: [],
  protectedStatus: "unprotected",
  networkIsolationEnabled: false,
  ipAllowList: [],
  ipAllowListEntries: [],
  ipAllowListProxiedDomains: [],
};

function fakeClient(script: Record<"Projects" | "Environments", Reply[]>) {
  const reads = { Projects: 0, Environments: 0, other: 0 };
  const creates: Array<Record<string, unknown>> = [];
  const link = new ApolloLink(
    (operation) =>
      new Observable((sub) => {
        if (operation.operationName === "CreateService") {
          // Record the wire variables; the outcome is not under test.
          creates.push(operation.variables);
          sub.next({ data: null, errors: [{ message: "not under test" }] });
          sub.complete();
          return;
        }
        const name = operation.operationName as "Projects" | "Environments";
        if (!(name in script)) {
          reads.other += 1;
          sub.error(new Error(`unexpected ${operation.operationName}`));
          return;
        }
        const replies = script[name];
        const reply = replies[Math.min(reads[name], replies.length - 1)];
        reads[name] += 1;
        setTimeout(() => {
          if (reply === "transport") {
            sub.error(new Error("network down"));
            return;
          }
          if (reply === "error") {
            sub.next({ data: null, errors: [{ message: "lookup failed" }] });
          } else if (reply === "partial") {
            // An empty list that arrives WITH an error is not authoritative.
            sub.next({
              data:
                name === "Projects" ? { projects: [] } : { environments: [] },
              errors: [{ message: "partial failure" }],
            });
          } else if (name === "Projects") {
            sub.next({
              data: { projects: reply === "ok" ? [project, otherProject] : [] },
            });
          } else {
            sub.next({
              data: { environments: reply === "ok" ? [environment] : [] },
            });
          }
          sub.complete();
        }, 0);
      }),
  );
  return {
    client: new ApolloClient({
      link,
      cache: new InMemoryCache(apolloCacheConfig),
      defaultOptions: apolloDefaultOptions,
    }),
    reads,
    creates,
  };
}

// The parent's own state, as the three create forms keep it, plus an
// unrelated draft that must survive the failure and the retry.
function Form({
  initialEnvironmentId = "evm-1",
}: {
  initialEnvironmentId?: string | null;
}) {
  const [projectId, setProjectId] = useState<string | null>("prj-1");
  const [environmentId, setEnvironmentId] = useState<string | null>(
    initialEnvironmentId,
  );
  const [name, setName] = useState("");
  return (
    <>
      <input
        aria-label="Name"
        value={name}
        onChange={(e) => setName(e.target.value)}
      />
      <ProjectEnvironmentSelector
        projectId={projectId}
        environmentId={environmentId}
        onProjectChange={setProjectId}
        onEnvironmentChange={setEnvironmentId}
      />
      <output data-testid="placement">
        {projectId ?? "none"}/{environmentId ?? "none"}
      </output>
    </>
  );
}

function renderForm(
  script: Record<"Projects" | "Environments", Reply[]>,
  form = <Form />,
) {
  const fake = fakeClient(script);
  render(<ApolloProvider client={fake.client}>{form}</ApolloProvider>);
  return fake;
}

// A web service draft as the create form holds it; only placement varies.
const DRAFT: NewServiceForm = {
  name: "web",
  serviceType: "web_service",
  tab: "git",
  selectedRepo: null,
  gitUrl: "https://github.com/bex-co/bex",
  image: "",
  imageCommand: "",
  registryCredentialId: "",
  branch: "main",
  rootDir: "",
  runtime: "node",
  buildCommand: "npm ci",
  startCommand: "npm start",
  dockerfilePath: "./Dockerfile",
  staticBuildCommand: "",
  publishPath: "",
  buildFilterPaths: [],
  buildFilterIgnored: [],
  plan: "free",
  port: "3000",
  autoDeploy: true,
  schedule: "",
  command: "",
  envVars: [],
  secretFiles: [],
  projectId: null,
  environmentId: null,
};

// The real create chain: selector state → buildCreateServiceInput →
// useCreateService → the CreateService operation on the wire.
function CreateForm() {
  const [projectId, setProjectId] = useState<string | null>("prj-1");
  const [environmentId, setEnvironmentId] = useState<string | null>("evm-1");
  const { create } = useCreateService();
  return (
    <>
      <ProjectEnvironmentSelector
        projectId={projectId}
        environmentId={environmentId}
        onProjectChange={setProjectId}
        onEnvironmentChange={setEnvironmentId}
      />
      <button
        type="button"
        onClick={() =>
          void create(
            buildCreateServiceInput({ ...DRAFT, projectId, environmentId }),
          )
        }
      >
        Deploy
      </button>
    </>
  );
}

const placement = () => screen.getByTestId("placement");
const projectSelect = () => screen.getByRole("combobox", { name: "Project" });
const environmentSelect = () =>
  screen.getByRole("combobox", { name: "Environment" });

describe("ProjectEnvironmentSelector read failures (w4/m221)", () => {
  it.each(["error", "transport"] as const)(
    "keeps the Environment through a failed Environments read (%s) and recovers it with Retry",
    async (failure) => {
      const user = userEvent.setup();
      const fake = renderForm({
        Projects: ["ok"],
        Environments: [failure, "ok"],
      });
      await user.type(screen.getByRole("textbox", { name: "Name" }), "web");

      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent(
        /couldn.t load projects or environments/i,
      );
      expect(placement()).toHaveTextContent("prj-1/evm-1");
      expect(projectSelect()).toHaveTextContent("Storefront");
      expect(environmentSelect()).toHaveTextContent(
        "Selected (name unavailable)",
      );
      expect(environmentSelect()).not.toHaveTextContent("No environment");

      await user.click(screen.getByRole("button", { name: "Try again" }));
      await waitFor(() =>
        expect(environmentSelect()).toHaveTextContent("Production"),
      );
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
      expect(placement()).toHaveTextContent("prj-1/evm-1");
      expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("web");
      // Retry re-read only the failed list; nothing else was sent.
      expect(fake.reads).toEqual({ Projects: 1, Environments: 2, other: 0 });
    },
  );

  it.each(["error", "transport"] as const)(
    "keeps both ids through a failed Projects read (%s) and recovers them",
    async (failure) => {
      const user = userEvent.setup();
      const fake = renderForm({
        Projects: [failure, "ok"],
        Environments: ["ok"],
      });

      await screen.findByRole("alert");
      expect(placement()).toHaveTextContent("prj-1/evm-1");
      expect(projectSelect()).toHaveTextContent("Selected (name unavailable)");
      expect(projectSelect()).not.toHaveTextContent("No project");

      await user.click(screen.getByRole("button", { name: "Try again" }));
      await waitFor(() =>
        expect(projectSelect()).toHaveTextContent("Storefront"),
      );
      expect(environmentSelect()).toHaveTextContent("Production");
      expect(placement()).toHaveTextContent("prj-1/evm-1");
      expect(fake.reads.Projects).toBe(2);
      expect(fake.reads.other).toBe(0);
    },
  );

  it("keeps the Environment when an empty list arrives with an error", async () => {
    renderForm({ Projects: ["ok"], Environments: ["partial"] });
    await screen.findByRole("alert");
    expect(placement()).toHaveTextContent("prj-1/evm-1");
    expect(environmentSelect()).toHaveTextContent(
      "Selected (name unavailable)",
    );
  });

  it("shows a kept selection as loading, not as none, before its list arrives", () => {
    renderForm({ Projects: ["ok"], Environments: ["ok"] });
    expect(projectSelect()).toHaveTextContent("Loading…");
    expect(projectSelect()).not.toHaveTextContent("No project");
    expect(placement()).toHaveTextContent("prj-1/evm-1");
  });

  it("keeps the selection when a warm list's refresh fails, then recovers", async () => {
    const user = userEvent.setup();
    const fake = renderForm({
      Projects: ["ok"],
      Environments: ["ok", "error", "ok"],
    });
    await waitFor(() =>
      expect(environmentSelect()).toHaveTextContent("Production"),
    );
    await act(async () => {
      await fake.client.refetchQueries({ include: "active" }).catch(() => []);
    });
    await screen.findByRole("alert");
    expect(placement()).toHaveTextContent("prj-1/evm-1");

    await user.click(screen.getByRole("button", { name: "Try again" }));
    await waitFor(() =>
      expect(environmentSelect()).toHaveTextContent("Production"),
    );
    expect(placement()).toHaveTextContent("prj-1/evm-1");
  });

  it("keeps a deliberate No environment through a failed read", async () => {
    renderForm(
      { Projects: ["ok"], Environments: ["error"] },
      <Form initialEnvironmentId={null} />,
    );
    await waitFor(() =>
      expect(projectSelect()).toHaveTextContent("Storefront"),
    );
    expect(placement()).toHaveTextContent("prj-1/none");
    expect(environmentSelect()).toHaveTextContent("No environment");
  });

  it("still clears the Environment when the customer changes Project during a failure", async () => {
    const user = userEvent.setup();
    renderForm({ Projects: ["ok"], Environments: ["error"] });
    await screen.findByRole("alert");
    await waitFor(() =>
      expect(projectSelect()).toHaveTextContent("Storefront"),
    );
    await user.click(projectSelect());
    await user.click(await screen.findByRole("option", { name: "Docs" }));
    expect(placement()).toHaveTextContent("prj-2/none");
  });

  it("sends the intended environmentId in CreateService after recovering a failed read", async () => {
    const user = userEvent.setup();
    const fake = renderForm(
      { Projects: ["ok"], Environments: ["error", "ok"] },
      <CreateForm />,
    );
    await screen.findByRole("alert");
    await user.click(screen.getByRole("button", { name: "Try again" }));
    await waitFor(() =>
      expect(environmentSelect()).toHaveTextContent("Production"),
    );
    await user.click(screen.getByRole("button", { name: "Deploy" }));
    await waitFor(() => expect(fake.creates).toHaveLength(1));
    expect(fake.creates[0]).toMatchObject({
      environmentId: "evm-1",
      ownerId: "tea-1",
    });
    // Retry re-read metadata only; the one create came from Deploy.
    expect(fake.reads.Environments).toBe(2);
  });

  it("still clears a selection that an error-free list does not contain", async () => {
    renderForm({ Projects: ["ok"], Environments: ["missing"] });
    await waitFor(() => expect(placement()).toHaveTextContent("prj-1/none"));
    expect(environmentSelect()).toHaveTextContent("No environment");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("still clears both ids when an error-free Projects list lacks the Project", async () => {
    renderForm({ Projects: ["missing"], Environments: ["ok"] });
    await waitFor(() => expect(placement()).toHaveTextContent("none/none"));
    expect(projectSelect()).toHaveTextContent("No project");
  });
});
