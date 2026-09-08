import fs from "fs";
import path from "path";
import {
  KeyValueLifecycleController,
  type KeyValueLifecycleDecision,
  type KeyValueLifecycleResource,
} from "../lifecycle";

const keyValue = (
  patch: Partial<KeyValueLifecycleResource> = {},
): KeyValueLifecycleResource => ({
  id: "red-one",
  name: "cache",
  status: "available",
  suspended: "not_suspended",
  ...patch,
});

const ready = (
  patch: Partial<KeyValueLifecycleDecision> = {},
): KeyValueLifecycleDecision => ({
  outcome: "allowed",
  precondition: "",
  ...patch,
});

describe("Key Value mobile lifecycle domain", () => {
  it("gates every verb on the server decision, not store state", async () => {
    const controller = new KeyValueLifecycleController({
      mutate: {
        suspend: async () => undefined,
        resume: async () => undefined,
      },
      refresh: async () => keyValue({ suspended: "suspended" }),
      wait: async () => undefined,
      maxPolls: 1,
    });
    // A ready decision runs regardless of the local status/suspended
    // predicates the projection replaced — even a transitional status no
    // longer blocks.
    expect(
      await controller.run({
        action: "suspend",
        resource: keyValue({ status: "creating" }),
        confirmed: true,
        decision: ready(),
      }),
    ).toEqual({
      status: "success",
      resource: keyValue({ suspended: "suspended" }),
    });
    // No row for this action means the verb does not exist for this resource.
    expect(
      await controller.run({
        action: "resume",
        resource: keyValue(),
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
          resource: keyValue(),
          confirmed: true,
          decision,
        }),
      ).toEqual({ status: "not_allowed", reason: "state" });
    }
  });

  it("still suspends through the protected-environment phrase round trip", async () => {
    const controller = new KeyValueLifecycleController({
      mutate: {
        suspend: async () => {
          throw new Error(
            'protected environment: retry with confirm="sudo suspend keyvalue cache"',
          );
        },
        resume: async () => undefined,
      },
      refresh: async () => keyValue(),
    });
    expect(
      await controller.run({
        action: "suspend",
        resource: keyValue(),
        confirmed: true,
        decision: ready({ precondition: "protected_confirmation_required" }),
      }),
    ).toEqual({
      status: "confirmation_required",
      source: "server",
      confirmation: "sudo suspend keyvalue cache",
    });
  });

  it("requires device confirmation for suspend only", async () => {
    let current = keyValue();
    const controller = new KeyValueLifecycleController({
      mutate: {
        suspend: async () => undefined,
        resume: async () => undefined,
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
    // Resume needs no device confirmation: unconfirmed proceeds to send
    // (here it times out waiting for convergence instead of asking).
    current = keyValue({ suspended: "suspended" });
    expect(
      (
        await controller.run({
          action: "resume",
          resource: current,
          confirmed: false,
          decision: ready(),
        })
      ).status,
    ).toBe("timeout");
  });

  it("does not claim success until a refresh observes the requested state", async () => {
    const current = keyValue();
    const controller = new KeyValueLifecycleController({
      mutate: {
        suspend: async () => undefined,
        resume: async () => undefined,
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
  });

  it("keeps its GraphQL surface to the exact two server operations", () => {
    const document = fs.readFileSync(
      path.resolve(
        process.cwd(),
        "src/features/keyvalue/api/lifecycle.graphql",
      ),
      "utf8",
    );
    expect(
      [...document.matchAll(/^mutation\s+(\w+)/gm)].map((match) => match[1]),
    ).toEqual(["MobileSuspendKeyValue", "MobileResumeKeyValue"]);
  });
});
