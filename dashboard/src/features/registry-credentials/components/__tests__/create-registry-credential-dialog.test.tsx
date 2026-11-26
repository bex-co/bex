import { describe, it, expect, vi, beforeEach, beforeAll } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CreateRegistryCredentialDialog } from "@/features/registry-credentials/components/create-registry-credential-dialog";

// Radix Select uses the Pointer Capture API and scrollIntoView jsdom lacks.
beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
  if (!Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = () => {};
  }
});

const create = vi.fn();
vi.mock(
  "@/features/registry-credentials/hooks/use-create-registry-credential",
  () => ({
    useCreateRegistryCredential: () => ({ create, busy: false }),
  }),
);

beforeEach(() => {
  create.mockReset();
});

type User = ReturnType<typeof userEvent.setup>;

async function openDialog(onCreated = vi.fn()) {
  const user = userEvent.setup();
  render(<CreateRegistryCredentialDialog onCreated={onCreated} />);
  await user.click(screen.getByRole("button", { name: "Add credential" }));
  const dialog = await screen.findByRole("dialog");
  return { user, dialog, onCreated };
}

async function chooseRegistry(user: User, dialog: HTMLElement, label: string) {
  await user.click(within(dialog).getByRole("combobox", { name: "Registry" }));
  await user.click(await screen.findByRole("option", { name: label }));
}

async function fill(user: User, input: HTMLElement, value: string) {
  await user.clear(input);
  await user.click(input);
  await user.paste(value);
  return input;
}

/** Name, the registry, Username and token: what every create needs. */
async function fillRequired(
  user: User,
  dialog: HTMLElement,
  registry = "GitHub",
) {
  await fill(user, within(dialog).getByLabelText("Name"), "Private GHCR");
  await chooseRegistry(user, dialog, registry);
  await fill(user, within(dialog).getByLabelText("Username"), "alice");
  await fill(
    user,
    within(dialog).getByLabelText(/^Personal Access Token/),
    "hunter2",
  );
}

describe("CreateRegistryCredentialDialog (w4/m670: Render's Add Credential)", () => {
  it("asks for Name, Registry, Username and Personal Access Token in Render's order", async () => {
    const { dialog } = await openDialog();
    expect(within(dialog).getByRole("heading")).toHaveTextContent(
      "Add Credential",
    );
    const labels = within(dialog)
      .getAllByText(/^(Name|Registry|Username|Personal Access Token)$/)
      .map((el) => el.textContent);
    expect(labels).toEqual([
      "Name",
      "Registry",
      "Username",
      "Personal Access Token",
    ]);
    expect(within(dialog).getByLabelText("Name")).toHaveAttribute(
      "placeholder",
      "My credential",
    );
    // The help link goes to bex's page on private registries.
    expect(
      within(dialog).getByRole("link", { name: "registry credentials" }),
    ).toHaveAttribute(
      "href",
      "https://bex.co/docs/platform/docker-deploys#use-a-private-registry",
    );
    // No registry host until a registry needs one.
    expect(within(dialog).queryByLabelText("Registry host")).toBeNull();
  });

  it("offers Render's five registries, in its order and with its labels", async () => {
    const { user, dialog } = await openDialog();
    await user.click(
      within(dialog).getByRole("combobox", { name: "Registry" }),
    );
    expect(
      (await screen.findAllByRole("option")).map((o) => o.textContent),
    ).toEqual(["GitHub", "GitLab", "Docker", "Google Artifact", "AWS ECR"]);
  });

  it.each([
    ["GitHub", "GITHUB", "ghcr.io"],
    ["GitLab", "GITLAB", "registry.gitlab.com"],
    ["Docker", "DOCKER", "docker.io"],
  ])(
    "sends %s as registry %s with no host, and hands the new id over",
    async (label, registry) => {
      create.mockResolvedValue("rgc-new");
      const { user, dialog, onCreated } = await openDialog();
      await fillRequired(user, dialog, label);
      expect(within(dialog).queryByLabelText("Registry host")).toBeNull();
      await user.click(
        within(dialog).getByRole("button", { name: "Add Credential" }),
      );

      expect(create).toHaveBeenCalledWith({
        registry,
        host: undefined,
        username: "alice",
        authToken: "hunter2",
        name: "Private GHCR",
      });
      expect(onCreated).toHaveBeenCalledWith("rgc-new");
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    },
  );

  it.each([
    ["Google Artifact", "GOOGLE_ARTIFACT", "us-docker.pkg.dev"],
    ["AWS ECR", "AWS_ECR", "123456789012.dkr.ecr.us-east-1.amazonaws.com"],
  ])(
    "asks %s for its registry host and sends both",
    async (label, registry, host) => {
      create.mockResolvedValue("rgc-new");
      const { user, dialog } = await openDialog();
      await fillRequired(user, dialog, label);
      const add = within(dialog).getByRole("button", {
        name: "Add Credential",
      });
      const hostField = within(dialog).getByLabelText("Registry host");
      expect(hostField).toHaveAttribute("placeholder", host);
      expect(add).toBeDisabled(); // the host is required here

      await fill(user, hostField, host);
      expect(add).toBeEnabled();
      await user.click(add);
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({ registry, host }),
      );
    },
  );

  it("marks a host of another registry, the pair bex-api refuses", async () => {
    const { user, dialog } = await openDialog();
    await fillRequired(user, dialog, "Google Artifact");
    const host = await fill(
      user,
      within(dialog).getByLabelText("Registry host"),
      "ghcr.io",
    );
    expect(host).toHaveAttribute("aria-invalid", "true");
    expect(host).toHaveAccessibleDescription(
      "Enter a Google Artifact host, such as us-docker.pkg.dev.",
    );
    expect(
      within(dialog).getByRole("button", { name: "Add Credential" }),
    ).toBeDisabled();

    await fill(user, host, "europe-west1-docker.pkg.dev");
    expect(host).not.toHaveAttribute("aria-invalid", "true");
  });

  it("shows Render's help for the chosen registry, its token page prefilled with the name", async () => {
    const { user, dialog } = await openDialog();
    await fill(user, within(dialog).getByLabelText("Name"), "ci pull");
    await chooseRegistry(user, dialog, "GitHub");
    expect(dialog).toHaveTextContent(
      "You’ll need to create a personal access token with the required read:packages permission in GitHub Settings > Developer Settings.",
    );
    expect(
      within(dialog).getByRole("link", { name: "GitHub" }),
    ).toHaveAttribute(
      "href",
      "https://github.com/settings/tokens/new?description=ci+pull&scopes=read%3Apackages",
    );

    await chooseRegistry(user, dialog, "AWS ECR");
    expect(dialog).toHaveTextContent(
      "The Username is the AWS Account ID that owns the ECR repository.",
    );
    expect(
      within(dialog).getByRole("link", { name: "the AWS documentation" }),
    ).toBeInTheDocument();
  });

  it("labels Docker's secret Personal Access Token or Password", async () => {
    const { user, dialog } = await openDialog();
    await chooseRegistry(user, dialog, "Docker");
    expect(
      within(dialog).getByLabelText("Personal Access Token or Password"),
    ).toBeInTheDocument();
  });

  it("holds Add Credential until Name, Registry, Username and token are filled", async () => {
    const { user, dialog } = await openDialog();
    const add = within(dialog).getByRole("button", { name: "Add Credential" });
    expect(add).toBeDisabled();

    await fill(user, within(dialog).getByLabelText("Username"), "alice");
    await fill(
      user,
      within(dialog).getByLabelText("Personal Access Token"),
      "hunter2",
    );
    expect(add).toBeDisabled(); // no name, no registry
    await chooseRegistry(user, dialog, "GitHub");
    expect(add).toBeDisabled(); // Render requires the name
    await fill(user, within(dialog).getByLabelText("Name"), "Private GHCR");
    expect(add).toBeEnabled();
    expect(create).not.toHaveBeenCalled();
  });

  // w21/035, w21/036: the name is a label held to 100 characters.
  it("stops the name at 100 characters and refuses a longer paste inline", async () => {
    const { user, dialog } = await openDialog();
    await fillRequired(user, dialog);
    const name = within(dialog).getByLabelText("Name");
    expect(name).toHaveAttribute("maxlength", "100");
    name.removeAttribute("maxlength"); // the field caps typing; exercise the check
    await fill(user, name, "a".repeat(101));
    expect(
      within(dialog).getByRole("button", { name: "Add Credential" }),
    ).toBeDisabled();
    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      "up to 100 characters",
    );
  });

  it("a create failure keeps the dialog open with the entered values intact", async () => {
    create.mockResolvedValue(null);
    const { user, dialog, onCreated } = await openDialog();
    await fillRequired(user, dialog);
    await user.click(
      within(dialog).getByRole("button", { name: "Add Credential" }),
    );

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(within(dialog).getByLabelText("Name")).toHaveValue("Private GHCR");
    expect(onCreated).not.toHaveBeenCalled();
  });

  it("resets the form when cancelled and reopened", async () => {
    const { user, dialog } = await openDialog();
    await fillRequired(user, dialog);
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));

    await user.click(screen.getByRole("button", { name: "Add credential" }));
    const reopened = await screen.findByRole("dialog");
    expect(within(reopened).getByLabelText("Name")).toHaveValue("");
    expect(
      within(reopened).getByRole("combobox", { name: "Registry" }),
    ).toHaveTextContent("Select a registry");
  });

  it("controlled, it renders no trigger of its own and reports closing", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(
      <CreateRegistryCredentialDialog
        open
        onOpenChange={onOpenChange}
        onCreated={vi.fn()}
      />,
    );
    const dialog = await screen.findByRole("dialog");
    expect(
      screen.queryByRole("button", { name: "Add credential" }),
    ).not.toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  // w21/053: what the server refuses is marked in the field and holds Add.
  describe("field rules (w21/053)", () => {
    it.each(["us-docker.pkg.dev/project/repo", "my registry.io"])(
      "marks the host %j and holds Add",
      async (value) => {
        const { user, dialog } = await openDialog();
        await fillRequired(user, dialog, "Google Artifact");
        const host = await fill(
          user,
          within(dialog).getByLabelText("Registry host"),
          value,
        );
        expect(host).toHaveAttribute("aria-invalid", "true");
        expect(host).toHaveAccessibleDescription(/without a path/);
        expect(
          within(dialog).getByRole("button", { name: "Add Credential" }),
        ).toBeDisabled();
      },
    );

    it("still accepts a pasted URL the server canonicalizes", async () => {
      create.mockResolvedValue("rgc-new");
      const { user, dialog } = await openDialog();
      await fillRequired(user, dialog, "Google Artifact");
      const host = await fill(
        user,
        within(dialog).getByLabelText("Registry host"),
        "https://US-DOCKER.PKG.DEV/",
      );
      expect(host).not.toHaveAttribute("aria-invalid", "true");
      await user.click(
        within(dialog).getByRole("button", { name: "Add Credential" }),
      );
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({ host: "https://US-DOCKER.PKG.DEV/" }),
      );
    });

    it("marks a username with a tab and a 300-byte username", async () => {
      const { user, dialog } = await openDialog();
      await fillRequired(user, dialog);
      const add = within(dialog).getByRole("button", {
        name: "Add Credential",
      });
      const username = await fill(
        user,
        within(dialog).getByLabelText("Username"),
        "qa\tuser",
      );
      expect(username).toHaveAttribute("aria-invalid", "true");
      expect(username).toHaveAccessibleDescription(
        "Must not contain control characters",
      );
      expect(add).toBeDisabled();

      await fill(user, username, "u".repeat(300));
      expect(username).toHaveAccessibleDescription("At most 253 bytes");
      expect(add).toBeDisabled();
      await fill(user, username, "alice");
      expect(add).toBeEnabled();
    });
  });
});
