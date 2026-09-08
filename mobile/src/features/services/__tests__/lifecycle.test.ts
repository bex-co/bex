import fs from "fs";
import path from "path";
import {
  ServiceLifecycleController,
  type ServiceLifecycleDecision,
  type ServiceLifecycleResource,
} from "../lifecycle";

const service = (
  patch: Partial<ServiceLifecycleResource> = {},
): ServiceLifecycleResource => ({
  id: "srv-one",
  name: "api",
  type: "web_service",
  phase: "Running",
  suspended: "not_suspended",
  updatedAt: "2026-01-01T00:00:00Z",
  revision: "one",
  latestDeployId: "dep-old",
  ...patch,
});

const ready = (
  patch: Partial<ServiceLifecycleDecision> = {},
): ServiceLifecycleDecision => ({
  outcome: "allowed",
  precondition: "",
  ...patch,
});

describe("service lifecycle domain", () => {
  it("gates every verb on the server decision, not service state", async () => {
    const controller = new ServiceLifecycleController({
      mutate: {
        suspend: async () => undefined,
        resume: async () => undefined,
        restart: async () => ({ operationId: "dep-new" }),
      },
      refresh: async () => service({ suspended: "suspended" }),
      wait: async () => undefined,
      maxPolls: 1,
    });
    // A ready decision runs regardless of the local phase/type predicates the
    // projection replaced — even a transitional phase no longer blocks.
    expect(
      await controller.run({
        action: "suspend",
        resource: service({ phase: "Pending" }),
        confirmed: true,
        decision: ready(),
      }),
    ).toEqual({
      status: "success",
      resource: service({ suspended: "suspended" }),
    });
    // No row for this action means the verb does not exist for this resource.
    expect(
      await controller.run({
        action: "resume",
        resource: service(),
        confirmed: true,
        decision: null,
      }),
    ).toEqual({ status: "not_allowed", reason: "type" });
    // Denied, unavailable, and blocked decisions never send.
    for (const decision of [
      ready({ outcome: "denied" }),
      ready({ outcome: "unavailable" }),
      ready({ precondition: "billing_blocked" }),
      ready({ precondition: "suspended" }),
    ]) {
      expect(
        await controller.run({
          action: "resume",
          resource: service(),
          confirmed: true,
          decision,
        }),
      ).toEqual({ status: "not_allowed", reason: "state" });
    }
  });

  it("still suspends through the protected-environment phrase round trip", async () => {
    const controller = new ServiceLifecycleController({
      mutate: {
        suspend: async () => {
          throw new Error(
            'protected environment: retry with confirm="sudo suspend service api"',
          );
        },
        resume: async () => undefined,
        restart: async () => undefined,
      },
      refresh: async () => service(),
    });
    expect(
      await controller.run({
        action: "suspend",
        resource: service(),
        confirmed: true,
        decision: ready({ precondition: "protected_confirmation_required" }),
      }),
    ).toEqual({
      status: "confirmation_required",
      source: "server",
      confirmation: "sudo suspend service api",
    });
  });

  it("requires device confirmation, disables duplicate execution, and polls truth", async () => {
    let release: (() => void) | undefined;
    const mutation = new Promise<void>((resolve) => {
      release = resolve;
    });
    let current = service();
    const controller = new ServiceLifecycleController({
      mutate: {
        suspend: async () => mutation,
        resume: async () => undefined,
        restart: async () => ({ operationId: "dep-new" }),
      },
      refresh: async () => current,
      wait: async () => undefined,
      maxPolls: 1,
    });
    expect(
      await controller.run({
        action: "suspend",
        resource: current,
        confirmed: false,
        decision: ready(),
      }),
    ).toEqual({ status: "confirmation_required", source: "device" });
    const first = controller.run({
      action: "suspend",
      resource: current,
      confirmed: true,
      decision: ready(),
    });
    expect(controller.pendingAction(current.id)).toBe("suspend");
    expect(
      await controller.run({
        action: "restart",
        resource: current,
        confirmed: true,
        decision: ready(),
      }),
    ).toEqual({ status: "busy", action: "suspend" });
    current = service({ suspended: "suspended" });
    release?.();
    expect((await first).status).toBe("success");
    expect(controller.pendingAction(current.id)).toBe(null);
  });

  it("returns the authoritative protected-environment phrase", async () => {
    const controller = new ServiceLifecycleController({
      mutate: {
        suspend: async () => {
          throw new Error(
            'protected environment: retry with confirm="sudo suspend service api"',
          );
        },
        resume: async () => undefined,
        restart: async () => undefined,
      },
      refresh: async () => service(),
    });
    expect(
      await controller.run({
        action: "suspend",
        resource: service(),
        confirmed: true,
        decision: ready(),
      }),
    ).toEqual({
      status: "confirmation_required",
      source: "server",
      confirmation: "sudo suspend service api",
    });
  });

  it("keeps the lifecycle document to the exact three operations", () => {
    const document = fs.readFileSync(
      path.resolve(
        process.cwd(),
        "src/features/services/api/lifecycle.graphql",
      ),
      "utf8",
    );
    expect(
      [...document.matchAll(/^mutation\s+(\w+)/gm)].map((match) => match[1]),
    ).toEqual([
      "MobileSuspendService",
      "MobileResumeService",
      "MobileRestartService",
    ]);
  });
});
