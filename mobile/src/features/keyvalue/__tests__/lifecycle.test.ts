import fs from "fs";
import path from "path";
import {
  KeyValueLifecycleController,
  keyValueLifecycleCapabilities,
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

describe("Key Value mobile lifecycle domain", () => {
  it("maps every state to the exact server-backed capability set", () => {
    for (const status of ["available", "unavailable"]) {
      expect(keyValueLifecycleCapabilities(keyValue({ status }))).toEqual([
        { action: "suspend", requiresConfirmation: true },
      ]);
    }
    expect(
      keyValueLifecycleCapabilities(keyValue({ suspended: "suspended" })),
    ).toEqual([{ action: "resume", requiresConfirmation: false }]);
    for (const status of ["creating", "deleting", "unknown", ""]) {
      expect(keyValueLifecycleCapabilities(keyValue({ status }))).toEqual([]);
    }
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
