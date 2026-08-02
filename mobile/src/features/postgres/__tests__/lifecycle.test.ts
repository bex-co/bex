import fs from "fs";
import path from "path";
import {
  PostgresLifecycleController,
  postgresLifecycleCapabilities,
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

describe("Postgres mobile lifecycle domain", () => {
  it("maps every state to only the approved capabilities", () => {
    for (const status of ["available", "unavailable"]) {
      expect(postgresLifecycleCapabilities(database({ status }))).toEqual([
        { action: "suspend", requiresConfirmation: true },
        { action: "restart", requiresConfirmation: true },
      ]);
    }
    expect(
      postgresLifecycleCapabilities(database({ suspended: "suspended" })),
    ).toEqual([{ action: "resume", requiresConfirmation: false }]);
    for (const status of ["creating", "upgrading", "deleting", "unknown", ""]) {
      expect(postgresLifecycleCapabilities(database({ status }))).toEqual([]);
    }
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
