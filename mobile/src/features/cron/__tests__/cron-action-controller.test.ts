import {
  CronActionController,
  awaitCronActionObservation,
  cronActionObserved,
  type CronActionRequest,
  type CronActionTransport,
  type CronMutationResult,
} from "../cron-action-controller";
import type { CronRunSummary } from "../cron-history";

type Resolve = (result: CronMutationResult) => void;

const cronRun = (id: string, status: string): CronRunSummary => ({
  id,
  status,
  startedAt: null,
  finishedAt: null,
});

class FakeTransport implements CronActionTransport {
  runCalls: string[] = [];
  cancelCalls: string[][] = [];
  runPromise?: Promise<CronMutationResult>;
  cancelPromise?: Promise<CronMutationResult>;
  runError?: unknown;

  async run(serviceId: string): Promise<CronMutationResult> {
    this.runCalls.push(serviceId);
    if (this.runError) throw this.runError;
    return this.runPromise ?? { run: { id: "crr-new", status: "pending" } };
  }

  async cancel(serviceId: string, runId: string): Promise<CronMutationResult> {
    this.cancelCalls.push([serviceId, runId]);
    return this.cancelPromise ?? { run: { id: runId, status: "canceled" } };
  }
}

const readyGate = { outcome: "allowed", precondition: "" } as const;

const runRequest = (requestId: string): CronActionRequest => ({
  requestId,
  action: "run",
  serviceId: "srv-cron",
  server: { ...readyGate },
});

describe("CronActionController", () => {
  it("single-flights double taps and never replays one confirmation", async () => {
    const transport = new FakeTransport();
    let resolve: Resolve = () => undefined;
    transport.runPromise = new Promise((done) => {
      resolve = done;
    });
    const subject = new CronActionController(transport);
    const first = subject.execute(runRequest("confirm-one"));
    const replay = subject.execute(runRequest("confirm-one"));
    const doubleTap = subject.execute(runRequest("confirm-two"));
    expect(first === replay).toBe(true);
    expect(transport.runCalls).toEqual(["srv-cron"]);
    resolve({ run: { id: "crr-new", status: "pending" } });
    expect((await doubleTap).deduplicated).toBe(true);
    await first;
    await subject.execute(runRequest("confirm-one"));
    expect(transport.runCalls).toEqual(["srv-cron"]);
  });

  it("allows run-now while unsuspended because the server replaces an active run", async () => {
    const transport = new FakeTransport();
    const result = await new CronActionController(transport).execute(
      runRequest("confirm-replace"),
    );
    expect(result.outcome).toBe("accepted");
    expect(transport.runCalls).toEqual(["srv-cron"]);
  });

  it("walks a nested deterministic error only once", async () => {
    const transport = new FakeTransport();
    let codeReads = 0;
    transport.runError = {
      extensions: {
        get code() {
          codeReads += 1;
          return "CRON_SUSPENDED";
        },
      },
    };
    const result = await new CronActionController(transport).execute(
      runRequest("confirm-coded-error"),
    );
    expect(result.outcome).toBe("rejected");
    expect(codeReads).toBe(1);
  });

  it("clear aborts active work and releases its confirmation record", async () => {
    const transport = new FakeTransport();
    transport.runPromise = new Promise(() => undefined);
    const subject = new CronActionController(transport);
    const first = subject.execute(runRequest("confirm-clear"));
    subject.clear();
    expect((await first).outcome).toBe("unknown");

    subject.markAuthoritativelyRefreshed("srv-cron");
    transport.runPromise = undefined;
    expect((await subject.execute(runRequest("confirm-clear"))).outcome).toBe(
      "accepted",
    );
    expect(transport.runCalls).toEqual(["srv-cron", "srv-cron"]);
  });

  it("blocks on the server gate without sending", async () => {
    const transport = new FakeTransport();
    const subject = new CronActionController(transport);
    const suspended = await subject.execute({
      ...runRequest("confirm-suspended"),
      server: { outcome: "allowed", precondition: "suspended" },
    });
    const noRun = await subject.execute({
      requestId: "confirm-no-run",
      action: "cancel",
      serviceId: "srv-cron",
      server: { outcome: "allowed", precondition: "no_active_run" },
      target: cronRun("crr-done", "successful"),
    });
    const missing = await subject.execute({
      ...runRequest("confirm-missing"),
      server: null,
    });
    expect(suspended.outcome).toBe("rejected");
    expect(noRun.outcome).toBe("rejected");
    expect(missing.outcome).toBe("rejected");
    expect(transport.runCalls).toEqual([]);
    expect(transport.cancelCalls).toEqual([]);
  });

  it("lets the server gate decide, not the cached run status", async () => {
    const transport = new FakeTransport();
    // A terminal-status row still sends when the projection is ready: history
    // may be stale and the verb rechecks at dispatch.
    const result = await new CronActionController(transport).execute({
      requestId: "confirm-stale-status",
      action: "cancel",
      serviceId: "srv-cron",
      server: { ...readyGate },
      target: cronRun("crr-done", "successful"),
    });
    expect(result.outcome).toBe("accepted");
    expect(transport.cancelCalls).toEqual([["srv-cron", "crr-done"]]);
  });

  it("rejects a reused confirmation when the server gate changes", async () => {
    const transport = new FakeTransport();
    const subject = new CronActionController(transport);
    const first = await subject.execute(runRequest("confirm-gate-change"));
    expect(first.outcome).toBe("accepted");
    const replay = await subject.execute({
      ...runRequest("confirm-gate-change"),
      server: { outcome: "allowed", precondition: "billing_blocked" },
    });
    expect(replay.outcome).toBe("rejected");
    expect(transport.runCalls).toEqual(["srv-cron"]);
  });

  it("passes only the service id and current opaque run id to cancel", async () => {
    const transport = new FakeTransport();
    const target = cronRun("crr-current", "pending");
    await new CronActionController(transport).execute({
      requestId: "confirm-cancel",
      action: "cancel",
      serviceId: "srv-cron",
      server: { ...readyGate },
      target,
    });
    expect(transport.cancelCalls).toEqual([["srv-cron", "crr-current"]]);
  });

  it("single-flights duplicate cancel confirmations and rejects a changed-target replay", async () => {
    const transport = new FakeTransport();
    let resolve: Resolve = () => undefined;
    transport.cancelPromise = new Promise((done) => {
      resolve = done;
    });
    const subject = new CronActionController(transport);
    const target = cronRun("crr-current", "running");
    const request: CronActionRequest = {
      requestId: "confirm-cancel-one",
      action: "cancel",
      serviceId: "srv-cron",
      server: { ...readyGate },
      target,
    };

    const first = subject.execute(request);
    const duplicate = subject.execute({
      ...request,
      requestId: "confirm-cancel-two",
    });
    const changedTarget = await subject.execute({
      ...request,
      target: cronRun("crr-other", "running"),
    });
    expect(changedTarget.outcome).toBe("rejected");
    expect(transport.cancelCalls).toEqual([["srv-cron", "crr-current"]]);

    resolve({ run: { id: target.id, status: "canceled" } });
    expect((await first).outcome).toBe("accepted");
    expect((await duplicate).deduplicated).toBe(true);
    await subject.execute(request);
    expect(transport.cancelCalls).toEqual([["srv-cron", "crr-current"]]);
  });

  it("keeps an ambiguous timeout unknown and does not retry it", async () => {
    const transport = new FakeTransport();
    transport.runError = Object.assign(new Error("gateway timeout"), {
      name: "TimeoutError",
    });
    const subject = new CronActionController(transport);
    const request = runRequest("confirm-timeout");
    const result = await subject.execute(request);
    expect(result.outcome).toBe("unknown");
    if (result.outcome === "unknown") {
      expect(result.refreshRequired).toBe(true);
    }
    await subject.execute(request);
    expect(transport.runCalls).toEqual(["srv-cron"]);
    const blocked = await subject.execute(runRequest("confirm-after-timeout"));
    expect(blocked.outcome).toBe("rejected");
    expect(transport.runCalls).toEqual(["srv-cron"]);
    subject.markAuthoritativelyRefreshed("srv-cron");
    await subject.execute(runRequest("confirm-after-refresh"));
    expect(transport.runCalls).toEqual(["srv-cron", "srv-cron"]);
  });

  it("bounds a hung transport and reports an unknown outcome", async () => {
    const transport = new FakeTransport();
    transport.runPromise = new Promise(() => undefined);
    const result = await new CronActionController(transport, 1).execute(
      runRequest("confirm-hung"),
    );
    expect(result.outcome).toBe("unknown");
  });

  it("maps billing enforcement to a deterministic conflict", async () => {
    const transport = new FakeTransport();
    transport.runError = {
      graphQLErrors: [{ extensions: { code: "BILLING_ENFORCED" } }],
    };
    const result = await new CronActionController(transport).execute(
      runRequest("confirm-billing"),
    );
    expect(result.outcome).toBe("rejected");
    if (result.outcome === "rejected") {
      expect((result.error as { statusCode?: number }).statusCode).toBe(409);
    }
  });

  it("uses exact cron codes and never error prose for deterministic outcomes", async () => {
    for (const code of [
      "CRON_SUSPENDED",
      "CRON_RUN_NOT_FOUND",
      "CRON_RUN_TERMINAL",
      "BILLING_ENFORCED",
    ]) {
      const transport = new FakeTransport();
      transport.runError = { extensions: { code } };
      const result = await new CronActionController(transport).execute(
        runRequest(`confirm-${code}`),
      );
      expect(result.outcome).toBe("rejected");
      if (result.outcome === "rejected") {
        expect(result.refreshRequired).toBe(true);
        expect((result.error as { statusCode?: number }).statusCode).toBe(409);
      }
    }

    const transport = new FakeTransport();
    transport.runError = new Error(
      "forbidden terminal suspended workspace billing enforcement is active",
    );
    const arbitrary = await new CronActionController(transport).execute(
      runRequest("confirm-arbitrary-prose"),
    );
    expect(arbitrary.outcome).toBe("unknown");
  });

  it("keeps payment required deterministic but separate from cron conflicts", async () => {
    const transport = new FakeTransport();
    transport.runError = { extensions: { code: "PAYMENT_REQUIRED" } };
    const result = await new CronActionController(transport).execute(
      runRequest("confirm-payment-required"),
    );
    expect(result.outcome).toBe("rejected");
    if (result.outcome === "rejected") {
      expect(result.refreshRequired).toBe(false);
      expect((result.error as { statusCode?: number }).statusCode).toBe(
        undefined,
      );
    }
  });

  it("keeps transport-level 404 and 409 outcomes ambiguous without exact codes", async () => {
    for (const statusCode of [404, 409]) {
      const transport = new FakeTransport();
      transport.runError = Object.assign(new Error("transport response"), {
        statusCode,
      });
      const result = await new CronActionController(transport).execute(
        runRequest(`confirm-raw-${statusCode}`),
      );
      expect(result.outcome).toBe("unknown");
      if (result.outcome === "unknown") {
        expect(result.refreshRequired).toBe(true);
      }
    }
  });

  it("keeps authorization refusal deterministic", async () => {
    const transport = new FakeTransport();
    transport.runError = {
      message: "GraphQL request failed",
      graphQLErrors: [
        {
          message: "caller is not authorized",
          extensions: { code: "FORBIDDEN" },
        },
      ],
    };
    const result = await new CronActionController(transport).execute(
      runRequest("confirm-forbidden"),
    );
    expect(result.outcome).toBe("rejected");
    expect(transport.runCalls).toEqual(["srv-cron"]);
  });

  it("does not misattribute an unrelated scheduled run to an ambiguous run-now", () => {
    const request = runRequest("confirm-ambiguous");
    const before = [cronRun("crr-before", "successful")];
    expect(
      cronActionObserved(
        request,
        {
          outcome: "unknown",
          action: "run",
          error: new Error("network"),
          refreshRequired: true,
          deduplicated: false,
        },
        [cronRun("crr-scheduled", "running"), ...before],
      ),
    ).toBe(false);
  });

  it("requires the exact run and terminal phase before cancel converges", () => {
    const target = cronRun("crr-target", "running");
    const request: CronActionRequest = {
      requestId: "confirm-cancel-phase",
      action: "cancel",
      serviceId: "srv-cron",
      server: { ...readyGate },
      target,
    };
    const accepted = {
      outcome: "accepted" as const,
      action: "cancel" as const,
      runId: target.id,
      status: "canceled",
      deduplicated: false,
    };

    expect(cronActionObserved(request, accepted, [target])).toBe(false);
    expect(
      cronActionObserved(request, accepted, [
        target,
        cronRun("crr-other", "canceled"),
      ]),
    ).toBe(false);
    expect(
      cronActionObserved(request, accepted, [cronRun(target.id, "canceled")]),
    ).toBe(true);
  });

  it("requires the accepted run id before run-now converges", () => {
    const request = runRequest("confirm-run-phase");
    const accepted = {
      outcome: "accepted" as const,
      action: "run" as const,
      runId: "crr-manual",
      status: "pending",
      deduplicated: false,
    };
    expect(
      cronActionObserved(request, accepted, [
        cronRun("crr-scheduled", "running"),
      ]),
    ).toBe(false);
    expect(
      cronActionObserved(request, accepted, [cronRun("crr-manual", "pending")]),
    ).toBe(true);
  });

  it("polls authoritative history to convergence and times out honestly", async () => {
    const request = runRequest("confirm-poll");
    const accepted = {
      outcome: "accepted" as const,
      action: "run" as const,
      runId: "crr-new",
      status: "pending",
      deduplicated: false,
    };
    let refreshes = 0;
    const converged = await awaitCronActionObservation({
      request,
      result: accepted,
      refresh: async () => {
        refreshes += 1;
        return refreshes === 2 ? [cronRun("crr-new", "pending")] : [];
      },
      wait: async () => undefined,
      maxPolls: 3,
    });
    expect(converged.observed).toBe(true);
    expect(refreshes).toBe(2);

    const timedOut = await awaitCronActionObservation({
      request,
      result: accepted,
      refresh: async () => [],
      wait: async () => undefined,
      maxPolls: 2,
    });
    expect(timedOut.observed).toBe(false);
  });
});
