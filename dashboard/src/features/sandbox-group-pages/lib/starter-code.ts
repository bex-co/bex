/**
 * The starter code of a sandbox group's empty states (w1/m658), Render's
 * "Create your first sandbox", "Create your first snapshot" and "Import your
 * first image": its CLI commands with bex's `bex` launcher, which runs
 * Render's CLI unchanged, and its SDK files with Render's own SDKs pointed at
 * bex-api (`base_url`, docs/ADR104-sandbox-render-parity.md § SDKs). Code is
 * never translated.
 */

import { config } from "@/config/config";

/** The API the SDK starter code points at: this dashboard's bex-api. */
export const API_BASE_URL = config.apiBaseUrl || "https://api.bex.co";

export const CLI_INSTALL = {
  homebrew: "brew install bex-co/tap/bex",
  linux:
    "curl -fsSL https://raw.githubusercontent.com/bex-co/bex/main/scripts/install-bex.sh | sh",
} as const;

export function cliLogin(workspaceId: string): string {
  return `bex login\nbex workspace set ${workspaceId}`;
}

export function cliCreateSandbox(groupId: string, image?: string): string {
  return image
    ? `bex sandboxes create --image ${image} --group ${groupId} --wait`
    : `bex sandboxes create --group ${groupId} --wait`;
}

export function cliImportImage(groupId: string): string {
  return `bex sandbox-groups images import ghcr.io/acme/agent-runtime:v1 --name agent-runtime --group ${groupId} --wait`;
}

export function cliSnapshot(sandboxId = "<sandbox-id>"): string {
  return `bex sandboxes snapshots create ${sandboxId} --kind filesystem --name my-first-snapshot --wait`;
}

export function cliStartFromSnapshot(groupId: string): string {
  return `bex sandboxes create --from my-first-snapshot --group ${groupId}`;
}

export const SDK_LANGUAGES = ["typescript", "python"] as const;
export type SdkLanguage = (typeof SDK_LANGUAGES)[number];

export const SDK_INSTALL: Record<SdkLanguage, string> = {
  python: `mkdir my-sandbox
cd my-sandbox
python3 -m venv .venv
source .venv/bin/activate
pip install -U "render>=1.5.0"`,
  typescript: `mkdir my-sandbox
cd my-sandbox
npm init -y
npm install @renderinc/sdk@latest`,
};

export const SDK_FILE: Record<SdkLanguage, string> = {
  python: "run_command.py",
  typescript: "run-command.mjs",
};

export const SNAPSHOT_SDK_FILE: Record<SdkLanguage, string> = {
  python: "snapshot.py",
  typescript: "snapshot.mjs",
};

export const IMAGE_SDK_FILE: Record<SdkLanguage, string> = {
  python: "image.py",
  typescript: "image.mjs",
};

export const SDK_RUN: Record<SdkLanguage, string> = {
  python: "python run_command.py",
  typescript: "node run-command.mjs",
};

export function sdkApiKey(): string {
  return "export RENDER_API_KEY=your-key-here";
}

interface StarterTarget {
  apiBaseUrl: string;
  workspaceId: string;
  sandboxGroupId: string;
}

export function sdkCreateSandbox(
  language: SdkLanguage,
  { apiBaseUrl, workspaceId, sandboxGroupId }: StarterTarget,
): string {
  if (language === "python") {
    return `from render import Render
from render.sandbox import SandboxExecOutput

WORKSPACE_ID = "${workspaceId}"
SANDBOX_GROUP_ID = "${sandboxGroupId}"

sandboxes = Render(owner_id=WORKSPACE_ID, base_url="${apiBaseUrl}").sandboxes

sandbox = sandboxes.create(
    sandbox_group_id=SANDBOX_GROUP_ID,
    wait=True,  # return once it's running
)
for event in sandboxes.exec(sandbox.id, "echo 'Hello from bex sandboxes'"):
    if isinstance(event, SandboxExecOutput):
        print(event.data, end="")
sandboxes.terminate(sandbox.id)`;
  }
  return `import { Render } from "@renderinc/sdk";

const WORKSPACE_ID = "${workspaceId}";
const SANDBOX_GROUP_ID = "${sandboxGroupId}";

const { sandboxes } = new Render({ ownerId: WORKSPACE_ID, baseUrl: "${apiBaseUrl}" });

const sandbox = await sandboxes.create({
  sandboxGroupId: SANDBOX_GROUP_ID,
  wait: true, // resolve once it's running
});
for await (const event of await sandboxes.exec(sandbox.id, "echo 'Hello from bex sandboxes'")) {
  if (event.type === "output") process.stdout.write(event.data);
}
await sandboxes.terminate(sandbox.id);`;
}

export function sdkSnapshot(
  language: SdkLanguage,
  { apiBaseUrl, workspaceId, sandboxGroupId }: StarterTarget,
): string {
  if (language === "python") {
    return `from render import Render

WORKSPACE_ID = "${workspaceId}"
SANDBOX_GROUP_ID = "${sandboxGroupId}"

sandboxes = Render(owner_id=WORKSPACE_ID, base_url="${apiBaseUrl}").sandboxes

# 1. Start a sandbox and set it up
source = sandboxes.create(
    sandbox_group_id=SANDBOX_GROUP_ID,
    wait=True,  # return once it's running, so it can be snapshotted
)
# Install dependencies, clone a repo: whatever new sandboxes should start with.

# 2. Snapshot its files
sandboxes.snapshots.create(
    source.id,
    name="my-first-snapshot",
    kind="filesystem",
    wait=True,  # return once the snapshot is available
)

# 3. Start new sandboxes from it
sandbox = sandboxes.create(
    sandbox_group_id=SANDBOX_GROUP_ID,
    snapshot_name="my-first-snapshot",
)`;
  }
  return `import { Render } from "@renderinc/sdk";

const WORKSPACE_ID = "${workspaceId}";
const SANDBOX_GROUP_ID = "${sandboxGroupId}";

const { sandboxes } = new Render({ ownerId: WORKSPACE_ID, baseUrl: "${apiBaseUrl}" });

// 1. Start a sandbox and set it up
const source = await sandboxes.create({
  sandboxGroupId: SANDBOX_GROUP_ID,
  wait: true, // resolve once it's running, so it can be snapshotted
});
// Install dependencies, clone a repo: whatever new sandboxes should start with.

// 2. Snapshot its files
await sandboxes.snapshots.create({
  sandboxId: source.id,
  name: "my-first-snapshot",
  kind: "filesystem",
  wait: true, // resolve once the snapshot is available
});

// 3. Start new sandboxes from it
const sandbox = await sandboxes.create({
  sandboxGroupId: SANDBOX_GROUP_ID,
  snapshotName: "my-first-snapshot",
});`;
}

/** Starting a sandbox from one snapshot, the snapshot drawer's starter code. */
export function sdkRestoreSnapshot(
  language: SdkLanguage,
  target: StarterTarget & { snapshotId: string },
): string {
  if (language === "python") {
    return `from render import Render

sandboxes = Render(owner_id="${target.workspaceId}", base_url="${target.apiBaseUrl}").sandboxes

sandbox = sandboxes.create(
    sandbox_group_id="${target.sandboxGroupId}",
    snapshot_id="${target.snapshotId}",
    wait=True,
)`;
  }
  return `import { Render } from "@renderinc/sdk";

const { sandboxes } = new Render({ ownerId: "${target.workspaceId}", baseUrl: "${target.apiBaseUrl}" });

const sandbox = await sandboxes.create({
  sandboxGroupId: "${target.sandboxGroupId}",
  snapshotId: "${target.snapshotId}",
  wait: true,
});`;
}

export function sdkImportImage(
  language: SdkLanguage,
  { apiBaseUrl, workspaceId, sandboxGroupId }: StarterTarget,
): string {
  if (language === "python") {
    return `from render import Render

WORKSPACE_ID = "${workspaceId}"
SANDBOX_GROUP_ID = "${sandboxGroupId}"

sandboxes = Render(owner_id=WORKSPACE_ID, base_url="${apiBaseUrl}").sandboxes

# 1. Import an image from a registry (linux/amd64)
image = sandboxes.images.import_(
    sandbox_group_id=SANDBOX_GROUP_ID,
    name="agent-runtime",
    source="ghcr.io/acme/agent-runtime:v1",
)

# 2. Wait for the import to finish. Raises if it fails.
sandboxes.images.wait_until_ready(
    sandbox_group_id=SANDBOX_GROUP_ID, image_id=image.id
)

# 3. Start new sandboxes from the image
sandbox = sandboxes.create(
    sandbox_group_id=SANDBOX_GROUP_ID,
    image="agent-runtime",
)`;
  }
  return `import { Render } from "@renderinc/sdk";

const WORKSPACE_ID = "${workspaceId}";
const SANDBOX_GROUP_ID = "${sandboxGroupId}";

const { sandboxes } = new Render({ ownerId: WORKSPACE_ID, baseUrl: "${apiBaseUrl}" });

// 1. Import an image from a registry (linux/amd64)
const image = await sandboxes.images.import({
  sandboxGroupId: SANDBOX_GROUP_ID,
  name: "agent-runtime",
  source: "ghcr.io/acme/agent-runtime:v1",
});

// 2. Wait for the import to finish. Throws if it fails.
await sandboxes.images.waitUntilReady({
  sandboxGroupId: SANDBOX_GROUP_ID,
  imageId: image.id,
});

// 3. Start new sandboxes from the image
const sandbox = await sandboxes.create({
  sandboxGroupId: SANDBOX_GROUP_ID,
  image: "agent-runtime",
});`;
}
