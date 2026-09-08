import fs from "fs";
import path from "path";
import {
  PostgresLifecycleController,
  type PostgresLifecycleDecision,
  type PostgresLifecycleResource,
} from "../lifecycle";

const database = (
  patch: Partial<PostgresLifecycleResource> = {},
): PostgresLifecycleResource => ({
  id: "dpg-one",
  name: "primary",
  status: "available",
  suspended: "not_suspended",
  ...patch,
});

const ready = (
  patch: Partial<PostgresLifecycleDecision> = {},
): PostgresLifecycleDecision => ({
  outcome: "allowed",
  precondition: "",
  ...patch,
});

describe("Postgres mobile lifecycle domain", () => {
  it("gates every verb on the server decision, not datastore state", async () => {
    const controller = new PostgresLifecycleController({
      mutate: {
        suspend: async () => undefined,
        resume: async () => undefined,
        restart: async () => undefined,
      },
      refresh: async () => database({ suspended: "suspended" }),
      wait: async () => undefined,
      maxPolls: 1,
    });
    // A ready decision runs regardless of the local status/suspended
    // predicates the projection replaced — even a transitional status no
    // longer blocks.
    expect(
      await controller.run({
        action: "suspend",
        resource: database({ status: "creating" }),
        confirmed: true,
        decision: ready(),
      }),
    ).toEqual({
      status: "success",
      resource: database({ suspended: "suspended" }),
    });
    // No row for this action means the verb does not exist for this resource.
    expect(
      await controller.run({
        action: "resume",
        resource: database(),
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
      ready({ precondition: "unavailable" }),
    ]) {
      expect(
        await controller.run({
          action: "resume",
          resource: database(),
          confirmed: true,
          decision,
        }),
      ).toEqual({ status: "not_allowed", reason: "state" });
    }
  });

  it("still suspends through the protected-environment phrase round trip", async () => {
    const controller = new PostgresLifecycleController({
      mutate: {
        suspend: async () => {
          throw new Error(
            'protected environment: retry with confirm="sudo suspend database primary"',
          );
        },
        resume: async () => undefined,
        restart: async () => undefined,
      },
      refresh: async () => database(),
    });
    expect(
      await controller.run({
        action: "suspend",
        resource: database(),
        confirmed: true,
        decision: ready({ precondition: "protected_confirmation_required" }),
      }),
    ).toEqual({
      status: "confirmation_required",
      source: "server",
      confirmation: "sudo suspend database primary",
    });
  });

  it("requires device confirmation, disables duplicate execution, and polls truth", async () => {
    let release: (() => void) | undefined;
    const mutation = new Promise<void>((resolve) => {
      release = resolve;
    });
    let current = database();
    const controller = new PostgresLifecycleController({
      mutate: {
        suspend: async () => mutation,
        resume: async () => undefined,
        restart: async () => undefined,
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
    expect(
      await controller.run({
        action: "restart",
        resource: current,
        confirmed: false,
        decision: ready(),
      }),
    ).toEqual({ status: "confirmation_required", source: "device" });
    // Resume needs no device confirmation: unconfirmed still converges.
    expect(
      (
        await controller.run({
          action: "resume",
          resource: database({ suspended: "suspended" }),
          confirmed: false,
          decision: ready(),
        })
      ).status,
    ).toBe("success");
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
    current = database({ suspended: "suspended" });
    release?.();
    expect((await first).status).toBe("success");
    expect(controller.pendingAction(current.id)).toBe(null);
  });

  it("refreshes server truth and times out instead of reporting optimistic success", async () => {
    let current = database();
    const controller = new PostgresLifecycleController({
      mutate: {
        suspend: async () => undefined,
        resume: async () => undefined,
        restart: async () => undefined,
      },
      refresh: async () => current,
      wait: async () => undefined,
      maxPolls: 1,
    });
    expect(
      (
        await controller.run({
          action: "suspend",
          resource: current,
          confirmed: true,
          decision: ready(),
        })
      ).status,
    ).toBe("timeout");
    current = database({ suspended: "suspended" });
    expect(
      (
        await controller.run({
          action: "suspend",
          resource: database(),
          confirmed: true,
          decision: ready(),
        })
      ).status,
    ).toBe("success");
  });

  it("reports restart acceptance without inventing convergence", async () => {
    const current = database();
    let refreshes = 0;
    const controller = new PostgresLifecycleController({
      mutate: {
        suspend: async () => undefined,
        resume: async () => undefined,
        restart: async () => undefined,
      },
      refresh: async () => {
        refreshes += 1;
        return current;
      },
      wait: async () => undefined,
      maxPolls: 3,
    });
    const result = await controller.run({
      action: "restart",
      resource: current,
      confirmed: true,
      decision: ready(),
    });
    expect(result.status).toBe("accepted_unverified");
    expect(refreshes).toBe(1);
  });

  it("keeps its GraphQL surface to the exact three lifecycle operations", () => {
    const document = fs.readFileSync(
      path.resolve(
        process.cwd(),
        "src/features/postgres/api/lifecycle.graphql",
      ),
      "utf8",
    );
    expect(
      [...document.matchAll(/^mutation\s+(\w+)/gm)].map((match) => match[1]),
    ).toEqual([
      "MobileSuspendPostgres",
      "MobileResumePostgres",
      "MobileRestartPostgres",
    ]);
  });
});
