import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { UndeployableImageFailureCode } from "@/graphql/schema-types";
import { ServiceSourcePicker } from "@/features/services/components/service-source-picker";
import type { ImageCheckMessages } from "@/features/services/lib/image-check-messages";
import type { ImageCheck } from "@/features/services/hooks/use-image-check";

// w1/m673: the image tab shows Render's isDeployableImage states — a spinner
// while it checks, a check mark or a caution after — and Render's sentence for
// each answer, in the create form and in Settings' Update source. The check's
// state is handed in; useImageCheck has its own tests.

vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => ({
    repos: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/git/hooks/use-git-connection", () => ({
  useGitConnection: () => ({
    connection: { connected: true },
    loading: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/git/hooks/use-on-git-connections-change", () => ({
  useOnGitConnectionsChange: () => {},
}));
vi.mock("@/features/git/hooks/use-connect-git", () => ({
  useConnectGitAccount: () => ({ connect: vi.fn(), busy: false }),
}));
vi.mock("@/features/git/components/git-credentials-menu", () => ({
  GitCredentialsMenu: () => null,
}));
vi.mock(
  "@/features/registry-credentials/hooks/use-registry-credentials",
  () => ({
    useRegistryCredentials: () => ({
      credentials: [
        {
          id: "rgc-ghcr",
          name: "GHCR",
          registry: "GITHUB",
          host: "ghcr.io",
          username: "robot",
          expiresAt: null,
          status: "active",
          createdAt: null,
        },
      ],
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    }),
  }),
);
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => ({ canManage: true }),
}));
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: ({ children, ...props }: { children: React.ReactNode }) => (
    <a {...props}>{children}</a>
  ),
}));

function check(
  status: ImageCheck["status"],
  failureCode: UndeployableImageFailureCode | null = null,
  withCredential = false,
): ImageCheck {
  return { status, failureCode, withCredential, notePaste: vi.fn() };
}

function renderField({
  state,
  messages,
  value = "ghcr.io/acme/app:1",
  credentialId = "",
  serverError = null,
}: {
  state: ImageCheck;
  messages: ImageCheckMessages;
  value?: string;
  credentialId?: string;
  serverError?: string | null;
}) {
  const onChange = vi.fn();
  render(
    <ServiceSourcePicker
      tab="image"
      onTabChange={vi.fn()}
      selectedRepo={null}
      onSelectRepo={vi.fn()}
      gitUrl=""
      onGitUrlChange={vi.fn()}
      image={{
        value,
        onChange,
        registryCredentialId: credentialId,
        onRegistryCredentialChange: vi.fn(),
        showPortHint: false,
        serverError,
        check: { state, messages },
      }}
    />,
  );
  return { onChange, input: screen.getByLabelText("Docker Image") };
}

const line = () => screen.getByTestId("image-message-line");
// The sentence the line shows, without the invisible sizers.
const shown = () =>
  within(line())
    .queryAllByText(/./)
    .filter((p) => p.getAttribute("aria-hidden") !== "true")
    .map((p) => p.textContent);

const NO_PUBLIC =
  "No public image found. It looks like you may be using a private image - please add a credential or check the image URL for mistakes.";

describe("the image field's check", () => {
  it.each([
    [UndeployableImageFailureCode.DoesNotExist, false, NO_PUBLIC],
    [
      UndeployableImageFailureCode.DoesNotExist,
      true,
      "The provided image URL and registry credential are invalid.",
    ],
    [
      UndeployableImageFailureCode.InvalidInputUrl,
      false,
      "The provided image URL is invalid.",
    ],
    [
      UndeployableImageFailureCode.InvalidRegistryCredential,
      true,
      "The provided registry credential is invalid.",
    ],
    [
      UndeployableImageFailureCode.UnsupportedPlatform,
      false,
      "The provided image URL points to an image with an invalid platform. Images must be built with the platform linux/amd64.",
    ],
    [
      UndeployableImageFailureCode.ImageTooLarge,
      false,
      "This Docker image's compressed size is larger than the allowed limit.",
    ],
    [null, false, "An error occurred."],
  ])(
    "says Render's sentence for %s (credential: %s), with the caution mark",
    (code, withCredential, sentence) => {
      for (const messages of ["create", "update"] as const) {
        const { input } = renderField({
          state: check("failed", code, withCredential),
          messages,
          credentialId: withCredential ? "rgc-ghcr" : "",
        });
        expect(screen.getByRole("alert")).toHaveTextContent(sentence);
        expect(input).toHaveAttribute("aria-invalid", "true");
        expect(input).toHaveAccessibleDescription(sentence);
        expect(screen.getByTestId("image-check-caution")).toBeInTheDocument();
        document.body.innerHTML = "";
      }
    },
  );

  it("shows a spinner while checking, and the create form keeps its helper line", () => {
    renderField({ state: check("checking"), messages: "create" });
    expect(
      screen.getByRole("status", { name: "Checking the image" }),
    ).toBeInTheDocument();
    expect(shown()).toEqual(["Deploy an image from a Docker registry"]);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("marks a deployable image and keeps the create form's helper line, as Render's create flow does", () => {
    renderField({ state: check("deployable"), messages: "create" });
    expect(screen.getByTestId("image-check-success")).toBeInTheDocument();
    expect(shown()).toEqual(["Deploy an image from a Docker registry"]);
  });

  it.each([
    [false, "This is a public image that can be accessed without credentials."],
    [
      true,
      "We were able to authorize access to this image with the selected credential.",
    ],
  ])(
    "says why a deployable image is in Settings (credential: %s)",
    (withCredential, sentence) => {
      renderField({
        state: check("deployable", null, withCredential),
        messages: "update",
        credentialId: withCredential ? "rgc-ghcr" : "",
      });
      expect(shown()).toEqual([sentence]);
      expect(screen.getByTestId("image-check-success")).toBeInTheDocument();
    },
  );

  it("says nothing in Settings while it checks, holding the line's height", () => {
    renderField({ state: check("checking"), messages: "update" });
    expect(shown()).toEqual([]);
    // Every sentence the line can say is there, invisible, so the line is as
    // tall as the longest one whatever it shows.
    const sizers = within(line())
      .getAllByText(/./)
      .filter((p) => p.getAttribute("aria-hidden") === "true")
      .map((p) => p.textContent);
    expect(sizers).toContain(NO_PUBLIC);
    expect(sizers).toContain(
      "We were able to authorize access to this image with the selected credential.",
    );
  });

  it("puts the field's own syntax message and the server's refusal before the check", () => {
    renderField({
      state: check("idle"),
      messages: "create",
      value: "Not An Image",
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      /Use a Docker image reference/,
    );
    document.body.innerHTML = "";
    renderField({
      state: check("deployable"),
      messages: "create",
      serverError: "image registry host is not trusted",
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "image registry host is not trusted",
    );
  });

  it("disables the credential select for a deployable public image, as Render does", () => {
    renderField({ state: check("deployable"), messages: "create" });
    expect(
      screen.getByRole("combobox", { name: /Registry credential/i }),
    ).toBeDisabled();
  });

  it("keeps the credential select open after a failure, and marks a chosen credential that failed", () => {
    renderField({
      state: check("failed", UndeployableImageFailureCode.DoesNotExist),
      messages: "create",
    });
    const select = screen.getByRole("combobox", {
      name: /Registry credential/i,
    });
    expect(select).toBeEnabled();
    expect(select).not.toHaveAttribute("aria-invalid");
    document.body.innerHTML = "";
    renderField({
      state: check(
        "failed",
        UndeployableImageFailureCode.InvalidRegistryCredential,
        true,
      ),
      messages: "create",
      credentialId: "rgc-ghcr",
    });
    expect(
      screen.getByRole("combobox", { name: /Registry credential/i }),
    ).toHaveAttribute("aria-invalid", "true");
  });

  it("tells the check about a paste", async () => {
    const user = userEvent.setup();
    const state = check("idle");
    const { input } = renderField({ state, messages: "create", value: "" });
    await user.click(input);
    await user.paste("nginx");
    expect(state.notePaste).toHaveBeenCalled();
  });
});
