// @vitest-environment node
//
// Exercises the actual local-bex.mjs dev-stub server (not a reimplementation of
// it) over real HTTP, so drift between this stub and the real bex-api schema
// (WorkspaceMembersQuery: subject/userId/email/role/createdAt, w6/m10) fails a
// test instead of silently rotting — the offline Team page verification
// w6/m11/t004 exists to cover.
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import path from "node:path";

const scriptPath = fileURLToPath(new URL("../local-bex.mjs", import.meta.url));
const PORT = 8137; // distinct from the dev default (8099) to avoid clashing with a running `yarn local-bex`
const BASE = `http://localhost:${PORT}`;

let child;

async function graphql(operationName, query, variables = {}) {
  const res = await fetch(`${BASE}/graphql`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ operationName, query, variables }),
  });
  return res.json();
}

async function waitForServer() {
  const deadline = Date.now() + 10_000;
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`${BASE}/graphql`, { method: "OPTIONS" });
      if (res.status === 204) return;
    } catch {
      // not up yet
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error("local-bex.mjs did not start in time");
}

beforeAll(async () => {
  child = spawn(process.execPath, [scriptPath], {
    cwd: path.dirname(scriptPath),
    env: { ...process.env, PORT: String(PORT) },
    stdio: "ignore",
  });
  await waitForServer();
});

afterAll(() => {
  child?.kill();
});

const WORKSPACE_MEMBERS_QUERY = `
  query WorkspaceMembers($workspaceId: String!) {
    workspaceMembers(workspaceId: $workspaceId) {
      subject
      userId
      email
      role
      createdAt
    }
  }
`;

describe("local-bex.mjs workspaceMembers", () => {
  it("returns enriched member rows (userId + email) for the default workspace", async () => {
    const { data, errors } = await graphql("WorkspaceMembers", WORKSPACE_MEMBERS_QUERY, {
      workspaceId: "tea-localdefault00000001",
    });

    expect(errors).toBeUndefined();
    expect(Array.isArray(data.workspaceMembers)).toBe(true);
    expect(data.workspaceMembers.length).toBeGreaterThan(0);

    for (const member of data.workspaceMembers) {
      expect(typeof member.subject).toBe("string");
      expect(member.subject.length).toBeGreaterThan(0);
      // userId/email/role/createdAt must at least be present on the shape
      // (possibly empty string for email — the honest-degrade case below),
      // matching WorkspaceMembersQuery's selection set exactly.
      expect(member).toHaveProperty("userId");
      expect(member).toHaveProperty("email");
      expect(member).toHaveProperty("role");
      expect(member).toHaveProperty("createdAt");
    }

    // At least one row must be fully enriched (email-primary identity, the
    // behavior w6/m10's Team page click-through verifies) ...
    expect(data.workspaceMembers.some((m) => m.email && m.userId)).toBe(true);
    // ... and at least one row exercises the degraded case (userId resolved,
    // email lookup missed) so the userId-fallback rendering path isn't dead code.
    expect(data.workspaceMembers.some((m) => m.userId && !m.email)).toBe(true);
  });

  it("scopes members per workspace — an unseeded workspace returns none, not another workspace's rows", async () => {
    const { data, errors } = await graphql("WorkspaceMembers", WORKSPACE_MEMBERS_QUERY, {
      workspaceId: "tea-localsecond000000002",
    });

    expect(errors).toBeUndefined();
    expect(data.workspaceMembers).toEqual([]);
  });

  it("returns an empty list for an unknown workspace id rather than erroring", async () => {
    const { data, errors } = await graphql("WorkspaceMembers", WORKSPACE_MEMBERS_QUERY, {
      workspaceId: "tea-does-not-exist",
    });

    expect(errors).toBeUndefined();
    expect(data.workspaceMembers).toEqual([]);
  });
});
