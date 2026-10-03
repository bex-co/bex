import { act, cleanup } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import { SessionRow } from "../session-row";

const authenticatedAt = "2026-10-02T12:00:00Z";

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-10-02T12:00:30Z"));
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("SessionRow clock", () => {
  it("refreshes the hydrated revoke label across a minute boundary", () => {
    const node = (
      <table>
        <tbody>
          <SessionRow
            session={{
              id: "other",
              current: false,
              userAgent: "Chrome",
              location: "US",
              authenticatedAt,
            }}
            onRevoke={vi.fn()}
            revoking={false}
          />
        </tbody>
      </table>
    );
    const container = document.createElement("div");
    container.innerHTML = renderToString(node);
    expect(container.querySelector("button")).toHaveAttribute(
      "aria-label",
      `Sign out Chrome, US, last active ${authenticatedAt}`,
    );
    document.body.appendChild(container);
    vi.setSystemTime(new Date("2026-10-02T12:01:30Z"));
    const recovered: unknown[] = [];
    const errors = vi.spyOn(console, "error").mockImplementation(() => {});
    let root: ReturnType<typeof hydrateRoot>;
    act(() => {
      root = hydrateRoot(container, node, {
        onRecoverableError: (error) => recovered.push(error),
      });
    });
    expect(container.querySelector("time")).toHaveTextContent("1m");
    expect(container.querySelector("button")).toHaveAttribute(
      "aria-label",
      "Sign out Chrome, US, last active 1m",
    );
    expect(errors).not.toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(60_000));
    expect(container.querySelector("time")).toHaveTextContent("2m");
    expect(container.querySelector("button")).toHaveAttribute(
      "aria-label",
      "Sign out Chrome, US, last active 2m",
    );
    expect(recovered).toEqual([]);
    act(() => root.unmount());
    container.remove();
    errors.mockRestore();
  });
});
