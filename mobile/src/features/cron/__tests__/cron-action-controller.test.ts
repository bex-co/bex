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
  runError?: unknown;

  async run(serviceId: string): Promise<CronMutationResult> {
    this.runCalls.push(serviceId);
    if (this.runError) throw this.runError;
    return this.runPromise ?? { run: { id: "crr-new", status: "pending" } };
  }

  async cancel(serviceId: string, runId: string): Promise<CronMutationResult> {
    this.cancelCalls.push([serviceId, runId]);
    return { run: { id: runId, status: "canceled" } };
  }
}

const runRequest = (requestId: string): CronActionRequest => ({
  requestId,
  action: "run",
  serviceId: "srv-cron",
  serviceSuspended: false,
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

  it("blocks suspended run-now and terminal cancel without sending", async () => {
    const transport = new FakeTransport();
    const subject = new CronActionController(transport);
    const suspended = await subject.execute({
      ...runRequest("confirm-suspended"),
      serviceSuspended: true,
    });
    const terminal = await subject.execute({
      requestId: "confirm-terminal",
      action: "cancel",
      serviceId: "srv-cron",
      serviceSuspended: false,
      target: cronRun("crr-done", "successful"),
    });
    expect(suspended.outcome).toBe("rejected");
    expect(terminal.outcome).toBe("rejected");
    expect(transport.runCalls).toEqual([]);
    expect(transport.cancelCalls).toEqual([]);
  });

  it("still permits canceling an active run after the service is suspended", async () => {
    const transport = new FakeTransport();
    const target = cronRun("crr-active", "running");
    const result = await new CronActionController(transport).execute({
      requestId: "confirm-suspended-cancel",
      action: "cancel",
      serviceId: "srv-cron",
      serviceSuspended: true,
      target,
    });
    expect(result.outcome).toBe("accepted");
    expect(transport.cancelCalls).toEqual([["srv-cron", "crr-active"]]);
  });

  it("passes only the service id and current opaque run id to cancel", async () => {
    const transport = new FakeTransport();
    const target = cronRun("crr-current", "pending");
    await new CronActionController(transport).execute({
      requestId: "confirm-cancel",
      action: "cancel",
      serviceId: "srv-cron",
      serviceSuspended: false,
      target,
    });
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
