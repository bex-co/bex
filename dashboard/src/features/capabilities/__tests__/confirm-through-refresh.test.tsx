// A routine permission refresh must not dismiss an open confirmation, and an
// open confirmation must never dispatch on stale, unavailable or obsolete
// access (w4/m159). These tests drive the REAL CapabilitiesProvider, resource
// projections and bound-confirm hook through the captured live race: the 30s
// receipt expires while the next (unchanged, successful) poll is still held.

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import type { ReactElement } from "react";

vi.unmock("@/features/capabilities/hooks/use-capabilities");

type QueryName = "caps" | "server" | "deploy";
type Held = { name: QueryName; resolve: () => void; fail: () => void };

const apolloQuery = vi.fn();
// One stable client, as in the app: projections refetch when it changes.
const client = { query: apolloQuery };
const rollbackService = vi.fn();
const cancelDeploy = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => client,
  useMutation: (doc: { definitions?: Array<{ name?: { value?: string } }> }) =>
    doc.definitions?.[0]?.name?.value === "RollbackService"
      ? [rollbackService, { loading: false }]
      : [cancelDeploy, { loading: false }],
}));

let workspaceId = "tea-first";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: workspaceId }),
}));
vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn(), warning: vi.fn() },
}));
const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({ useNavigate: () => navigate }));
const restart = vi.fn();
const trigger = vi.fn();
vi.mock("@/features/services/hooks/use-trigger-deploy", () => ({
  useTriggerDeploy: () => ({ deploying: false, trigger, restart }),
}));
const setAutoDeploy = vi.fn();
vi.mock("@/features/services/hooks/use-auto-deploy", () => ({
  useAutoDeploy: () => ({ setAutoDeploy, busy: false }),
}));
vi.mock("@/features/projects/components/move-to-project-menu", () => ({
  MoveToProjectMenu: () => null,
}));

import {
  DeployActionsDocument,
  ServerActionsDocument,
  ViewerCapabilitiesDocument,
} from "@/graphql/definitions";
import { CapabilitiesProvider } from "@/features/capabilities/context/capabilities-provider";
import {
  bumpAccessGeneration,
  resetAccessGenerationForTests,
} from "@/features/capabilities/lib/access-generation";
import { CAPABILITY_FRESHNESS_MS } from "@/features/capabilities/lib/capability-policy";
import { SuspendServiceCard } from "@/features/services/components/suspend-service-card";
import { ManualDeployButton } from "@/features/services/components/manual-deploy-button";
import { ServiceRowActions } from "@/features/services/components/service-row-actions";
import { DeployActions } from "@/features/deploys/components/deploy-actions";
import type { ServiceView } from "@/features/services/types";

const CHECKING = "Checking whether you can perform this action…";
const UNAVAILABLE =
  "Permissions could not be refreshed. Try again — this is not a role change.";

const CAPABILITIES = [
  "can_view",
  "can_view_logs",
  "can_operate",
  "can_create",
  "can_view_sensitive",
  "can_manage_keys",
  "can_manage",
  "can_manage_billing",
];

let operateOutcome: "allowed" | "denied";
let holdPolls: boolean;
let holdChecks: boolean;
let held: Held[];

function decisions(actions: string[], precondition: Record<string, string>) {
  return actions.map((action) => ({
    __typename: "ActionDecision",
    action,
    outcome: "allowed",
    reason: null,
    precondition: precondition[action] ?? null,
  }));
}

function answer(name: QueryName) {
  if (name === "caps")
    return {
      data: {
        viewerCapabilities: {
          role: "ADMIN",
          grants: CAPABILITIES.map((action) => ({
            action,
            outcome: action === "can_operate" ? operateOutcome : "allowed",
            reason: null,
          })),
        },
      },
    };
  if (name === "server")
    return {
      data: {
        serverActions: decisions(["suspend", "resume", "restart"], {}),
      },
    };
  // The service-wide summary says no rollback target; the selected
  // deactivated row must still be one (w6/m143, w4/m141).
  return {
    data: {
      deployActions: decisions(["deploy", "cancel_deploy", "rollback"], {
        rollback: "no_eligible_rollback_target",
      }),
    },
  };
}

beforeEach(() => {
  vi.useFakeTimers({
    toFake: [
      "setTimeout",
      "clearTimeout",
      "setInterval",
      "clearInterval",
      "Date",
    ],
  });
  vi.setSystemTime(1_000_000);
  Object.defineProperty(document, "hidden", {
    configurable: true,
    get: () => false,
  });
  workspaceId = "tea-first";
  resetAccessGenerationForTests();
  operateOutcome = "allowed";
  holdPolls = false;
  holdChecks = false;
  held = [];
  for (const mock of [
    navigate,
    restart,
    trigger,
    setAutoDeploy,
    rollbackService,
    cancelDeploy,
  ])
    mock.mockReset();
  restart.mockResolvedValue("dep-restart");
  trigger.mockResolvedValue("dep-commit");
  setAutoDeploy.mockResolvedValue(true);
  rollbackService.mockResolvedValue({
    data: { rollbackService: { id: "dep-rb" } },
  });
  apolloQuery
    .mockReset()
    .mockImplementation(
      (opts: {
        query: unknown;
        variables: { fresh?: boolean };
        context?: { fetchOptions?: { signal?: AbortSignal } };
      }) => {
        const name: QueryName =
          opts.query === ViewerCapabilitiesDocument
            ? "caps"
            : opts.query === ServerActionsDocument
              ? "server"
              : opts.query === DeployActionsDocument
                ? "deploy"
                : (() => {
                    throw new Error("unexpected query");
                  })();
        return new Promise((resolve, reject) => {
          const fail = () => reject(new Error("aborted or unavailable"));
          opts.context?.fetchOptions?.signal?.addEventListener("abort", fail);
          const call: Held = {
            name,
            resolve: () => resolve(answer(name)),
            fail,
          };
          // A routine poll asks fresh=false; a dispatch recheck of server/deploy
          // actions is the click-time check a lapse must invalidate.
          const hold =
            (name === "caps" && holdPolls && opts.variables.fresh === false) ||
            (name !== "caps" && holdChecks);
          if (hold) held.push(call);
          else queueMicrotask(call.resolve);
        });
      },
    );
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

async function flush() {
  for (let i = 0; i < 5; i++) {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
  }
}

/** The captured race: the receipt expires before the next poll answers. */
async function expireWithPollHeld() {
  holdPolls = true;
  await act(async () => {
    await vi.advanceTimersByTimeAsync(CAPABILITY_FRESHNESS_MS);
  });
  await flush();
  expect(held.some((call) => call.name === "caps")).toBe(true);
}

async function settleHeld(kind: "resolve" | "fail", name: QueryName = "caps") {
  holdPolls = false;
  const calls = held.filter((call) => call.name === name);
  held = held.filter((call) => call.name !== name);
  await act(async () => {
    for (const call of calls) call[kind]();
  });
  await flush();
}

function renderWithProvider(ui: ReactElement) {
  return render(<CapabilitiesProvider>{ui}</CapabilitiesProvider>);
}

/** Open a Radix dropdown by keyboard and pick an item (fake-timer safe). */
async function pickMenuItem(trigger: RegExp | string, item: string) {
  fireEvent.keyDown(screen.getByRole("button", { name: trigger }), {
    key: "Enter",
  });
  await flush();
  fireEvent.click(screen.getByRole("menuitem", { name: item }));
  await flush();
}

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "srv-first",
    name: "web",
    slug: null,
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: "https://web.onbex.co",
    internalAddress: null,
    createdAt: null,
    sshAddress: null,
    replicas: 1,
    revision: "r1",
    plan: "starter",
    idleTTLSeconds: 0,
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
    registryCredentialId: null,
    buildFilter: null,
    autoDeploy: null,
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

type Consumer = {
  name: string;
  confirmLabel: string;
  render: () => { dispatched: () => number };
  open: () => Promise<void>;
};

const onRun = vi.fn();

const consumers: Consumer[] = [
  {
    name: "Settings → Suspend (SuspendServiceCard)",
    confirmLabel: "Suspend",
    render: () => {
      onRun.mockReset().mockResolvedValue({ status: "success" });
      renderWithProvider(
        <SuspendServiceCard service={svc()} pending={null} onRun={onRun} />,
      );
      return { dispatched: () => onRun.mock.calls.length };
    },
    open: async () => {
      fireEvent.click(screen.getByRole("button", { name: "Suspend" }));
    },
  },
  {
    name: "Manual Deploy → Restart (ManualDeployButton)",
    confirmLabel: "Restart",
    render: () => {
      renderWithProvider(
        <ManualDeployButton service={svc()} pending={false} />,
      );
      return { dispatched: () => restart.mock.calls.length };
    },
    open: () => pickMenuItem(/Manual Deploy/, "Restart service"),
  },
  {
    name: "row menu → Restart (ServiceRowActions)",
    confirmLabel: "Restart",
    render: () => {
      onRun.mockReset().mockResolvedValue({ status: "success" });
      renderWithProvider(
        <ServiceRowActions service={svc()} pending={null} onRun={onRun} />,
      );
      return { dispatched: () => onRun.mock.calls.length };
    },
    open: () => pickMenuItem("Open actions menu", "Restart"),
  },
  {
    name: "deploy row → Rollback (DeployActions, selected-row adjustment)",
    confirmLabel: "Proceed",
    render: () => {
      renderWithProvider(
        <DeployActions
          serviceId="srv-first"
          deployId="dep-old"
          status="deactivated"
        />,
      );
      return { dispatched: () => rollbackService.mock.calls.length };
    },
    open: async () => {
      fireEvent.click(
        screen.getByRole("button", { name: "Roll back to dep-old" }),
      );
    },
  },
];

function dialog() {
  return screen.getByRole("alertdialog");
}

function confirmButton(label: string) {
  return within(dialog()).getByRole("button", { name: label });
}

describe.each(consumers)("$name", (consumer) => {
  async function openReady() {
    const handle = consumer.render();
    await flush();
    await consumer.open();
    await flush();
    expect(confirmButton(consumer.confirmLabel)).toBeEnabled();
    return handle;
  }

  it("stays open through an expiry-before-response refresh and needs a new click", async () => {
    const { dispatched } = await openReady();

    await expireWithPollHeld();
    // Still the same dialog, visibly blocked, Cancel still usable.
    expect(confirmButton(consumer.confirmLabel)).toBeDisabled();
    expect(within(dialog()).getByRole("status")).toHaveTextContent(CHECKING);
    expect(
      within(dialog())
        .getAllByRole("button")
        .find((button) => button !== confirmButton(consumer.confirmLabel)),
    ).toBeEnabled();
    fireEvent.click(confirmButton(consumer.confirmLabel));
    await flush();
    expect(dispatched()).toBe(0);

    // The unchanged successful answer restores the same dialog, without
    // replaying anything.
    await settleHeld("resolve");
    expect(confirmButton(consumer.confirmLabel)).toBeEnabled();
    expect(within(dialog()).getByRole("status")).toBeEmptyDOMElement();
    expect(dispatched()).toBe(0);

    fireEvent.click(confirmButton(consumer.confirmLabel));
    await flush();
    expect(dispatched()).toBe(1);
  });

  it("keeps a failed refresh visible but disabled with retry guidance", async () => {
    const { dispatched } = await openReady();
    await expireWithPollHeld();
    await settleHeld("fail");
    expect(confirmButton(consumer.confirmLabel)).toBeDisabled();
    expect(within(dialog()).getByRole("status")).toHaveTextContent(UNAVAILABLE);
    expect(dispatched()).toBe(0);
  });

  it("closes on a confirmed downgrade and a late allow cannot reopen it", async () => {
    const { dispatched } = await openReady();
    await expireWithPollHeld();
    operateOutcome = "denied";
    await settleHeld("resolve");
    expect(screen.queryByRole("alertdialog")).toBeNull();
    operateOutcome = "allowed";
    await act(async () => {
      await vi.advanceTimersByTimeAsync(CAPABILITY_FRESHNESS_MS);
    });
    await flush();
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(dispatched()).toBe(0);
  });

  it("drops an in-flight check that straddles a lapse, then accepts a new click", async () => {
    const { dispatched } = await openReady();
    holdChecks = true;
    fireEvent.click(confirmButton(consumer.confirmLabel));
    await flush();
    const check = held.find((call) => call.name !== "caps");
    expect(check).toBeDefined();

    await expireWithPollHeld();
    holdChecks = false;
    await settleHeld("resolve");
    // The obsolete check answers "allowed" after recovery: still no dispatch.
    await act(async () => check!.resolve());
    await flush();
    expect(dispatched()).toBe(0);
    expect(confirmButton(consumer.confirmLabel)).toBeEnabled();

    fireEvent.click(confirmButton(consumer.confirmLabel));
    await flush();
    expect(dispatched()).toBe(1);
  });

  it.each(["workspace", "generation"] as const)(
    "a %s change during the gap invalidates the intent for good",
    async (change) => {
      const { dispatched } = await openReady();
      await expireWithPollHeld();
      await act(async () => {
        if (change === "workspace") workspaceId = "tea-second";
        bumpAccessGeneration();
      });
      await flush();
      expect(screen.queryByRole("alertdialog")).toBeNull();
      await settleHeld("resolve");
      await flush();
      expect(screen.queryByRole("alertdialog")).toBeNull();
      expect(dispatched()).toBe(0);
    },
  );
});

describe("drafts survive a routine refresh", () => {
  it("keeps a typed specific-commit SHA and still runs the fresh can_create check", async () => {
    renderWithProvider(
      <ManualDeployButton
        service={svc({
          repo: "https://github.com/bex-co/bex",
          autoDeploy: false,
        })}
        pending={false}
      />,
    );
    await flush();
    await pickMenuItem(/Manual Deploy/, "Deploy a specific commit");
    fireEvent.change(screen.getByLabelText("Commit SHA"), {
      target: { value: "abc1234" },
    });
    expect(confirmButton("Deploy commit")).toBeEnabled();

    await expireWithPollHeld();
    expect(screen.getByLabelText("Commit SHA")).toHaveValue("abc1234");
    expect(confirmButton("Deploy commit")).toBeDisabled();

    await settleHeld("resolve");
    expect(screen.getByLabelText("Commit SHA")).toHaveValue("abc1234");
    expect(trigger).not.toHaveBeenCalled();
    const before = apolloQuery.mock.calls.length;
    fireEvent.click(confirmButton("Deploy commit"));
    await flush();
    expect(trigger).toHaveBeenCalledWith("srv-first", { commitId: "abc1234" });
    expect(
      apolloQuery.mock.calls
        .slice(before)
        .some(
          ([opts]) =>
            opts.query === ViewerCapabilitiesDocument && opts.variables.fresh,
        ),
    ).toBe(true);
  });

  it("keeps a typed server-issued protected phrase blocked until access is fresh", async () => {
    onRun
      .mockReset()
      .mockResolvedValueOnce({
        status: "confirmation_required",
        confirmation: "sudo suspend web",
      })
      .mockResolvedValue({ status: "success" });
    renderWithProvider(
      <SuspendServiceCard service={svc()} pending={null} onRun={onRun} />,
    );
    await flush();
    fireEvent.click(screen.getByRole("button", { name: "Suspend" }));
    await flush();
    fireEvent.click(confirmButton("Suspend"));
    await flush();
    const protectedDialog = screen.getByRole("dialog");
    const field = within(protectedDialog).getByRole("textbox");
    fireEvent.change(field, { target: { value: "sudo suspend web" } });
    const action = within(protectedDialog).getByRole("button", {
      name: "Suspend",
    });
    expect(action).toBeEnabled();

    await expireWithPollHeld();
    expect(within(screen.getByRole("dialog")).getByRole("textbox")).toHaveValue(
      "sudo suspend web",
    );
    expect(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: "Suspend",
      }),
    ).toBeDisabled();
    expect(onRun).toHaveBeenCalledTimes(1);

    await settleHeld("resolve");
    fireEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: "Suspend",
      }),
    );
    await flush();
    expect(onRun).toHaveBeenCalledTimes(2);
    expect(onRun).toHaveBeenLastCalledWith(
      "suspend",
      expect.objectContaining({ id: "srv-first" }),
      "sudo suspend web",
    );
  });
});
