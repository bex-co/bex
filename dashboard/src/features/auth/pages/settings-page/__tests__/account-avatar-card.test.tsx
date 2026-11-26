import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccountAvatarCard } from "@/features/auth/pages/settings-page/account-avatar-card";

// Account Settings' Avatar row (w1/m648): the account's own avatar with
// Render's Edit menu, its loading frame, and nothing without an account to
// show.

const avatar = vi.hoisted(() => ({
  value: {
    userId: null as string | null,
    imageURL: null as string | null,
    loading: true,
  },
}));
vi.mock("@/features/avatars/hooks/use-user-avatar", () => ({
  useUserAvatar: () => avatar.value,
}));
vi.mock("@/features/avatars/hooks/use-avatar-actions", async (original) => ({
  ...(await original<
    typeof import("@/features/avatars/hooks/use-avatar-actions")
  >()),
  useAvatarActions: () => ({ upload: vi.fn(), reset: vi.fn() }),
}));
vi.mock("@/common/hooks/use-root-context", () => ({
  useRootContext: () => ({
    session: { identity: { traits: { email: "ada@example.com" } } },
  }),
}));

beforeEach(() => {
  avatar.value = { userId: null, imageURL: null, loading: true };
});

describe("AccountAvatarCard", () => {
  it("draws its loading frame while the account loads", () => {
    render(<AccountAvatarCard />);
    expect(
      screen.getByTestId("account-avatar-card-skeleton"),
    ).toBeInTheDocument();
  });

  it("shows the monogram of the email and an enabled Edit", () => {
    avatar.value = { userId: "usr-ada", imageURL: null, loading: false };
    const { container } = render(<AccountAvatarCard />);
    expect(screen.getByText("Avatar")).toBeVisible();
    expect(container.querySelector('[data-slot="monogram"]')).toHaveTextContent(
      "A",
    );
    expect(screen.getByRole("button", { name: "Edit" })).toBeEnabled();
  });

  it("shows the account's upload", () => {
    avatar.value = {
      userId: "usr-ada",
      imageURL: "https://api.bex.test/v1/avatars/k1",
      loading: false,
    };
    render(<AccountAvatarCard />);
    expect(
      screen.getByRole("img", {
        name: "User-uploaded Avatar for ada@example.com",
      }),
    ).toHaveAttribute("src", "https://api.bex.test/v1/avatars/k1");
  });

  it("draws nothing when there is no account to show", () => {
    avatar.value = { userId: null, imageURL: null, loading: false };
    const { container } = render(<AccountAvatarCard />);
    expect(container).toBeEmptyDOMElement();
  });
});
