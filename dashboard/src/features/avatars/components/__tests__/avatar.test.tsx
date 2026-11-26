import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  UserAvatar,
  WorkspaceAvatar,
} from "@/features/avatars/components/avatar";
import { sha256Hex } from "@/common/lib/utils/sha256";
import { forgetGravatars } from "@/features/avatars/lib/avatar-utils";

// The three cases every avatar site draws (w1/m648): an upload, else (for a
// person) the Gravatar, else the monogram, and a Gravatar 404 is the monogram
// with no broken image.

vi.mock("@/config/config", () => ({
  config: { apiBaseUrl: "https://api.bex.test" },
}));

/** jsdom loads no images: this one answers each probe as the test says. */
const probes: string[] = [];
let gravatarAnswers = (_src: string) => false;
class ProbeImage {
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  set src(value: string) {
    probes.push(value);
    const ok = gravatarAnswers(value);
    queueMicrotask(() => (ok ? this.onload?.() : this.onerror?.()));
  }
}

beforeEach(() => {
  probes.length = 0;
  gravatarAnswers = () => false;
  forgetGravatars();
  vi.stubGlobal("Image", ProbeImage);
});
afterEach(() => vi.unstubAllGlobals());

const monogram = (container: HTMLElement) =>
  container.querySelector<HTMLElement>('[data-slot="monogram"]');

describe("UserAvatar", () => {
  it("shows an upload, on the API origin when bex-api answers a path", () => {
    render(<UserAvatar email="ada@example.com" imageURL="/v1/avatars/k1" />);
    const img = screen.getByRole("img", {
      name: "User-uploaded Avatar for ada@example.com",
    });
    expect(img).toHaveAttribute("src", "https://api.bex.test/v1/avatars/k1");
    expect(probes).toEqual([]);
  });

  it("falls back to the default when the upload cannot load", async () => {
    const { container } = render(
      <UserAvatar email="ada@example.com" imageURL="https://x.test/a.jpg" />,
    );
    fireEvent.error(screen.getByRole("img"));
    await waitFor(() => expect(monogram(container)).toHaveTextContent("A"));
  });

  it("shows the Gravatar when the email has one", async () => {
    gravatarAnswers = () => true;
    render(<UserAvatar email=" Ada@Example.com" size="large" />);
    const hash = await sha256Hex("ada@example.com");
    const img = await screen.findByRole("img", {
      name: "Gravatar for ada@example.com",
    });
    expect(img).toHaveAttribute(
      "src",
      `https://en.gravatar.com/avatar/${hash}?s=32&r=x&d=404`,
    );
    // One image, the size this screen shows: the probe already fetched it.
    expect(img).not.toHaveAttribute("srcset");
    expect(probes).toEqual([img.getAttribute("src")]);
  });

  it("draws the monogram on a Gravatar 404, never a broken image, and does not ask again", async () => {
    const { container, unmount } = render(
      <UserAvatar email="ada@example.com" />,
    );
    await waitFor(() => expect(probes).toHaveLength(1));
    await act(async () => {});
    expect(screen.queryByRole("img")).toBeNull();
    const letter = monogram(container);
    expect(letter).toHaveTextContent("A");
    // Render's palette for ada@example.com: orange.
    expect(letter).toHaveStyle({ backgroundColor: "#ffedd5" });
    unmount();

    // The miss is remembered: a second avatar for the same email draws the
    // monogram with no second request.
    render(<UserAvatar email="ada@example.com" />);
    await act(async () => {});
    expect(probes).toHaveLength(1);
  });
});

describe("WorkspaceAvatar", () => {
  it("shows the workspace's upload", () => {
    render(
      <WorkspaceAvatar
        id="tea-acme"
        name="acme"
        imageURL="https://api.bex.test/v1/avatars/k2"
      />,
    );
    expect(
      screen.getByRole("img", { name: "User-uploaded Avatar" }),
    ).toHaveAttribute("src", "https://api.bex.test/v1/avatars/k2");
  });

  it("is the monogram of its name colored by its id, with no Gravatar", async () => {
    const { container } = render(<WorkspaceAvatar id="tea-acme" name="acme" />);
    await act(async () => {});
    expect(monogram(container)).toHaveTextContent("A");
    expect(monogram(container)).toHaveStyle({ backgroundColor: "#dcfce7" });
    expect(probes).toEqual([]);
  });
});
