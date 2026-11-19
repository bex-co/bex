import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CreateApiKeyDialog } from "@/features/api-keys/components/create-api-key-dialog";
import type { UseApiKeyPermissionCatalogResult } from "@/features/api-keys/hooks/use-api-key-permission-catalog";
import {
  CATALOG,
  VIEWER_CATALOG,
} from "@/features/api-keys/__tests__/catalog-fixtures";

const create = vi.fn();
let busy = false;
vi.mock("@/features/api-keys/hooks/use-create-api-key", () => ({
  useCreateApiKey: () => ({ create, busy }),
}));

// bex-api's catalog as a Developer sees it (w9/m496).
let catalog: UseApiKeyPermissionCatalogResult;
const catalogRead = vi.fn();
vi.mock("@/features/api-keys/hooks/use-api-key-permission-catalog", () => ({
  useApiKeyPermissionCatalog: () => {
    catalogRead();
    return catalog;
  },
}));

const copy = vi.fn();
vi.mock("@/common/hooks/use-copy-to-clipboard", () => ({
  useCopyToClipboard: () => ({ copied: false, copy }),
}));

vi.mock("@/config/config", () => ({
  config: {
    apiBaseUrl: "https://api.example.test",
    oauthTokenEndpoint: "https://oauth.example.test/oauth2/token",
  },
}));

// The shapes bex-api mints (docs/ADR012-auth.md §8e): a UUID, a 26-character
// URL-safe secret, and the bearer string that joins them.
const ID = "0190f3a2-6c1e-7d4b-9a58-2f6e1c0b7d43";
// Joined, not written out: a literal that looks like a secret trips the
// repository's credential scan. 26 characters, as bex-api's secrets are.
const SECRET = ["fixture", "client", "secret", "0001"].join("-");
const KEY = `bex_${ID}_${SECRET}`;
const CREATED = { id: ID, name: "deploy-agent", key: KEY, secret: SECRET };

beforeEach(() => {
  create.mockReset();
  copy.mockReset();
  catalogRead.mockReset();
  busy = false;
  catalog = { areas: CATALOG, error: undefined, refetch: vi.fn() };
});

/** Opens the dialog, names the key, submits, and waits for the created step. */
async function mint(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Create API key" }));
  const dialog = await screen.findByRole("dialog");
  await user.type(within(dialog).getByLabelText("Name"), "deploy-agent");
  await user.click(within(dialog).getByRole("button", { name: "Create" }));
  await within(dialog).findByText(KEY);
  return dialog;
}

/** The collapsed "OAuth client credentials" detail. */
function oauthDetail(dialog: HTMLElement): HTMLDetailsElement {
  const detail = within(dialog)
    .getByText("OAuth client credentials")
    .closest("details");
  if (!detail) throw new Error("the OAuth detail is not a <details>");
  return detail;
}

describe("CreateApiKeyDialog — one bearer key, shown once (w4/m8/t003, w7/m426)", () => {
  it("shows the one key as the primary credential and copies exactly it", async () => {
    create.mockResolvedValue(CREATED);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    const dialog = await mint(user);

    // Untouched, the picker stays on All (w9/m496).
    expect(create).toHaveBeenCalledWith("deploy-agent", { all: true });
    const field = within(dialog).getByRole("group", { name: "API key" });
    expect(within(field).getByText(KEY)).toBeVisible();

    await user.click(
      within(field).getByRole("button", { name: "Copy API key" }),
    );
    expect(copy).toHaveBeenCalledTimes(1);
    expect(copy).toHaveBeenCalledWith(KEY);
  });

  it("says in the primary area that the key is shown once and is bounded by your role", async () => {
    create.mockResolvedValue(CREATED);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    const dialog = await mint(user);

    const once = within(dialog).getByText(
      "Copy the key now. You won't be able to see it again.",
    );
    // w9/m481, w9/m496: a key uses only the permissions chosen for it, and
    // never more than its creator's current member role in this workspace
    // allows, as on Render; it never had "full authority".
    const authority = within(dialog).getByText(
      /can use only the permissions you chose for it, never more than your current member role in this workspace allows/,
    );
    expect(authority).toHaveTextContent(/treat it like a password/);
    expect(authority).toHaveTextContent(/Authorization: Bearer/);
    // Neither statement may hide inside the collapsed detail.
    for (const statement of [once, authority]) {
      expect(statement).toBeVisible();
      expect(oauthDetail(dialog)).not.toContainElement(statement);
    }
  });

  it("gives a curl that lists services at the API origin with the key as the bearer", async () => {
    create.mockResolvedValue(CREATED);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    const dialog = await mint(user);

    const example = within(dialog).getByRole("group", {
      name: "Example request",
    });
    const curl = example.querySelector("pre")?.textContent ?? "";
    expect(curl).toMatch(/^curl /);
    expect(curl).toContain("https://api.example.test/v1/services");
    expect(curl).toContain(`Authorization: Bearer ${KEY}`);

    // What is copied is what is shown: the whole command, real key included.
    await user.click(
      within(example).getByRole("button", { name: "Copy example request" }),
    );
    expect(copy).toHaveBeenCalledWith(curl);
  });

  it("keeps the client ID and secret in a collapsed detail, not as the primary credential", async () => {
    create.mockResolvedValue(CREATED);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    const dialog = await mint(user);

    const detail = oauthDetail(dialog);
    expect(detail.open).toBe(false);
    // Both halves live only inside the detail, hidden until it is opened.
    expect(within(dialog).getAllByText(ID)).toHaveLength(1);
    expect(within(dialog).getAllByText(SECRET)).toHaveLength(1);
    expect(within(detail).getByText(ID)).not.toBeVisible();
    expect(within(detail).getByText(SECRET)).not.toBeVisible();

    await user.click(within(detail).getByText("OAuth client credentials"));
    expect(detail.open).toBe(true);
    expect(within(detail).getByText(ID)).toBeVisible();
    expect(within(detail).getByText(SECRET)).toBeVisible();

    await user.click(
      within(detail).getByRole("button", { name: "Copy client ID" }),
    );
    expect(copy).toHaveBeenLastCalledWith(ID);
    await user.click(
      within(detail).getByRole("button", { name: "Copy client secret" }),
    );
    expect(copy).toHaveBeenLastCalledWith(SECRET);
  });

  it("offers the client_credentials exchange as optional, at the configured token endpoint", async () => {
    create.mockResolvedValue(CREATED);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    const dialog = await mint(user);

    const help = within(oauthDetail(dialog)).getByText(/client_credentials/);
    expect(help).toHaveTextContent("https://oauth.example.test/oauth2/token");
    expect(help).toHaveTextContent(/also works as an OAuth2 client/);
    expect(help).toHaveTextContent(/If you prefer short-lived tokens/);
    // The key is a bearer token now: nothing may say otherwise.
    expect(dialog).not.toHaveTextContent(/not a bearer/i);
    // w4/148: machine keys carry no capability scopes (w4/112), so the dialog
    // must not advertise any: Hydra answers them with invalid_scope.
    expect(help).toHaveTextContent(/Do not send a scope/);
    expect(help).not.toHaveTextContent(/bex\.read|bex\.write|bex\.sensitive/);
  });

  it("the key exists nowhere after the dialog is dismissed and reopened", async () => {
    create.mockResolvedValue(CREATED);
    const onCreated = vi.fn();
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={onCreated} />);
    let dialog = await mint(user);

    // Dismiss (the "Done" button), then reopen — nothing in the DOM carries the
    // key, its secret or the curl over; the dialog resets to the name-entry
    // step, not a stale reveal.
    await user.click(within(dialog).getByRole("button", { name: "Done" }));
    expect(document.body).not.toHaveTextContent(SECRET);
    expect(onCreated).toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Create API key" }));
    dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText("Name")).toBeInTheDocument();
    expect(document.body).not.toHaveTextContent(SECRET);
    expect(document.body).not.toHaveTextContent(ID);
  });

  it("a create failure (null from the hook) keeps the dialog on the name step — no key to show (t006)", async () => {
    create.mockResolvedValue(null);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Create API key" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "deploy-agent");
    await user.click(within(dialog).getByRole("button", { name: "Create" }));

    expect(within(dialog).getByLabelText("Name")).toBeInTheDocument();
    expect(
      within(dialog).queryByText(/won't be able to see it again/i),
    ).not.toBeInTheDocument();
    expect(
      within(dialog).queryByRole("group", { name: "API key" }),
    ).not.toBeInTheDocument();
  });

  it("keeps submit disabled for an empty/whitespace-only name", async () => {
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Create API key" }));
    const dialog = await screen.findByRole("dialog");
    const submit = within(dialog).getByRole("button", { name: "Create" });
    expect(submit).toBeDisabled();

    await user.type(within(dialog).getByLabelText("Name"), "   ");
    expect(submit).toBeDisabled();
    expect(create).not.toHaveBeenCalled();
  });

  it("a create in flight keeps the submit's label in its box, so the button does not change width", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<CreateApiKeyDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Create API key" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "deploy-agent");
    expect(
      within(dialog).getByRole("button", { name: "Create" }),
    ).toBeEnabled();

    busy = true;
    rerender(<CreateApiKeyDialog onCreated={vi.fn()} />);

    // The label stays in the button while the create runs, so it still sizes
    // the button; how the spinner is drawn is the shared Button's business.
    const pending = within(dialog).getByRole("button", { name: "Create" });
    expect(pending).toBeDisabled();
    expect(pending).toHaveTextContent("Create");
    expect(
      within(dialog).getByRole("button", { name: "Cancel" }),
    ).toBeDisabled();
  });
});

/** Opens the dialog and names the key; returns the dialog. */
async function openNamed(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Create API key" }));
  const dialog = await screen.findByRole("dialog");
  await user.type(within(dialog).getByLabelText("Name"), "deploy-agent");
  return dialog;
}

describe("CreateApiKeyDialog — the key's permissions (w9/m496)", () => {
  it("opens on All, loads the catalog only while open, and creates with {all: true}", async () => {
    create.mockResolvedValue(CREATED);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    expect(catalogRead).not.toHaveBeenCalled();

    const dialog = await openNamed(user);
    expect(catalogRead).toHaveBeenCalled();
    expect(
      within(dialog).getByRole("group", {
        name: "Select the permissions this API key can use.",
      }),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(
        /All grants every permission the creator's member role allows/,
      ),
    ).toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "Create" }));
    expect(create).toHaveBeenCalledWith("deploy-agent", { all: true });
  });

  it("Read only creates with Read on every permission the viewer may grant it on", async () => {
    create.mockResolvedValue(CREATED);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    const dialog = await openNamed(user);

    await user.click(within(dialog).getByRole("button", { name: "Read only" }));
    await user.click(within(dialog).getByRole("button", { name: "Create" }));

    expect(create).toHaveBeenCalledWith("deploy-agent", {
      grants: [
        { permission: "services", level: "read" },
        { permission: "services_env", level: "read" },
        { permission: "git", level: "read" },
      ],
    });
  });

  it("Clear all creates a key with no permissions", async () => {
    create.mockResolvedValue(CREATED);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    const dialog = await openNamed(user);

    await user.click(within(dialog).getByRole("button", { name: "Clear all" }));
    await user.click(within(dialog).getByRole("button", { name: "Create" }));

    expect(create).toHaveBeenCalledWith("deploy-agent", { grants: [] });
  });

  it("creates with exactly the subset chosen", async () => {
    create.mockResolvedValue(CREATED);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    const dialog = await openNamed(user);

    await user.click(within(dialog).getByRole("button", { name: "Clear all" }));
    await user.click(
      within(dialog).getByRole("switch", {
        name: "Grant Services permissions",
      }),
    );
    await user.click(
      within(
        within(dialog).getByRole("radiogroup", {
          name: "Web and private services, static sites, background workers, and cron jobs",
        }),
      ).getByRole("radio", { name: "Read" }),
    );
    await user.click(
      within(
        within(dialog).getByRole("radiogroup", {
          name: "Environment variables and secret files",
        }),
      ).getByRole("radio", { name: "None" }),
    );
    await user.click(within(dialog).getByRole("button", { name: "Create" }));

    expect(create).toHaveBeenCalledWith("deploy-agent", {
      grants: [{ permission: "services", level: "read" }],
    });
  });

  it("a level the viewer cannot grant is disabled with the role it needs", async () => {
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    const dialog = await openNamed(user);

    const readWrite = within(
      within(dialog).getByRole("radiogroup", {
        name: "Git connections and repositories",
      }),
    ).getByRole("radio", { name: "Read & write" });
    expect(readWrite).toBeDisabled();
    expect(readWrite).toHaveAccessibleDescription(
      "Read & write requires the Admin role.",
    );
  });

  it("still creates (with All) when the catalog cannot load, and says so", async () => {
    catalog = {
      areas: undefined,
      error: new Error("boom"),
      refetch: vi.fn().mockResolvedValue(undefined),
    };
    create.mockResolvedValue(CREATED);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    const dialog = await openNamed(user);

    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      "Couldn't load the permission list. A key you create now gets All permissions.",
    );
    // The read can be tried again in place.
    await user.click(within(dialog).getByRole("button", { name: "Try again" }));
    expect(catalog.refetch).toHaveBeenCalledTimes(1);

    await user.click(within(dialog).getByRole("button", { name: "Create" }));
    expect(create).toHaveBeenCalledWith("deploy-agent", { all: true });
  });

  it("a reopened dialog starts on All again", async () => {
    create.mockResolvedValue(null);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    let dialog = await openNamed(user);
    await user.click(within(dialog).getByRole("button", { name: "Clear all" }));
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));

    dialog = await openNamed(user);
    await user.click(within(dialog).getByRole("button", { name: "Create" }));
    expect(create).toHaveBeenLastCalledWith("deploy-agent", { all: true });
  });
});

describe("CreateApiKeyDialog — a Viewer mints a key capped at their role (w9/m514)", () => {
  beforeEach(() => {
    catalog = { areas: VIEWER_CATALOG, error: undefined, refetch: vi.fn() };
  });

  it("creates an All key, which acts with no more than the Viewer's role", async () => {
    create.mockResolvedValue(CREATED);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    const dialog = await openNamed(user);

    await user.click(within(dialog).getByRole("button", { name: "Create" }));
    expect(create).toHaveBeenCalledWith("deploy-agent", { all: true });
  });

  it("Read only selects only the Read levels a Viewer may grant", async () => {
    create.mockResolvedValue(CREATED);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    const dialog = await openNamed(user);

    await user.click(within(dialog).getByRole("button", { name: "Read only" }));
    await user.click(within(dialog).getByRole("button", { name: "Create" }));

    // Environment variables at Read needs a Developer: it stays None.
    expect(create).toHaveBeenCalledWith("deploy-agent", {
      grants: [
        { permission: "services", level: "read" },
        { permission: "git", level: "read" },
      ],
    });
  });

  it("disables every level above Viewer and names the role it needs", async () => {
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);
    const dialog = await openNamed(user);

    const services = within(
      within(dialog).getByRole("radiogroup", {
        name: "Web and private services, static sites, background workers, and cron jobs",
      }),
    );
    expect(services.getByRole("radio", { name: "Read" })).toBeEnabled();
    const readWrite = services.getByRole("radio", { name: "Read & write" });
    expect(readWrite).toBeDisabled();
    expect(readWrite).toHaveAccessibleDescription(
      "Read & write requires the Developer role.",
    );
    const env = within(
      within(dialog).getByRole("radiogroup", {
        name: "Environment variables and secret files",
      }),
    );
    for (const name of ["Read", "Write", "Read & write"]) {
      expect(env.getByRole("radio", { name })).toBeDisabled();
    }
  });
});
