import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider, useQuery } from "@apollo/client/react";
import { Observable } from "rxjs";
import { toast } from "sonner";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { ServerDocument } from "@/graphql/definitions";
import { CronDeploySection } from "@/features/services/components/cron-deploy-section";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

/**
 * w4/276: a Server read that started before an UpdateCronJob can finish
 * after it and write the old schedule/command back into the cache; the next
 * sibling-field save then replays that stale value and persists the
 * rollback. Real Apollo cache and the real section + hook; only the network
 * is faked, as a server whose Server responses carry the state at request
 * time and can be held — the pre-save snapshot arriving late.
 */
interface FakeServer {
  schedule: string;
  command: string;
  runtime: string;
  startCommand: string;
  holdServer: boolean;
  /** Held reads released as transport failures instead of stale data. */
  failHeld: number;
  /** Post-mutation Server reads that fail (the authoritative refresh). */
  failReadsAfterMutation: number;
  mutated: boolean;
  held: Array<{ send: () => void; fail: () => void }>;
  mutations: Array<{ schedule: string; command: string | null }>;
  startCommandWrites: string[];
  serverReads: number;
}

let server: FakeServer;

function newServer(): FakeServer {
  return {
    schedule: "0 0 15 * *",
    command: "sh -c 'printf \"old\\n\"'",
    runtime: "image",
    startCommand: "",
    holdServer: false,
    failHeld: 0,
    failReadsAfterMutation: 0,
    mutated: false,
    held: [],
    mutations: [],
    startCommandWrites: [],
    serverReads: 0,
  };
}

function releaseHeld() {
  server.holdServer = false;
  server.held.splice(0).forEach((entry, i) => {
    if (i < server.failHeld) entry.fail();
    else entry.send();
  });
}

const link = new ApolloLink(
  (op) =>
    new Observable((sub) => {
      const v = op.variables as {
        id?: string;
        schedule?: string;
        command?: string | null;
      };
      const send = (result: Record<string, unknown>) =>
        setTimeout(() => {
          sub.next(result);
          sub.complete();
        }, 0);
      const fail = () =>
        setTimeout(() => sub.error(new Error("503 Service Unavailable")), 0);

      if (op.operationName === "UpdateCronJob") {
        server.mutations.push({
          schedule: v.schedule!,
          command: v.command ?? null,
        });
        server.schedule = v.schedule!;
        if (v.command !== null && v.command !== undefined)
          server.command = v.command;
        server.mutated = true;
        send({
          data: {
            updateCronJob: {
              __typename: "Service",
              id: v.id,
              schedule: server.schedule,
              command: server.command,
            },
          },
        });
        return;
      }
      // A native cron's start-command write also drops the override (w8/m69).
      if (op.operationName === "SetStartCommand") {
        server.startCommandWrites.push(v.command!);
        server.startCommand = v.command!;
        server.command = "";
        server.mutated = true;
        send({
          data: {
            setStartCommand: {
              __typename: "Service",
              id: v.id,
              startCommand: server.startCommand,
              command: server.command,
              phase: "Running",
            },
          },
        });
        return;
      }
      if (op.operationName === "Server") {
        server.serverReads += 1;
        if (server.mutated && server.failReadsAfterMutation > 0) {
          server.failReadsAfterMutation -= 1;
          fail();
          return;
        }
        // Snapshot the state when the read STARTED. Full field set, so
        // Apollo marks watchers settled (not perpetually loading).
        const snapshot = {
          data: {
            server: {
              __typename: "Service",
              id: v.id,
              name: "nightly",
              immutableName: "nightly",
              slug: "nightly",
              displayName: "",
              type: "cron_job",
              suspended: "not_suspended",
              dashboardUrl: "",
              region: "fsn1",
              url: "",
              publicRoutingNotice: "",
              undeployedChanges: false,
              undeployedChangesApplying: false,
              internalAddress: "",
              createdAt: "2026-10-12T00:00:00Z",
              sshAddress: "",
              phase: "Running",
              replicas: 1,
              revision: "rev-1",
              plan: "free",
              idleTTLSeconds: 0,
              maintenanceMode: null,
              repo: "",
              branch: "",
              imagePath: "docker.io/library/busybox:1.37.0",
              rootDir: "",
              runtime: server.runtime,
              builder: "auto",
              buildCommand: "",
              startCommand: server.startCommand,
              dockerfilePath: "",
              registryCredentialId: "",
              buildFilter: null,
              autoDeploy: false,
              environmentId: null,
              linkedEnvGroupIds: [],
              pushDeliveryMethod: "none",
              notifyOnFail: "default",
              notificationsToSend: "default",
              renderSubdomainPolicy: null,
              port: 0,
              healthCheckPath: "",
              maxShutdownDelaySeconds: null,
              preDeployCommand: "",
              schedule: server.schedule,
              command: server.command,
              lastSuccessfulRunAt: "",
              nextRunAt: "2026-10-15T00:00:00Z",
              runs: [],
              publishPath: "",
              routes: [],
              headers: [],
              ipAllowList: [],
              ipAllowListEntries: [],
              ipAllowListProxiedDomains: [],
              outboundIps: {
                __typename: "OutboundIPs",
                type: "shared",
                ips: [],
              },
            },
          },
        };
        if (server.holdServer)
          server.held.push({ send: () => send(snapshot), fail });
        else send(snapshot);
        return;
      }
      sub.error(new Error(`unexpected operation ${op.operationName}`));
    }),
);

/** An extra active Server watcher (the layout/sidebar stand-in). */
function ExtraWatcher() {
  useQuery(ServerDocument, {
    variables: { id: "srv-1" },
    fetchPolicy: "network-only",
    // Its own request, not deduplicated onto the page's identical read.
    context: { queryDeduplication: false },
  });
  return null;
}

function Page({ watchers = 0 }: { watchers?: number }) {
  const { data } = useQuery(ServerDocument, {
    variables: { id: "srv-1" },
    fetchPolicy: "cache-first",
  });
  const s = data?.server;
  return (
    <>
      {Array.from({ length: watchers }, (_, i) => (
        <ExtraWatcher key={i} />
      ))}
      <CronDeploySection
        serviceId="srv-1"
        schedule={s ? (s.schedule ?? "") : null}
        command={s ? (s.command ?? null) : null}
        runtime={s?.runtime ?? null}
        startCommand={s?.startCommand ?? null}
      />
    </>
  );
}

function renderPage(watchers = 0) {
  const client = new ApolloClient({
    link,
    cache: new InMemoryCache(apolloCacheConfig),
  });
  const utils = render(
    <ApolloProvider client={client}>
      <Page watchers={watchers} />
    </ApolloProvider>,
  );
  return {
    ...utils,
    client,
    setWatchers: (n: number) =>
      utils.rerender(
        <ApolloProvider client={client}>
          <Page watchers={n} />
        </ApolloProvider>,
      ),
  };
}

const settle = () => new Promise((r) => setTimeout(r, 60));

async function saveRow(editLabel: string, fieldName: string, value: string) {
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: editLabel }));
  const input = screen.getByRole("textbox", { name: fieldName });
  await user.clear(input);
  await user.type(input, value);
  await user.click(screen.getByRole("button", { name: "Save changes" }));
  return user;
}

beforeEach(() => {
  server = newServer();
  vi.mocked(toast.success).mockReset();
  vi.mocked(toast.error).mockReset();
  vi.mocked(toast.warning).mockReset();
});

describe("cron saves survive a late pre-save Server read (w4/276)", () => {
  it("schedule first: a held stale read cannot undo the save, and a command-only save preserves it", async () => {
    const page = renderPage();
    await screen.findByDisplayValue("0 0 15 * *");

    // Two pre-save reads start now and are held; one will fail on release,
    // the other delivers the stale day-15 snapshot after the save commits.
    server.holdServer = true;
    server.failHeld = 1;
    page.setWatchers(2);
    await waitFor(() => expect(server.held).toHaveLength(2));

    await saveRow("Edit schedule", "Schedule", "0 0 16 * *");
    await waitFor(() => expect(server.mutations).toHaveLength(1));
    await settle();
    // The write committed but publication waits for the held reads.
    expect(toast.success).not.toHaveBeenCalled();

    await act(async () => releaseHeld());

    await waitFor(() => expect(toast.success).toHaveBeenCalledTimes(1));
    await settle();
    expect(screen.getByRole("textbox", { name: "Schedule" })).toHaveValue(
      "0 0 16 * *",
    );

    // Command-only save must carry the accepted schedule, not day 15.
    await saveRow("Edit command", "Command", "sh -c 'printf \"new\\n\"'");
    await waitFor(() => expect(server.mutations).toHaveLength(2));
    expect(server.mutations[1]).toEqual({
      schedule: "0 0 16 * *",
      command: "sh -c 'printf \"new\\n\"'",
    });
    expect(server.schedule).toBe("0 0 16 * *");
    expect(toast.warning).not.toHaveBeenCalled();
  });

  it("command first: a held stale read cannot undo the save, and a schedule-only save preserves it", async () => {
    const page = renderPage();
    await screen.findByDisplayValue("0 0 15 * *");

    server.holdServer = true;
    page.setWatchers(1);
    await waitFor(() => expect(server.held).toHaveLength(1));

    await saveRow("Edit command", "Command", "sh -c 'printf \"new\\n\"'");
    await waitFor(() => expect(server.mutations).toHaveLength(1));
    await act(async () => releaseHeld());

    await waitFor(() => expect(toast.success).toHaveBeenCalledTimes(1));
    await settle();
    expect(screen.getByRole("textbox", { name: "Command" })).toHaveValue(
      "sh -c 'printf \"new\\n\"'",
    );

    await saveRow("Edit schedule", "Schedule", "0 0 16 * *");
    await waitFor(() => expect(server.mutations).toHaveLength(2));
    expect(server.mutations[1]).toEqual({
      schedule: "0 0 16 * *",
      command: "sh -c 'printf \"new\\n\"'",
    });
    expect(server.command).toBe("sh -c 'printf \"new\\n\"'");
  });

  it("a failed refresh keeps the committed save, blocks stale edits, and retries only the read", async () => {
    server.failReadsAfterMutation = 1;
    renderPage();
    await screen.findByDisplayValue("0 0 15 * *");

    await saveRow("Edit schedule", "Schedule", "0 0 16 * *");

    await waitFor(() => expect(toast.warning).toHaveBeenCalledTimes(1));
    expect(server.mutations).toHaveLength(1);
    expect(toast.success).not.toHaveBeenCalled();
    expect(toast.error).not.toHaveBeenCalled();
    // Stale rows are not writable while the fresh read is outstanding.
    expect(
      screen.queryByRole("button", { name: "Edit schedule" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Edit command" }),
    ).not.toBeInTheDocument();

    const options = vi.mocked(toast.warning).mock.calls[0]?.[1] as {
      action: { label: string; onClick: () => void };
    };
    expect(options.action.label).toBe("Try again");
    await act(async () => options.action.onClick());

    await waitFor(() => expect(toast.success).toHaveBeenCalledTimes(1));
    await settle();
    expect(screen.getByRole("textbox", { name: "Schedule" })).toHaveValue(
      "0 0 16 * *",
    );
    expect(
      screen.getByRole("button", { name: "Edit schedule" }),
    ).toBeInTheDocument();
    expect(server.mutations).toHaveLength(1); // retry never replays the write
  });

  it("native cron (w8/m69): a held stale read cannot restore the dropped override", async () => {
    server.runtime = "node";
    server.startCommand = "npm start";
    server.command = "[[ 1 = 1 ]] && printf old";
    const page = renderPage();
    await screen.findByDisplayValue("[[ 1 = 1 ]] && printf old");

    server.holdServer = true;
    page.setWatchers(1);
    await waitFor(() => expect(server.held).toHaveLength(1));

    // Pasted: user.type reads "[[" as an escaped "[".
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Edit command" }));
    await user.clear(screen.getByRole("textbox", { name: "Command" }));
    await user.paste("[[ 1 = 1 ]] && printf new");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(server.startCommandWrites).toHaveLength(1));
    await act(async () => releaseHeld());

    await waitFor(() => expect(toast.success).toHaveBeenCalledTimes(1));
    await settle();
    expect(screen.getByRole("textbox", { name: "Command" })).toHaveValue(
      "[[ 1 = 1 ]] && printf new",
    );

    // The reschedule replays no override: the stale one never came back.
    await saveRow("Edit schedule", "Schedule", "0 0 16 * *");
    await waitFor(() => expect(server.mutations).toHaveLength(1));
    expect(server.mutations[0]).toEqual({
      schedule: "0 0 16 * *",
      command: null,
    });
  });

  it("control: settled saves without an overlapping read preserve both fields", async () => {
    renderPage();
    await screen.findByDisplayValue("0 0 15 * *");

    await saveRow("Edit schedule", "Schedule", "0 0 16 * *");
    await waitFor(() => expect(server.mutations).toHaveLength(1));
    await saveRow("Edit command", "Command", "sh -c 'printf \"new\\n\"'");
    await waitFor(() => expect(server.mutations).toHaveLength(2));

    expect(server.mutations[1]).toEqual({
      schedule: "0 0 16 * *",
      command: "sh -c 'printf \"new\\n\"'",
    });
    expect(toast.warning).not.toHaveBeenCalled();
  });
});
