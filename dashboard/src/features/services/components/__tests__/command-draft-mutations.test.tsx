import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  SetBuildCommandDocument,
  SetPreDeployCommandDocument,
  SetStartCommandDocument,
  UpdateCronJobDocument,
} from "@/graphql/definitions";
import { BuildDeploySection } from "@/features/services/components/build-deploy-section";
import { CronDeploySection } from "@/features/services/components/cron-deploy-section";
import { DeployCard } from "@/features/services/components/deploy-card";

// Command editors at the mutation boundary: real sections and hooks, with only
// Apollo's useMutation faked (w8/m68).
const ESCAPED_SPACE = "printf '%s' tail\\ ";
const BLANK = " \t ";

const mutate = vi.fn(
  async (_document: unknown, _options: { variables: object }) => ({
    data: {},
  }),
);
// The cron save's post-commit refresh settles in-flight Server reads, then
// re-reads by id; with no watchers here both resolve empty.
const apolloClient = {
  refetchQueries: vi.fn(() =>
    Object.assign(Promise.resolve([]), { results: [] }),
  ),
  query: vi.fn(async () => ({})),
};
vi.mock("@apollo/client/react", () => ({
  useMutation: (document: unknown) => [
    (options: { variables: object }) => mutate(document, options),
    { loading: false },
  ],
  useApolloClient: () => apolloClient,
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

// The Deploy card embeds the Deploy Hook rows, which read their own query.
vi.mock("@/features/services/hooks/use-deploy-hook", () => ({
  useDeployHook: () => ({
    url: "https://api.bex.co/v1/deploy-hooks/dhk-test",
    loading: false,
    error: undefined,
    regenerate: vi.fn(async () => true),
    regenerating: false,
  }),
}));

beforeEach(() => {
  mutate.mockClear();
  apolloClient.refetchQueries.mockClear();
  apolloClient.query.mockClear();
});

type User = ReturnType<typeof userEvent.setup>;

/** The variables of the one call `document`'s mutation received. */
async function sentVariables(document: unknown) {
  await waitFor(() =>
    expect(mutate.mock.calls.some(([doc]) => doc === document)).toBe(true),
  );
  const calls = mutate.mock.calls.filter(([doc]) => doc === document);
  expect(calls).toHaveLength(1);
  return calls[0][1].variables;
}

/** Open a row for editing and replace its draft with `text`, pasted as is. */
async function editTo(user: User, edit: string, field: string, text: string) {
  await user.click(screen.getByRole("button", { name: edit }));
  const input = screen.getByRole("textbox", { name: field });
  await user.clear(input);
  if (text !== "") await user.paste(text);
  // The input itself holds the exact bytes, so any change is the row's.
  expect(input).toHaveValue(text);
}

/**
 * Save the row, through its confirm dialog when it has one; with
 * `expectedTitle`, the dialog must be open and titled exactly that.
 */
async function save(user: User, expectedTitle?: string) {
  await user.click(screen.getByRole("button", { name: "Save changes" }));
  const dialog = expectedTitle
    ? screen.getByRole("alertdialog")
    : screen.queryByRole("alertdialog");
  if (!dialog) return;
  if (expectedTitle) {
    expect(within(dialog).getByText(expectedTitle)).toBeInTheDocument();
  }
  await user.click(
    within(dialog).getByRole("button", { name: "Save changes" }),
  );
}

function renderBuildCommand(buildCommand: string | null) {
  render(
    <BuildDeploySection
      serviceId="app"
      repo="https://github.com/x/site"
      branch="main"
      rootDir={null}
      buildCommand={buildCommand}
      autoDeploy={false}
      preDeployCommand={null}
      showPreDeployCommand={false}
      showBuildCommand
      showDeployCard={false}
      showSourceFields={false}
    />,
  );
}

function renderDeployCard(
  commandKind: "start" | "docker" | "image",
  {
    command = null,
    preDeployCommand = null,
  }: { command?: string | null; preDeployCommand?: string | null },
) {
  render(
    <DeployCard
      serviceId="app"
      commandKind={commandKind}
      command={command}
      preDeployCommand={preDeployCommand}
      showPreDeployCommand
      showCommand
    />,
  );
}

const setter = (command: string) => ({ id: "app", command });

const ROWS = [
  {
    name: "Build Command",
    edit: "Edit build command",
    field: "Build Command",
    document: SetBuildCommandDocument,
    renderRow: renderBuildCommand,
    sent: setter,
    clearTitle: "Change build command to Runtime default?",
  },
  {
    name: "Pre-Deploy Command",
    edit: "Edit Pre-Deploy Command",
    field: "Pre-Deploy Command",
    document: SetPreDeployCommandDocument,
    renderRow: (preDeployCommand: string | null) =>
      renderDeployCard("start", { preDeployCommand }),
    sent: setter,
  },
  {
    name: "Start Command",
    edit: "Edit Start Command",
    field: "Start Command",
    document: SetStartCommandDocument,
    renderRow: (command: string | null) =>
      renderDeployCard("start", { command }),
    sent: setter,
    clearTitle: "Change the start command to the default command?",
  },
  {
    name: "Dockerfile Docker Command",
    edit: "Edit Docker Command",
    field: "Docker Command",
    document: SetStartCommandDocument,
    renderRow: (command: string | null) =>
      renderDeployCard("docker", { command }),
    sent: setter,
    clearTitle: "Change the Docker Command to the default command?",
  },
  {
    name: "Existing Image Docker Command",
    edit: "Edit Docker Command",
    field: "Docker Command",
    document: SetStartCommandDocument,
    renderRow: (command: string | null) =>
      renderDeployCard("image", { command }),
    sent: setter,
    clearTitle: "Change the Docker Command to the default command?",
  },
  {
    name: "cron Command",
    edit: "Edit command",
    field: "Command",
    document: UpdateCronJobDocument,
    renderRow: (command: string | null) =>
      render(
        <CronDeploySection
          serviceId="nightly"
          schedule="0 6 * * *"
          command={command}
        />,
      ),
    // The Command row carries the persisted schedule.
    sent: (command: string) => ({
      id: "nightly",
      schedule: "0 6 * * *",
      command,
    }),
  },
];

describe.each(ROWS)("$name", ({ edit, field, document, renderRow, sent }) => {
  it("sends a command ending in an escaped space verbatim", async () => {
    const user = userEvent.setup();
    renderRow("node old.js");
    await editTo(user, edit, field, ESCAPED_SPACE);
    await save(user);
    expect(await sentVariables(document)).toEqual(sent(ESCAPED_SPACE));
  });

  it("opens a stored command ending in an escaped space clean", async () => {
    const user = userEvent.setup();
    renderRow(ESCAPED_SPACE);
    await user.click(screen.getByRole("button", { name: edit }));
    // Trimming the untouched draft made it differ from the saved command,
    // so Save offered to rewrite it without its final escaped character.
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
  });

  it("offers no save for a whitespace-only draft over an unset command", async () => {
    const user = userEvent.setup();
    renderRow(null);
    await editTo(user, edit, field, BLANK);
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(mutate).not.toHaveBeenCalled();
  });
});

// cron-deploy-section.test.tsx owns the cron row's whitespace-only clear.
it.each(ROWS.filter((row) => row.document !== UpdateCronJobDocument))(
  "$name clears on a whitespace-only draft, like an empty one",
  async ({ edit, field, document, renderRow, sent, clearTitle }) => {
    const user = userEvent.setup();
    renderRow("node old.js");
    await editTo(user, edit, field, BLANK);
    await save(user, clearTitle);
    expect(await sentVariables(document)).toEqual(sent(""));
  },
);
