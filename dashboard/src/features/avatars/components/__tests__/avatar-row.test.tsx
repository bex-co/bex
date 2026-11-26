import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AvatarEditor } from "@/features/avatars/components/avatar-row";
import { AvatarType } from "@/features/avatars/hooks/use-avatar-actions";
import {
  AvatarPresignedUploadDocument,
  DeleteAvatarDocument,
} from "@/graphql/definitions";

// Render's Avatar row (w1/m648, docs/render-artifacts/avatars.md): Upload
// image, Change monogram and Reset to default, Render's toasts, and the row a
// member without permission sees.

const client = vi.hoisted(() => ({
  query: vi.fn(),
  mutate: vi.fn(),
  refetchQueries: vi.fn(() => Promise.resolve([])),
  cache: {
    identify: vi.fn(
      (o: { __typename: string; id: string }) => `${o.__typename}:${o.id}`,
    ),
    modify: vi.fn(),
  },
}));
vi.mock("@apollo/client/react", () => ({ useApolloClient: () => client }));
const toastError = vi.hoisted(() => vi.fn());
vi.mock("sonner", () => ({ toast: { error: toastError, success: vi.fn() } }));
vi.mock("@/config/config", () => ({
  config: { apiBaseUrl: "https://api.bex.test" },
}));
// jsdom has no canvas: the drawn JPEGs are stand-ins, and the 20 MB limit is
// scaled down so a test file can cross it.
const drawn = vi.hoisted(() => ({
  crop: new Blob(["crop-jpeg"], { type: "image/jpeg" }),
  monogram: new Blob(["monogram-jpeg"], { type: "image/jpeg" }),
}));
vi.mock("@/features/avatars/lib/avatar-image", async (original) => ({
  ...(await original<typeof import("@/features/avatars/lib/avatar-image")>()),
  MAX_AVATAR_DATA_URL_LENGTH: 200,
  cropToJpeg: vi.fn(() => Promise.resolve(drawn.crop)),
  monogramToJpeg: vi.fn(() => Promise.resolve(drawn.monogram)),
}));

const fetchMock = vi.fn();

beforeEach(() => {
  vi.clearAllMocks();
  client.query.mockResolvedValue({
    data: {
      avatarPresignedUpload: {
        __typename: "PresignedUpload",
        url: "/v1/avatar-uploads/token",
        verb: "PUT",
      },
    },
  });
  client.mutate.mockResolvedValue({ data: { deleteAvatar: true } });
  fetchMock.mockResolvedValue(new Response(null, { status: 200 }));
  vi.stubGlobal("fetch", fetchMock);
});

function editor(overrides: Partial<Parameters<typeof AvatarEditor>[0]> = {}) {
  return render(
    <AvatarEditor
      ownerId="usr-ada"
      avatarType={AvatarType.Account}
      avatar={<span>avatar</span>}
      isUpload={false}
      defaultMonogram={{ letter: "A", colorIndex: 3 }}
      canEdit
      {...overrides}
    />,
  );
}

const png = (bytes = 8) =>
  new File([new Uint8Array(bytes)], "me.png", { type: "image/png" });

async function openMenu(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Edit" }));
}

/** The cache write that makes the change show without a reload. */
function cachedAvatar(): unknown {
  const call = client.cache.modify.mock.calls.at(-1)?.[0] as {
    fields: { avatarDownload: () => unknown };
  };
  return call.fields.avatarDownload();
}

describe("AvatarEditor", () => {
  it("offers Reset to default only while the avatar is an upload", async () => {
    const user = userEvent.setup();
    const { unmount } = editor();
    await openMenu(user);
    expect(
      screen.getByRole("menuitem", { name: "Upload image" }),
    ).toBeVisible();
    expect(
      screen.getByRole("menuitem", { name: "Change monogram" }),
    ).toBeVisible();
    expect(
      screen.queryByRole("menuitem", { name: "Reset to default" }),
    ).toBeNull();
    unmount();

    editor({ isUpload: true });
    await openMenu(user);
    expect(
      screen.getByRole("menuitem", { name: "Reset to default" }),
    ).toBeVisible();
  });

  it("uploads a cropped image through Render's presigned PUT and shows it at once", async () => {
    const user = userEvent.setup();
    editor();
    await user.upload(screen.getByTestId("avatar-file-input"), png());
    expect(
      await screen.findByRole("dialog", { name: "Crop Avatar" }),
    ).toBeVisible();
    // jsdom loads no image: tell the cropper its size.
    const img = screen.getByTestId("avatar-cropper").querySelector("img")!;
    Object.defineProperty(img, "naturalWidth", { value: 200 });
    Object.defineProperty(img, "naturalHeight", { value: 100 });
    img.dispatchEvent(new Event("load"));
    await user.click(await screen.findByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(screen.queryByRole("dialog", { name: "Crop Avatar" })).toBeNull(),
    );
    expect(client.query).toHaveBeenCalledWith(
      expect.objectContaining({
        query: AvatarPresignedUploadDocument,
        variables: {
          avatarOwnerId: "usr-ada",
          avatarType: "ACCOUNT",
          avatarSizeBytes: drawn.crop.size,
        },
      }),
    );
    expect(fetchMock).toHaveBeenCalledWith(
      "https://api.bex.test/v1/avatar-uploads/token",
      {
        method: "PUT",
        body: drawn.crop,
        headers: { "Content-Type": "image/jpeg" },
      },
    );
    expect(client.cache.modify).toHaveBeenCalledWith(
      expect.objectContaining({ id: "User:usr-ada" }),
    );
    expect(cachedAvatar()).toEqual({
      __typename: "PresignedDownload",
      url: expect.stringMatching(/^data:image\/jpeg;base64,/),
    });
    expect(toastError).not.toHaveBeenCalled();
  });

  it("refuses a file that is not a PNG or JPEG", async () => {
    const user = userEvent.setup({ applyAccept: false });
    editor();
    await user.upload(
      screen.getByTestId("avatar-file-input"),
      new File(["<svg/>"], "me.svg", { type: "image/svg+xml" }),
    );
    expect(toastError).toHaveBeenCalledWith("Invalid file type", {
      description: "Please select a JPG, JPEG, or PNG file.",
    });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("refuses a file over the size limit", async () => {
    const user = userEvent.setup();
    editor();
    await user.upload(screen.getByTestId("avatar-file-input"), png(400));
    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith("File too large", {
        description: "Please select a file under 20 MB.",
      }),
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("says so when the upload fails, and keeps the avatar", async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 400 }));
    const user = userEvent.setup();
    editor();
    await openMenu(user);
    await user.click(screen.getByRole("menuitem", { name: "Change monogram" }));
    await user.clear(screen.getByLabelText("Text"));
    await user.type(screen.getByLabelText("Text"), "z");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith("Unable to upload file", {
        description:
          "There was a problem connecting to the server. Please try again.",
      }),
    );
    expect(client.cache.modify).not.toHaveBeenCalled();
    // The dialog stays open for another try.
    expect(screen.getByRole("dialog", { name: "Edit Monogram" })).toBeVisible();
  });

  it("uploads a changed monogram as an image, and resets for the default", async () => {
    const user = userEvent.setup();
    editor({ isUpload: true });
    await openMenu(user);
    await user.click(screen.getByRole("menuitem", { name: "Change monogram" }));
    const dialog = screen.getByRole("dialog", { name: "Edit Monogram" });
    // It opens on the default: A, orange.
    expect(screen.getByLabelText("Text")).toHaveValue("A");
    expect(screen.getByRole("radio", { name: "Orange" })).toBeChecked();
    // One character, upper-cased.
    await user.clear(screen.getByLabelText("Text"));
    await user.type(screen.getByLabelText("Text"), "bq");
    expect(screen.getByLabelText("Text")).toHaveValue("B");
    await user.click(screen.getByRole("radio", { name: "Purple" }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(fetchMock).toHaveBeenCalledWith(
      "https://api.bex.test/v1/avatar-uploads/token",
      expect.objectContaining({ body: drawn.monogram }),
    );

    // The default, saved over an upload, is Reset to default.
    fetchMock.mockClear();
    await openMenu(user);
    await user.click(screen.getByRole("menuitem", { name: "Change monogram" }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(client.mutate).toHaveBeenCalledWith({
        mutation: DeleteAvatarDocument,
        variables: { avatarOwnerId: "usr-ada", avatarType: "ACCOUNT" },
      }),
    );
    expect(fetchMock).not.toHaveBeenCalled();
    expect(cachedAvatar()).toBeNull();
  });

  it("does not disable Save for the default when nothing is uploaded, and sends nothing", async () => {
    const user = userEvent.setup();
    editor();
    await openMenu(user);
    await user.click(screen.getByRole("menuitem", { name: "Change monogram" }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(client.mutate).not.toHaveBeenCalled();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("disables Save until the text is one character", async () => {
    const user = userEvent.setup();
    editor();
    await openMenu(user);
    await user.click(screen.getByRole("menuitem", { name: "Change monogram" }));
    await user.clear(screen.getByLabelText("Text"));
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("resets an upload with deleteAvatar, and says so when it fails", async () => {
    const user = userEvent.setup();
    editor({
      isUpload: true,
      avatarType: AvatarType.Workspace,
      ownerId: "tea-acme",
    });
    await openMenu(user);
    await user.click(
      screen.getByRole("menuitem", { name: "Reset to default" }),
    );
    await waitFor(() =>
      expect(client.mutate).toHaveBeenCalledWith({
        mutation: DeleteAvatarDocument,
        variables: { avatarOwnerId: "tea-acme", avatarType: "WORKSPACE" },
      }),
    );
    expect(client.cache.modify).toHaveBeenCalledWith(
      expect.objectContaining({ id: "Workspace:tea-acme" }),
    );

    client.mutate.mockRejectedValueOnce(new Error("boom"));
    await openMenu(user);
    await user.click(
      screen.getByRole("menuitem", { name: "Reset to default" }),
    );
    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith("Unable to delete file", {
        description:
          "There was a problem connecting to the server. Please try again.",
      }),
    );
  });

  it("shows a member without permission the avatar and a disabled Edit with the reason", async () => {
    const user = userEvent.setup();
    editor({
      canEdit: false,
      noPermissionMessage: "Only workspace admins can set the workspace image.",
    });
    expect(screen.getByText("avatar")).toBeVisible();
    const edit = screen.getByRole("button", { name: "Edit" });
    expect(edit).toBeDisabled();
    await user.hover(edit.parentElement!);
    expect(
      await screen.findAllByText(
        "Only workspace admins can set the workspace image.",
      ),
    ).not.toHaveLength(0);
    expect(screen.queryByRole("menuitem")).toBeNull();
  });
});
