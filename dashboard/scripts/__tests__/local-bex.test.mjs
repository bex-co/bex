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

describe("local-bex.mjs GitHub connections", () => {
  it("answers the plural connection query used by create source pickers", async () => {
    const { data, errors } = await graphql(
      "GitConnections",
      `
        query GitConnections {
          gitConnections {
            accountLogin
            installationId
            installUrl
          }
        }
      `,
    );

    expect(errors).toBeUndefined();
    expect(data.gitConnections).toEqual([
      expect.objectContaining({
        accountLogin: "acme-corp",
        installationId: 87654321,
      }),
    ]);
  });
});

describe("local-bex.mjs workspaceMembers", () => {
  it("returns enriched member rows (userId + email) for the default workspace", async () => {
    const { data, errors } = await graphql(
      "WorkspaceMembers",
      WORKSPACE_MEMBERS_QUERY,
      {
        workspaceId: "tea-localdefault00000001",
      },
    );

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
    const { data, errors } = await graphql(
      "WorkspaceMembers",
      WORKSPACE_MEMBERS_QUERY,
      {
        workspaceId: "tea-localsecond000000002",
      },
    );

    expect(errors).toBeUndefined();
    expect(data.workspaceMembers).toEqual([]);
  });

  it("returns an empty list for an unknown workspace id rather than erroring", async () => {
    const { data, errors } = await graphql(
      "WorkspaceMembers",
      WORKSPACE_MEMBERS_QUERY,
      {
        workspaceId: "tea-does-not-exist",
      },
    );

    expect(errors).toBeUndefined();
    expect(data.workspaceMembers).toEqual([]);
  });
});

const DEPLOY_HOOK_QUERY = `
  query DeployHook($serviceId: String!) {
    deployHook(serviceId: $serviceId) { url }
  }
`;

const REGENERATE_DEPLOY_HOOK_MUTATION = `
  mutation RegenerateDeployHook($serviceId: String!) {
    regenerateDeployHook(serviceId: $serviceId) { url }
  }
`;

describe("local-bex.mjs deploy hooks", () => {
  it("triggers without auth and invalidates the old URL after regeneration", async () => {
    const firstResult = await graphql("DeployHook", DEPLOY_HOOK_QUERY, {
      serviceId: "eden-cms-v2",
    });
    const firstURL = firstResult.data.deployHook.url;

    const firstTrigger = await fetch(firstURL, { method: "POST" });
    expect(firstTrigger.status).toBe(200);
    await expect(firstTrigger.json()).resolves.toMatchObject({
      deploy: { id: expect.stringMatching(/^dep-local-hook-/) },
    });

    const rotatedResult = await graphql(
      "RegenerateDeployHook",
      REGENERATE_DEPLOY_HOOK_MUTATION,
      { serviceId: "eden-cms-v2" },
    );
    const rotatedURL = rotatedResult.data.regenerateDeployHook.url;
    expect(rotatedURL).not.toBe(firstURL);

    expect((await fetch(firstURL)).status).toBe(404);
    expect((await fetch(rotatedURL)).status).toBe(200);
  });
});

describe("local-bex.mjs Build & Deploy settings", () => {
  it("persists Start Command and Dockerfile Path across a fresh server read", async () => {
    const start = await graphql(
      "SetStartCommand",
      `
        mutation SetStartCommand($id: String!, $command: String!) {
          setStartCommand(id: $id, command: $command) {
            startCommand
          }
        }
      `,
      { id: "eden-cms-v2", command: "bin/server" },
    );
    expect(start.errors).toBeUndefined();
    expect(start.data.setStartCommand.startCommand).toBe("bin/server");

    const pathResult = await graphql(
      "SetDockerfilePath",
      `
        mutation SetDockerfilePath($id: String!, $dockerfilePath: String!) {
          setDockerfilePath(id: $id, dockerfilePath: $dockerfilePath) {
            dockerfilePath
          }
        }
      `,
      {
        id: "eden-cms-v2",
        dockerfilePath: "docker/Dockerfile.prod",
      },
    );
    expect(pathResult.errors).toBeUndefined();
    expect(pathResult.data.setDockerfilePath.dockerfilePath).toBe(
      "docker/Dockerfile.prod",
    );

    const read = await graphql(
      "Server",
      `
        query Server($id: String!) {
          server(id: $id) {
            startCommand
            dockerfilePath
          }
        }
      `,
      { id: "eden-cms-v2" },
    );
    expect(read.data.server).toMatchObject({
      startCommand: "bin/server",
      dockerfilePath: "docker/Dockerfile.prod",
    });
  });
});

describe("local-bex.mjs service environment", () => {
  it("returns paged key/name shapes without bulk secret material", async () => {
    const env = await graphql(
      "EnvVarKeys",
      `
        query EnvVarKeys($serviceId: String!) {
          envVars(serviceId: $serviceId) {
            envVar {
              id
              key
            }
            cursor
          }
        }
      `,
      { serviceId: "eden-cms-v2" },
    );
    expect(env.errors).toBeUndefined();
    expect(env.data.envVars).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          envVar: {
            id: "APP_MODE",
            key: "APP_MODE",
            __typename: "EnvVarListValue",
          },
          cursor: "APP_MODE",
        }),
      ]),
    );
    expect(JSON.stringify(env.data)).not.toContain("production");

    const files = await graphql(
      "SecretFileNames",
      `
        query SecretFileNames($serviceId: String!) {
          secretFiles(serviceId: $serviceId) {
            secretFile {
              id
              name
            }
            cursor
          }
        }
      `,
      { serviceId: "eden-cms-v2" },
    );
    expect(files.errors).toBeUndefined();
    expect(files.data.secretFiles[0].secretFile.name).toBe(
      "service-account.json",
    );
    expect(JSON.stringify(files.data)).not.toContain("project");
  });

  it("applies one sparse save-only patch and preserves omitted values", async () => {
    const patched = await graphql(
      "PatchServiceEnvironment",
      `
        mutation PatchServiceEnvironment(
          $serviceId: String!
          $envVars: [EnvironmentEnvVarPatchInput!]
          $secretFiles: [EnvironmentSecretFilePatchInput!]
          $saveMode: String!
        ) {
          patchServiceEnvironment(
            serviceId: $serviceId
            envVars: $envVars
            secretFiles: $secretFiles
            saveMode: $saveMode
          ) {
            envVarKeys
            secretFileNames
            rolledOut
          }
        }
      `,
      {
        serviceId: "eden-cms-v2",
        saveMode: "save_only",
        envVars: [
          { key: "APP_STAGE", fromKey: "APP_MODE" },
          { key: "ADDED", value: "new-secret" },
        ],
        secretFiles: [
          { name: "renamed.json", fromName: "service-account.json" },
        ],
      },
    );
    expect(patched.errors).toBeUndefined();
    expect(patched.data.patchServiceEnvironment).toMatchObject({
      envVarKeys: ["ADDED", "APP_STAGE", "DATABASE_URL"],
      secretFileNames: ["renamed.json"],
      rolledOut: false,
    });
    expect(JSON.stringify(patched.data)).not.toContain("new-secret");

    const unchanged = await graphql(
      "EnvVarValue",
      `
        query EnvVarValue($id: String!, $key: String!) {
          service(id: $id) {
            envVar(key: $key) {
              key
              value
            }
          }
        }
      `,
      { id: "eden-cms-v2", key: "DATABASE_URL" },
    );
    expect(unchanged.data.service.envVar.value).toContain("postgres://");
  });
});

describe("local-bex.mjs service environment groups", () => {
  it("creates a populated group already linked, then unlinks and relinks it", async () => {
    const created = await graphql(
      "CreateEnvGroup",
      `
        mutation CreateEnvGroup($name: String!) {
          createEnvGroup(name: $name) {
            id
            name
          }
        }
      `,
      {
        name: "browser-shared",
        ownerId: "tea-localdefault00000001",
        envVars: [{ key: "SHARED_BROWSER", value: "fixture-only" }],
        secretFiles: [{ name: "shared.pem", content: "fixture-only" }],
        serviceIds: ["eden-cms-v2"],
      },
    );
    expect(created.errors).toBeUndefined();
    const id = created.data.createEnvGroup.id;

    const list = async () =>
      graphql(
        "EnvGroups",
        `
          query EnvGroups($ownerId: String) {
            envGroups(ownerId: $ownerId) {
              id
              name
              serviceLinks
              envVars {
                key
              }
              secretFiles {
                name
              }
            }
          }
        `,
        { ownerId: "tea-localdefault00000001" },
      );
    let group = (await list()).data.envGroups.find(
      (candidate) => candidate.id === id,
    );
    expect(group).toMatchObject({
      name: "browser-shared",
      serviceLinks: ["eden-cms-v2"],
      envVars: [{ key: "SHARED_BROWSER", __typename: "EnvVar" }],
      secretFiles: [{ name: "shared.pem", __typename: "SecretFile" }],
    });
    expect(JSON.stringify(group)).not.toContain("fixture-only");

    await graphql(
      "UnlinkEnvGroup",
      `
        mutation UnlinkEnvGroup($id: String!, $serviceId: String!) {
          unlinkEnvGroup(id: $id, serviceId: $serviceId)
        }
      `,
      { id, serviceId: "eden-cms-v2" },
    );
    group = (await list()).data.envGroups.find(
      (candidate) => candidate.id === id,
    );
    expect(group.serviceLinks).toEqual([]);

    await graphql(
      "LinkEnvGroup",
      `
        mutation LinkEnvGroup($id: String!, $serviceId: String!) {
          linkEnvGroup(id: $id, serviceId: $serviceId)
        }
      `,
      { id, serviceId: "eden-cms-v2" },
    );
    group = (await list()).data.envGroups.find(
      (candidate) => candidate.id === id,
    );
    expect(group.serviceLinks).toEqual(["eden-cms-v2"]);
  });
});

describe("local-bex.mjs API keys", () => {
  it("mints one bearer key from its id and secret, and never lists either again", async () => {
    // bex-api's contract (docs/ADR012-auth.md §8e, w7/m426): `key` is the one
    // string sent as Authorization: Bearer, `bex_<id>_<secret>`, and it exists
    // only in the create response.
    const ownerId = "tea-localdefault00000001";
    const { data, errors } = await graphql(
      "CreateApiKey",
      `
        mutation CreateApiKey($name: String!, $ownerId: String) {
          createApiKey(name: $name, ownerId: $ownerId) {
            id
            name
            key
            secret
            createdAt
          }
        }
      `,
      { name: "deploy-agent", ownerId },
    );

    expect(errors).toBeUndefined();
    const { id, key, secret } = data.createApiKey;
    expect(id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/,
    );
    expect(secret).toMatch(/^[A-Za-z0-9_-]{26}$/);
    expect(key).toBe(`bex_${id}_${secret}`);

    const listed = await graphql(
      "ApiKeys",
      `
        query ApiKeys($ownerId: String) {
          apiKeys(ownerId: $ownerId) {
            id
            name
            createdAt
            createdBy
            lastUsedAt
          }
        }
      `,
      { ownerId },
    );
    const row = listed.data.apiKeys.find((candidate) => candidate.id === id);
    expect(row).toMatchObject({ id, name: "deploy-agent" });
    expect(row).not.toHaveProperty("key");
    expect(row).not.toHaveProperty("secret");

    await graphql(
      "RevokeApiKey",
      `
        mutation RevokeApiKey($id: String!, $ownerId: String) {
          revokeApiKey(id: $id, ownerId: $ownerId)
        }
      `,
      { id, ownerId },
    );
  });

  it("keeps a key's permissions: All by default, then whatever the editor saves (w9/m496)", async () => {
    const ownerId = "tea-localdefault00000001";
    const PERMISSIONS = "permissions { all grants { permission level } }";
    const created = await graphql(
      "CreateApiKey",
      `
        mutation CreateApiKey($name: String!, $ownerId: String) {
          createApiKey(name: $name, ownerId: $ownerId) {
            id
          }
        }
      `,
      { name: "scoped-agent", ownerId },
    );
    const { id } = created.data.createApiKey;
    const list = async () =>
      (
        await graphql(
          "ApiKeys",
          `query ApiKeys($ownerId: String) { apiKeys(ownerId: $ownerId) { id ${PERMISSIONS} } }`,
          { ownerId },
        )
      ).data.apiKeys.find((key) => key.id === id);
    expect((await list()).permissions).toMatchObject({ all: true, grants: [] });

    const grants = [{ permission: "services", level: "read" }];
    const updated = await graphql(
      "UpdateApiKeyPermissions",
      `
        mutation UpdateApiKeyPermissions($id: String!, $ownerId: String, $permissions: ApiKeyPermissionsInput!) {
          updateApiKeyPermissions(id: $id, ownerId: $ownerId, permissions: $permissions) { id ${PERMISSIONS} }
        }
      `,
      { id, ownerId, permissions: { grants } },
    );
    expect(updated.errors).toBeUndefined();
    expect((await list()).permissions).toMatchObject({ all: false, grants });

    const catalog = await graphql(
      "ApiKeyPermissionCatalog",
      `
        query ApiKeyPermissionCatalog($ownerId: String) {
          apiKeyPermissionCatalog(ownerId: $ownerId) {
            id
            permissions {
              id
              levels {
                level
                grantable
              }
            }
          }
        }
      `,
      { ownerId },
    );
    const offered = catalog.data.apiKeyPermissionCatalog.flatMap((area) =>
      area.permissions.map((p) => p.id),
    );
    expect(offered).toContain("services");
  });
});

// A project's observability views (w3/m505, w3/m508) run offline: the seeded
// project has Render's three default views and its Service Metrics charts
// answer one series per service, labelled RESOURCE (w3/099's touch-target
// walk was taken on these pages).
describe("local-bex.mjs project observability views", () => {
  it("lists the seeded project's default views, by type, and none for an unknown project", async () => {
    const query = `
      query SavedSearches($projectId: String!, $type: SavedSearchType) {
        savedSearches(projectId: $projectId, type: $type) { id name type filters }
      }
    `;
    const all = await graphql("SavedSearches", query, {
      projectId: "prj-local0001",
    });
    expect(all.data.savedSearches.map((v) => [v.type, v.name])).toEqual([
      ["APPLICATION_LOGS", "Application Logs"],
      ["REQUEST_LOGS", "Request Logs"],
      ["METRICS", "Service Metrics"],
    ]);
    expect(all.data.savedSearches[0].filters).toEqual({
      filters: [],
      version: 1,
    });
    const metrics = await graphql("SavedSearches", query, {
      projectId: "prj-local0001",
      type: "METRICS",
    });
    expect(metrics.data.savedSearches.map((v) => v.type)).toEqual(["METRICS"]);
    const unknown = await graphql("SavedSearches", query, {
      projectId: "prj-missing",
    });
    expect(unknown.data.savedSearches).toEqual([]);
  });

  it("charts each of the project's services as its own RESOURCE series", async () => {
    const result = await graphql(
      "ProjectMetrics",
      `
        query ProjectMetrics($query: ProjectMetricsQueryInput!) {
          projectMetrics(query: $query) {
            series {
              unit
              labels {
                field
                value
              }
              values {
                time
                value
              }
            }
            resources {
              id
              name
              type
              status
            }
          }
        }
      `,
      { query: { project: "prj-local0001", name: "CPU" } },
    );
    const { series, resources } = result.data.projectMetrics;
    expect(resources.length).toBeGreaterThan(0);
    expect(
      series.map((s) => s.labels.map(({ field, value }) => [field, value])),
    ).toEqual(resources.map((r) => [["RESOURCE", r.id]]));
    expect(series.every((s) => s.values.length > 0)).toBe(true);
  });
});
