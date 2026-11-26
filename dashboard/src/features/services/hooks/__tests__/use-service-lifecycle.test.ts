import { describe, it, expect, vi, beforeEach } from "vitest";
import { isValidElement, type ReactNode } from "react";
import { renderHook, act, render } from "@testing-library/react";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import type { ServiceView } from "@/features/services/types";
import { codedGraphQLError } from "@/test/mocks/apollo";

// --- mocks ---------------------------------------------------------------

const mutateCalls: { op: string; id: string; confirm?: string }[] = [];
let rejectNext: Error | null = null;
// What the next mutation answers, e.g. the deploy a resume opened.
let nextData: Record<string, unknown> = {};

const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({ useNavigate: () => navigate }));

vi.mock("@apollo/client/react", () => ({
  // Each useMutation(doc) returns a fn tagged with the operation name so the
  // test can assert which Render-named mutation fired.
  // `id` field in mutateCalls holds whichever of variables.id or
  // variables.serviceId is present (suspend/resume use `id`; triggerDeploy
  // uses `serviceId` after the w2/m30 restart consolidation).
  useMutation: (doc: { definitions: { name: { value: string } }[] }) => {
    const op = doc.definitions[0].name.value;
    const fn = vi.fn(
      async ({
        variables,
      }: {
        variables: { id?: string; serviceId?: string; confirm?: string };
      }) => {
        mutateCalls.push({
          op,
          id: variables.id ?? variables.serviceId ?? "",
          confirm: variables.confirm,
        });
        if (rejectNext) {
          const err = rejectNext;
          rejectNext = null;
          throw err;
        }
        const data = nextData;
        nextData = {};
        return { data };
      },
    );
    return [fn, {}];
  },
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (m: ReactNode) => toastSuccess(m),
    error: (m: ReactNode) => toastError(m),
  },
}));

/** A toast's text, and the words it sets in bold. */
function toastText(message: ReactNode) {
  if (!isValidElement(message)) return { text: String(message), bold: [] };
  const { container } = render(message);
  return {
    text: container.textContent,
    bold: [...container.querySelectorAll("strong")].map((b) => b.textContent),
  };
}

// --- helpers -------------------------------------------------------------

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "app",
    name: "app",
    slug: null,
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: null,
    internalAddress: null,
    createdAt: null,
    sshAddress: null,
    replicas: 1,
    revision: "r1",
    plan: null,
    idleTTLSeconds: null,
    schedule: null,
    command: null,
    runs: [],
    repo: null,
    branch: null,
    rootDir: null,
    runtime: null,
    builder: null,
    buildCommand: null,
    startCommand: null,
    dockerfilePath: null,
    dockerContext: null,
    registryCredentialId: null,
    buildFilter: null,
    autoDeploy: null,
    autoDeployTrigger: null,
    notifyOnFail: null,
    notificationsToSend: null,
    healthCheckPath: null,
    maxShutdownDelaySeconds: null,
    preDeployCommand: null,
    renderSubdomainPolicy: null,
    publishPath: null,
    routes: [],
    headers: [],
    ipAllowList: null,
    ipAllowListEntries: null,
    maintenanceMode: null,
    outboundIps: null,
    ...overrides,
  };
}

beforeEach(() => {
  mutateCalls.length = 0;
  rejectNext = null;
  nextData = {};
  navigate.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useServiceLifecycle", () => {
  it("fires the matching Render mutation for each verb", async () => {
    const refetch = vi.fn(async () => [
      svc({ suspended: true, phase: "Hibernated" }),
    ]);
    const { result } = renderHook(() =>
      useServiceLifecycle({ refetch, pollIntervalMs: 0, maxPolls: 1 }),
    );

    await act(async () => {
      await result.current.run("suspend", svc());
    });
    await act(async () => {
      await result.current.run("resume", svc({ suspended: true }));
    });
    await act(async () => {
      await result.current.run("restart", svc());
    });

    expect(mutateCalls.map((c) => c.op)).toEqual([
      "SuspendService",
      "ResumeService",
      // restartServer keeps the running commit; a bare triggerDeploy would
      // build the branch head (w1/m148).
      "RestartServer",
    ]);
    expect(mutateCalls.every((c) => c.id === "app")).toBe(true);
  });

  it("polls the list until the row converges, then stops", async () => {
    // First poll still shows the old state, second shows converged.
    const refetch = vi
      .fn()
      .mockResolvedValueOnce([svc({ suspended: false })])
      .mockResolvedValueOnce([svc({ suspended: true, phase: "Hibernated" })])
      .mockResolvedValue([svc({ suspended: true, phase: "Hibernated" })]);

    const { result } = renderHook(() =>
      useServiceLifecycle({ refetch, pollIntervalMs: 0, maxPolls: 5 }),
    );

    await act(async () => {
      await result.current.run("suspend", svc());
    });

    // stopped as soon as it saw suspended=true (2 polls), not all 5
    expect(refetch).toHaveBeenCalledTimes(2);
    expect(toastSuccess).toHaveBeenCalledOnce();
  });

  it("clears pending and toasts an error when the mutation fails", async () => {
    rejectNext = new Error("boom");
    const refetch = vi.fn(async () => [svc()]);
    const { result } = renderHook(() =>
      useServiceLifecycle({ refetch, pollIntervalMs: 0, maxPolls: 1 }),
    );

    await act(async () => {
      await result.current.run("suspend", svc());
    });

    expect(toastError).toHaveBeenCalledOnce();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(result.current.pending).toBeNull();
    // a failed mutation does not poll for convergence
    expect(refetch).not.toHaveBeenCalled();
  });

  it("returns the exact protected phrase and sends it on the retry", async () => {
    rejectNext = codedGraphQLError(
      "PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED",
      { confirm: "sudo suspend service app" },
    );
    const refetch = vi.fn(async () => [
      svc({ suspended: true, phase: "Hibernated" }),
    ]);
    const { result } = renderHook(() =>
      useServiceLifecycle({ refetch, pollIntervalMs: 0, maxPolls: 1 }),
    );

    let first;
    await act(async () => {
      first = await result.current.run("suspend", svc());
    });

    expect(first).toEqual({
      status: "confirmation_required",
      confirmation: "sudo suspend service app",
    });
    expect(toastError).not.toHaveBeenCalled();
    expect(refetch).not.toHaveBeenCalled();

    await act(async () => {
      await result.current.run("suspend", svc(), "sudo suspend service app");
    });

    expect(mutateCalls).toEqual([
      { op: "SuspendService", id: "app", confirm: undefined },
      {
        op: "SuspendService",
        id: "app",
        confirm: "sudo suspend service app",
      },
    ]);
    expect(toastSuccess).toHaveBeenCalledOnce();
    expect(refetch).toHaveBeenCalledOnce();
  });

  // w3/m643: Render's Resume service toasts "<name> has been resumed." and
  // opens the deploy (a cron job's build) its resumeService answers.
  it("opens the deploy a resume answers, with Render's resume toast", async () => {
    nextData = { resumeService: { id: "app", latestDeployId: "dep-resumed" } };
    const refetch = vi.fn(async () => [svc()]);
    const { result } = renderHook(() =>
      useServiceLifecycle({ refetch, pollIntervalMs: 0, maxPolls: 1 }),
    );

    await act(async () => {
      await result.current.run("resume", svc({ suspended: true }));
    });

    expect(navigate).toHaveBeenCalledWith({
      to: "/services/$serviceId/deploys/$deployId",
      params: { serviceId: "app", deployId: "dep-resumed" },
    });
    expect(toastSuccess).toHaveBeenCalledOnce();
    expect(toastText(toastSuccess.mock.calls[0][0])).toEqual({
      text: "app has been resumed.",
      bold: ["app"],
    });
  });

  it("opens a static site's deploy under its own base", async () => {
    nextData = { resumeService: { id: "app", latestDeployId: "dep-site" } };
    const refetch = vi.fn(async () => [svc({ type: "static_site" })]);
    const { result } = renderHook(() =>
      useServiceLifecycle({ refetch, pollIntervalMs: 0, maxPolls: 1 }),
    );

    await act(async () => {
      await result.current.run(
        "resume",
        svc({ type: "static_site", suspended: true }),
      );
    });

    expect(navigate).toHaveBeenCalledWith({
      to: "/static/$serviceId/deploys/$deployId",
      params: { serviceId: "app", deployId: "dep-site" },
    });
  });

  it("stays on the page when the resume opened no deploy", async () => {
    nextData = { resumeService: { id: "app", latestDeployId: null } };
    const refetch = vi.fn(async () => [svc()]);
    const { result } = renderHook(() =>
      useServiceLifecycle({ refetch, pollIntervalMs: 0, maxPolls: 1 }),
    );

    await act(async () => {
      await result.current.run("resume", svc({ suspended: true }));
    });

    expect(navigate).not.toHaveBeenCalled();
    expect(toastText(toastSuccess.mock.calls[0][0]).text).toBe(
      "app has been resumed.",
    );
  });

  it("toasts Render's resume failure, or the server's own reason", async () => {
    rejectNext = new Error("boom");
    const refetch = vi.fn(async () => [svc()]);
    const { result } = renderHook(() =>
      useServiceLifecycle({ refetch, pollIntervalMs: 0, maxPolls: 1 }),
    );

    await act(async () => {
      await result.current.run("resume", svc({ suspended: true }));
    });
    expect(toastText(toastError.mock.calls[0][0])).toEqual({
      text: "An error occurred while resuming app. Please try again.",
      bold: ["app"],
    });
    expect(navigate).not.toHaveBeenCalled();

    rejectNext = codedGraphQLError(
      "FREE_HOURS_EXHAUSTED",
      {},
      "this workspace has used its monthly Free instance hours",
    );
    await act(async () => {
      await result.current.run("resume", svc({ suspended: true }));
    });
    // The server's refusal, capitalized as a sentence.
    expect(toastError.mock.calls[1][0]).toBe(
      "This workspace has used its monthly Free instance hours",
    );
  });
});
