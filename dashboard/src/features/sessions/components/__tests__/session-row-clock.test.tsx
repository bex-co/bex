import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { SessionRow } from "../session-row";
import i18n from "@/i18n/init";
import type { SessionView } from "../../types";

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-10-02T12:00:00Z"));
});
afterEach(async () => {
  cleanup();
  await i18n.changeLanguage("en");
  vi.useRealTimers();
});

it("advances the unchanged session's visible age and accessible revoke label together", () => {
  const session: SessionView = {
    id: "other-session",
    current: false,
    userAgent: "Safari on iOS",
    location: "Paris",
    authenticatedAt: "2026-10-02T11:55:00Z",
  };
  const revoke = vi.fn().mockResolvedValue(true);
  const { container } = render(
    <table>
      <tbody>
        <SessionRow session={session} onRevoke={revoke} revoking={false} />
      </tbody>
    </table>,
  );
  const time = container.querySelector("time")!;
  const button = screen.getByRole("button", {
    name: "Sign out Safari on iOS, Paris, last active 5m",
  });
  expect(time).toHaveTextContent("5m");
  expect(time).toHaveAttribute("datetime", session.authenticatedAt);
  expect(button).toBeEnabled();
  button.focus();

  act(() => vi.advanceTimersByTime(120_000));

  expect(time).toHaveTextContent("7m");
  expect(time).toHaveAttribute("datetime", session.authenticatedAt);
  expect(
    screen.getByRole("button", {
      name: "Sign out Safari on iOS, Paris, last active 7m",
    }),
  ).toBe(button);
  expect(button).toBeEnabled();
  expect(button).toHaveFocus();
  expect(revoke).not.toHaveBeenCalled();
});

// w4/m188: the revoke name came from a direct formatRelativeAge call with no
// language, so a Chinese row's visible age and accessible name disagreed.
it("localizes the visible age and the revoke name together", async () => {
  const session: SessionView = {
    id: "other-session",
    current: false,
    userAgent: "Safari on iOS",
    location: "Paris",
    authenticatedAt: "2026-10-02T11:55:00Z",
  };
  const { container } = render(
    <table>
      <tbody>
        <SessionRow session={session} onRevoke={vi.fn()} revoking={false} />
      </tbody>
    </table>,
  );
  await act(() => i18n.changeLanguage("zh"));
  const time = container.querySelector("time")!;
  expect(time).toHaveTextContent("5分钟");
  expect(time).toHaveAttribute("datetime", session.authenticatedAt);
  expect(
    screen.getByRole("button", {
      name: "退出登录 Safari on iOS，Paris，最近活动 5分钟",
    }),
  ).toBeInTheDocument();
});
