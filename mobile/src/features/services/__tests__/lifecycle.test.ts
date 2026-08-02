import fs from "fs";
import path from "path";
import {
  ServiceLifecycleController,
  serviceLifecycleCapabilities,
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

describe("service lifecycle domain", () => {
  it("offers only confirmed disruptive verbs for stable supported types", () => {
    expect(serviceLifecycleCapabilities(service())).toEqual([
      { action: "suspend", requiresConfirmation: true },
      { action: "restart", requiresConfirmation: true },
    ]);
    expect(
      serviceLifecycleCapabilities(service({ suspended: "suspended" })),
    ).toEqual([{ action: "resume", requiresConfirmation: false }]);
  });

  it("offers nothing for unknown types or transitional states", () => {
    for (const phase of ["Pending", "Building", "Deploying", "Deleting", ""]) {
      expect(serviceLifecycleCapabilities(service({ phase }))).toEqual([]);
    }
    expect(
      serviceLifecycleCapabilities(service({ type: "future_type" })),
    ).toEqual([]);
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
      }),
    ).toEqual({ status: "confirmation_required", source: "device" });
    const first = controller.run({
      action: "suspend",
      resource: current,
      confirmed: true,
    });
    expect(controller.pendingAction(current.id)).toBe("suspend");
    expect(
      await controller.run({
        action: "restart",
        resource: current,
        confirmed: true,
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
