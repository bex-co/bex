import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { InviteFallbackPage } from "@/features/invites/invite-fallback-page";
import { INVITE_TOKEN_STORAGE_KEY } from "@/common/lib/invite-token";

const TOKEN = "0123456789abcdef0123456789abcdef";

describe("InviteFallbackPage", () => {
  it.each([
    [false, "/auth/sign-up"],
    [true, "/"],
  ] as const)(
    "scrubs and routes authenticated=%s",
    async (authenticated, destination) => {
      window.sessionStorage.clear();
      window.history.replaceState(null, "", `/invite?invite=${TOKEN}`);
      const continueTo = vi.fn();

      render(
        <InviteFallbackPage
          authenticated={authenticated}
          continueTo={continueTo}
        />,
      );

      expect(screen.getByText("Opening your invitation")).toBeInTheDocument();
      await waitFor(() => expect(continueTo).toHaveBeenCalledWith(destination));
      expect(window.location.search).toBe("");
      expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBe(
        TOKEN,
      );
    },
  );
});
