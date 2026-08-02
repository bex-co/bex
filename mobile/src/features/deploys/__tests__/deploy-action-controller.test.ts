import { DeployActionController } from "../deploy-action-controller";
import { DeployMutationFailure } from "../deploy-action-errors";
import type {
  DeployActionTransport,
  DeployMutationResult,
} from "../deploy-action-types";

type Resolve = (result: DeployMutationResult) => void;

class FakeTransport implements DeployActionTransport {
  triggerCalls: string[] = [];
  cancelCalls: string[][] = [];
  rollbackCalls: string[][] = [];
  triggerPromise?: Promise<DeployMutationResult>;
  triggerError?: unknown;

  async trigger(serviceId: string): Promise<DeployMutationResult> {
    this.triggerCalls.push(serviceId);
    if (this.triggerError) throw this.triggerError;
    return this.triggerPromise ?? accepted("dep-trigger", "created");
  }

  async cancel(
    serviceId: string,
    deployId: string,
  ): Promise<DeployMutationResult> {
    this.cancelCalls.push([serviceId, deployId]);
    return accepted(deployId, "canceled");
  }

  async rollback(
    serviceId: string,
    deployId: string,
  ): Promise<DeployMutationResult> {
    this.rollbackCalls.push([serviceId, deployId]);
    return accepted("dep-rollback", "created");
  }
}

const triggerRequest = (requestId: string) => ({
  requestId,
  action: "trigger" as const,
  serviceId: "srv-one",
  serviceSuspended: false,
});

describe("DeployActionController", () => {
  it("single-flights double taps and replays a stable confirmation at most once", async () => {
    const transport = new FakeTransport();
    let resolve: Resolve = () => undefined;
    transport.triggerPromise = new Promise((done) => {
      resolve = done;
    });
    const subject = new DeployActionController(transport);

    const first = subject.execute(triggerRequest("confirmation-one"));
    const replay = subject.execute(triggerRequest("confirmation-one"));
    const secondTap = subject.execute(triggerRequest("confirmation-two"));
    expect(first === replay).toBe(true);
    expect(transport.triggerCalls.length).toBe(1);
    resolve(accepted("dep-one", "created"));

    const [firstResult, replayResult, dedupedResult] = await Promise.all([
      first,
      replay,
      secondTap,
    ]);
    expect(firstResult).toEqual(replayResult);
    expect(dedupedResult.deduplicated).toBe(true);
    expect(dedupedResult.requestId).toBe("confirmation-two");
    await subject.execute(triggerRequest("confirmation-one"));
    expect(transport.triggerCalls.length).toBe(1);
  });

  it("never sends ineligible terminal cancel or rollback requests", async () => {
    const transport = new FakeTransport();
    const subject = new DeployActionController(transport);
    const cancel = await subject.execute({
      requestId: "confirmation-cancel",
      action: "cancel",
      serviceId: "srv-one",
      serviceSuspended: false,
      target: { id: "dep-live", status: "live" },
    });
    const rollback = await subject.execute({
      requestId: "confirmation-rollback",
      action: "rollback",
      serviceId: "srv-one",
      serviceSuspended: false,
      target: { id: "dep-failed", status: "update_failed" },
    });
    expect(cancel.outcome).toBe("rejected");
    expect(rollback.outcome).toBe("rejected");
    expect(transport.cancelCalls.length).toBe(0);
    expect(transport.rollbackCalls.length).toBe(0);
  });

  it("passes only service and known target ids to each mutation", async () => {
    const transport = new FakeTransport();
    const subject = new DeployActionController(transport);
    await subject.execute({
      requestId: "confirmation-cancel",
      action: "cancel",
      serviceId: "srv-one",
      serviceSuspended: false,
      target: { id: "dep-open", status: "build_in_progress" },
    });
    await subject.execute({
      requestId: "confirmation-rollback",
      action: "rollback",
      serviceId: "srv-one",
      serviceSuspended: false,
      target: { id: "dep-live", status: "deactivated" },
    });
    expect(transport.cancelCalls).toEqual([["srv-one", "dep-open"]]);
    expect(transport.rollbackCalls).toEqual([["srv-one", "dep-live"]]);
  });

  it("does not retry an ambiguous timeout and requires refresh", async () => {
    const transport = new FakeTransport();
    transport.triggerError = new DeployMutationFailure(
      "request timed out",
      "possibly_sent",
    );
    const subject = new DeployActionController(transport);
    const result = await subject.execute(
      triggerRequest("confirmation-timeout"),
    );
    expect(result.outcome).toBe("unknown");
    if (result.outcome === "unknown") {
      expect(result.error.delivery).toBe("possibly_committed");
      expect(result.error.refreshRequired).toBe(true);
    }
    expect(transport.triggerCalls.length).toBe(1);
    await subject.execute(triggerRequest("confirmation-timeout"));
    expect(transport.triggerCalls.length).toBe(1);
  });

  it("treats a missing deploy id as an unknown partial success", async () => {
    const transport = new FakeTransport();
    transport.triggerPromise = Promise.resolve({
      deploy: { id: null, status: "created" },
    });
    const result = await new DeployActionController(transport).execute(
      triggerRequest("confirmation-no-id"),
    );
    expect(result.outcome).toBe("unknown");
    if (result.outcome === "unknown") {
      expect(result.error.code).toBe("invalid_response");
      expect(result.error.retry).toBe("after_refresh");
    }
  });

  it("moves accepted work from refresh-pending to observed terminal convergence", async () => {
    const transport = new FakeTransport();
    const subject = new DeployActionController(transport);
    const result = await subject.execute(
      triggerRequest("confirmation-converge"),
    );
    expect(result.outcome).toBe("accepted");
    expect(subject.state("confirmation-converge")?.phase).toBe(
      "awaiting_refresh",
    );
    subject.markConverged("confirmation-converge", "live");
    expect(subject.state("confirmation-converge")?.phase).toBe("converged");
  });

  it("does not publish a stale result after the identity boundary clears", async () => {
    const transport = new FakeTransport();
    let resolve: Resolve = () => undefined;
    transport.triggerPromise = new Promise((done) => {
      resolve = done;
    });
    const subject = new DeployActionController(transport);
    const observed: string[] = [];
    subject.subscribe((state) => observed.push(state.phase));
    const pending = subject.execute(triggerRequest("confirmation-old-user"));
    subject.clear();
    resolve(accepted("dep-old-user", "created"));
    await pending;
    expect(observed).toEqual(["submitting"]);
    expect(subject.state("confirmation-old-user")).toBe(undefined);
  });
});

function accepted(id: string, status: string): DeployMutationResult {
  return { deploy: { id, status } };
}
